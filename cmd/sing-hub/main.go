package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"sing-hub/internal/assembler"
	"sing-hub/internal/config"
	"sing-hub/internal/server"
	"sing-hub/internal/store"
	"sing-hub/internal/updater"
	"sing-hub/internal/versioning"
)

func main() {
	// 纯内存加载环境变量与 Flag，彻底告别文件 I/O 报错
	cfg := config.Load()

	logLevel := new(slog.LevelVar)
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "warn":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))

	slog.Info("starting sing-hub engine",
		"listen", cfg.Listen,
		"data_dir", cfg.DataDir,
		"log_level", cfg.LogLevel,
	)

	_ = os.MkdirAll(cfg.DataDir, 0755)
	dbPath := filepath.Join(cfg.DataDir, "sing-hub.db")

	s, err := store.New(dbPath)
	if err != nil {
		slog.Error("fatal: open sqlite failed", "path", dbPath, "err", err)
		os.Exit(1)
	}
	defer s.Close()

	gitRepo, err := versioning.NewGitRepo(cfg.DataDir)
	if err != nil {
		slog.Warn("gitops engine init warning (version audit disabled)", "err", err)
	}

	binDir := filepath.Join(cfg.DataDir, "bin")
	eng := assembler.New(s, gitRepo, binDir)
	upd := updater.New(s, binDir)

	srv := server.New(s, eng, gitRepo, upd, dbPath, logLevel)

	httpServer := &http.Server{
		Addr:    cfg.Listen,
		Handler: srv,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("sing-hub dual-stack service is running", "endpoint", cfg.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("shutting down sing-hub gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server forced to shutdown", "err", err)
	}
	slog.Info("sing-hub shutdown gracefully completed")
}
