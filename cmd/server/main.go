package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"clip-share/internal/config"
	"clip-share/internal/httpapi"
	"clip-share/internal/media"
	"clip-share/internal/retention"
	"clip-share/internal/store"
	"clip-share/internal/webui"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration_invalid", "error", err)
		os.Exit(1)
	}
	if err := config.PrepareDataDirectories(cfg); err != nil {
		logger.Error("data_directory_failed", "error", err)
		os.Exit(1)
	}
	for _, command := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(command); err != nil {
			logger.Error("media_tool_missing", "tool", command)
			os.Exit(1)
		}
	}

	db, err := store.Open(cfg.DatabasePath)
	if err != nil {
		logger.Error("database_open_failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.RecoverInterruptedJobs(context.Background()); err != nil {
		logger.Error("job_recovery_failed", "error", err)
		os.Exit(1)
	}
	if storageIDs, err := db.RecoverInterruptedEditorPreviews(context.Background()); err != nil {
		logger.Error("editor_preview_recovery_failed", "error", err)
		os.Exit(1)
	} else {
		for _, storageID := range storageIDs {
			if err := os.Remove(media.PendingPreviewPath(cfg.DataDir, storageID)); err != nil && !os.IsNotExist(err) {
				logger.Error("editor_preview_recovery_asset_failed", "error", err)
				os.Exit(1)
			}
		}
	}
	// Preserve recent sessions across a deployment; only sessions already past
	// the ordinary five-minute inactivity lease are abandoned at startup.
	if storageIDs, err := db.ExpireEditorSessions(context.Background(), time.Now()); err != nil {
		logger.Error("editor_recovery_failed", "error", err)
		os.Exit(1)
	} else {
		for _, storageID := range storageIDs {
			if err := media.RemoveStoredAssets(cfg.DataDir, storageID); err != nil {
				logger.Error("editor_recovery_asset_cleanup_failed", "error", err)
				os.Exit(1)
			}
		}
	}

	handler := httpapi.New(cfg, db, logger, webui.Handler())
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	processor := media.NewProcessor(db, cfg.DataDir, logger)
	processorDone := make(chan struct{})
	retentionDone := make(chan struct{})
	go func() {
		defer close(processorDone)
		processor.Run(ctx)
	}()
	go func() {
		defer close(retentionDone)
		retention.Run(ctx, db, cfg.DataDir, logger)
	}()

	go func() {
		logger.Info("server_started", "address", cfg.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server_failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server_shutdown_failed", "error", err)
		return
	}
	select {
	case <-processorDone:
	case <-shutdownCtx.Done():
		logger.Error("processor_shutdown_timed_out")
	}
	select {
	case <-retentionDone:
	case <-shutdownCtx.Done():
		logger.Error("retention_shutdown_timed_out")
	}
	logger.Info("server_stopped")
}
