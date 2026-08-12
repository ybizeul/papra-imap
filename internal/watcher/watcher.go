package watcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"

	"github.com/ybizeul/papra-imap/internal/config"
	"github.com/ybizeul/papra-imap/internal/papra"
)

const maxBackoff = 5 * time.Minute

type attachment struct {
	filename string
	data     []byte
}

// Run monitors an IMAP account and imports attachments to papra.
// It reconnects automatically with exponential backoff on failure.
func Run(ctx context.Context, acc config.AccountConfig, papraClient *papra.Client) {
	log := slog.With("account", acc.Name)
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		start := time.Now()
		err := watch(ctx, acc, papraClient, log)

		// Reset backoff if the session ran successfully for at least a minute.
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}

		if err != nil {
			log.Error("watcher error, reconnecting", "error", err, "backoff", backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func watch(ctx context.Context, acc config.AccountConfig, papraClient *papra.Client, log *slog.Logger) error {
	newMailCh := make(chan struct{}, 1)

	opts := &imapclient.Options{
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					select {
					case newMailCh <- struct{}{}:
					default:
					}
				}
			},
		},
	}

	c, err := dial(acc, opts)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer c.Close()

	if err := c.Login(acc.Username, acc.Password).Wait(); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	defer func() {
		// Only logout if the connection is still in an authenticated state.
		if s := c.State(); s == imap.ConnStateAuthenticated || s == imap.ConnStateSelected {
			c.Logout().Wait()
		}
	}()

	// Verify the connection is alive after login before proceeding.
	if err := c.Noop().Wait(); err != nil {
		return fmt.Errorf("noop after login: %w", err)
	}

	selData, err := c.Select(acc.Folder, nil).Wait()
	if err != nil {
		return fmt.Errorf("select %q: %w", acc.Folder, err)
	}
	log.Debug("folder selected", "messages", selData.NumMessages)

	caps := c.Caps()
	hasIdle := caps.Has(imap.CapIdle) || caps.Has(imap.CapIMAP4rev2)
	if hasIdle {
		log.Info("connected", "folder", acc.Folder, "mode", "idle")
	} else {
		log.Info("connected", "folder", acc.Folder, "mode", "poll", "interval", acc.PollInterval.Duration)
	}

	// Process any existing unseen messages on startup.
	if err := processUnseen(ctx, c, acc, papraClient, log); err != nil {
		log.Error("initial processing failed", "error", err)
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		if hasIdle {
			idleCmd, err := c.Idle()
			if err != nil {
				return fmt.Errorf("IDLE: %w", err)
			}

			select {
			case <-ctx.Done():
				idleCmd.Close()
				idleCmd.Wait()
				return nil
			case <-newMailCh:
				idleCmd.Close()
			case <-time.After(acc.PollInterval.Duration):
				idleCmd.Close()
			}

			if err := idleCmd.Wait(); err != nil {
				log.Debug("IDLE ended", "error", err)
			}
		} else {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(acc.PollInterval.Duration):
			}
		}

		if err := processUnseen(ctx, c, acc, papraClient, log); err != nil {
			return fmt.Errorf("processUnseen: %w", err)
		}
	}
}

func dial(acc config.AccountConfig, opts *imapclient.Options) (*imapclient.Client, error) {
	addr := fmt.Sprintf("%s:%d", acc.Host, acc.Port)
	if acc.SSL {
		return imapclient.DialTLS(addr, opts)
	}
	return imapclient.DialInsecure(addr, opts)
}

