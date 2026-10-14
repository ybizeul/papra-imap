package watcher

import "testing"

func boolPtr(value bool) *bool {
	return &value
}

func TestShouldMarkAsRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		markAsRead          *bool
		eligibleAttachments int
		allUploaded         bool
		want                bool
	}{
		{
			name:                "marks a message when eligible attachments uploaded",
			markAsRead:          boolPtr(true),
			eligibleAttachments: 1,
			allUploaded:         true,
			want:                true,
		},
		{
			name:                "does not mark a message without eligible attachments",
			markAsRead:          boolPtr(true),
			eligibleAttachments: 0,
			allUploaded:         true,
			want:                false,
		},
		{
			name:                "does not mark a message after a failed upload",
			markAsRead:          boolPtr(true),
			eligibleAttachments: 1,
			allUploaded:         false,
			want:                false,
		},
		{
			name:                "does not mark a message when marking is disabled",
			markAsRead:          boolPtr(false),
			eligibleAttachments: 1,
			allUploaded:         true,
			want:                false,
		},
		{
			name:                "does not mark a message when setting is unset",
			markAsRead:          nil,
			eligibleAttachments: 1,
			allUploaded:         true,
			want:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldMarkAsRead(tt.markAsRead, tt.eligibleAttachments, tt.allUploaded); got != tt.want {
				t.Fatalf("shouldMarkAsRead() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestFilenameFromSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		subject  string
		fallback string
		want     string
	}{
		{
			name:     "uses subject with original extension",
			subject:  "Monthly report",
			fallback: "invoice.pdf",
			want:     "Monthly report.pdf",
		},
		{
			name:     "falls back when subject is empty",
			subject:  "   ",
			fallback: "invoice.pdf",
			want:     "invoice.pdf",
		},
		{
			name:     "sanitizes path separators",
			subject:  "Q4/2026: summary",
			fallback: "report.docx",
			want:     "Q4-2026- summary.docx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := filenameFromSubject(tt.subject, tt.fallback); got != tt.want {
				t.Fatalf("filenameFromSubject() = %q, want %q", got, tt.want)
			}
		})
	}
}
