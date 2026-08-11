package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/papra-hq/papra-imap/internal/config"
	"github.com/papra-hq/papra-imap/internal/papra"
	"github.com/papra-hq/papra-imap/internal/watcher"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	if *debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	papraClient := papra.NewClient(cfg.Papra.Host, cfg.Papra.APIKey)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	for _, acc := range cfg.Accounts {
		wg.Add(1)
		go func(a config.AccountConfig) {
			defer wg.Done()
			watcher.Run(ctx, a, papraClient)
		}(acc)
	}

	wg.Wait()
	slog.Info("shutdown complete")
}