func processUnseen(ctx context.Context, c *imapclient.Client, acc config.AccountConfig, papraClient *papra.Client, log *slog.Logger) error {
	criteria := &imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}
	if acc.Email != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key:   "To",
			Value: acc.Email,
		})
	}

	searchData, err := c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return fmt.Errorf("UID SEARCH: %w", err)
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		log.Debug("no unseen messages")
		return nil
	}

	log.Debug("found unseen messages", "count", len(uids))

	bodySectionItem := &imap.FetchItemBodySection{Peek: true}
	fetchOpts := &imap.FetchOptions{
		Envelope:    true,
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{bodySectionItem},
	}

	msgs, err := c.Fetch(imap.UIDSetNum(uids...), fetchOpts).Collect()
	if err != nil {
		return fmt.Errorf("FETCH: %w", err)
	}

	var toMarkRead []imap.UID

	for _, msg := range msgs {
		msgLog := log

		if msg.Envelope != nil && msg.Envelope.MessageID != "" {
			msgLog = log.With("messageID", msg.Envelope.MessageID)
		}

		bodyBytes := msg.FindBodySection(bodySectionItem)
		if bodyBytes == nil {
			msgLog.Warn("message has no body")
			continue
		}

		attachments, err := extractAttachments(bodyBytes)
		if err != nil {
			msgLog.Error("failed to extract attachments", "error", err)
			continue
		}

		if len(acc.Extensions) > 0 {
			attachments = filterByExtension(attachments, acc.Extensions)
		}

		if len(attachments) == 0 {
			msgLog.Debug("no attachments found")
		}

		allUploaded := true
		messageSubject := ""
		if msg.Envelope != nil {
			messageSubject = msg.Envelope.Subject
		}
		for _, att := range attachments {
			filename := att.filename
			if acc.SubjectAsTitle && len(attachments) == 1 {
				filename = filenameFromSubject(messageSubject, att.filename)
			}

			if err := papraClient.UploadDocument(ctx, acc.OrganizationID, filename, att.data, acc.Tags); err != nil {
				msgLog.Error("upload failed", "filename", filename, "error", err)
				allUploaded = false
			} else {
				msgLog.Info("uploaded document", "filename", filename)
			}
		}

		if acc.MarkAsRead != nil && *acc.MarkAsRead && allUploaded {
			toMarkRead = append(toMarkRead, msg.UID)
		}
	}

	if len(toMarkRead) > 0 {
		storeFlags := &imap.StoreFlags{
			Op:     imap.StoreFlagsAdd,
			Silent: true,
			Flags:  []imap.Flag{imap.FlagSeen},
		}
		if err := c.Store(imap.UIDSetNum(toMarkRead...), storeFlags, nil).Close(); err != nil {
			log.Error("failed to mark messages as read", "error", err)
		}
	}

	return nil
}

func extractAttachments(msgBytes []byte) ([]attachment, error) {
	mr, err := mail.CreateReader(bytes.NewReader(msgBytes))
	if err != nil {
		return nil, fmt.Errorf("parse email: %w", err)
	}

	var attachments []attachment
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		var filename string
		switch h := part.Header.(type) {
		case *mail.AttachmentHeader:
			filename, _ = h.Filename()
		case *mail.InlineHeader:
			filename = filenameFromRawHeader(h.Get("Content-Disposition"), h.Get("Content-Type"))
		default:
			continue
		}
		if filename == "" {
			continue
		}

		data, err := io.ReadAll(part.Body)
		if err != nil {
			continue
		}

		attachments = append(attachments, attachment{filename: filename, data: data})
	}

	return attachments, nil
}

func filterByExtension(attachments []attachment, extensions []string) []attachment {
	allowed := make(map[string]struct{}, len(extensions))
	for _, ext := range extensions {
		allowed["."+strings.ToLower(strings.TrimPrefix(ext, "."))] = struct{}{}
	}

	filtered := attachments[:0]
	for _, att := range attachments {
		ext := strings.ToLower(filepath.Ext(att.filename))
		if _, ok := allowed[ext]; ok {
			filtered = append(filtered, att)
		}
	}
	return filtered
}

// filenameFromRawHeader extracts a filename from Content-Disposition then Content-Type params.
func filenameFromRawHeader(disposition, contentType string) string {
	for _, raw := range []string{disposition, contentType} {
		if raw == "" {
			continue
		}
		_, params, err := mime.ParseMediaType(raw)
		if err != nil {
			continue
		}
		if name := params["filename"]; name != "" {
			return name
		}
		if name := params["name"]; name != "" {
			return name
		}
	}
	return ""
}

func filenameFromSubject(subject, fallbackFilename string) string {
	trimmed := strings.TrimSpace(subject)
	if trimmed == "" {
		return fallbackFilename
	}

	base := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	).Replace(trimmed)
	base = strings.Join(strings.Fields(base), " ")
	if base == "" {
		return fallbackFilename
	}

	return base + filepath.Ext(fallbackFilename)
}
