// cmd/sing-hub/main.go
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"sing-hub/internal/assembler"
	"sing-hub/internal/config"
	"sing-hub/internal/server"
	"sing-hub/internal/store"
	"sing-hub/internal/versioning"
)

func main() {
	configPath := flag.String("config", "./config.toml", "Path to config.toml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("fatal: failed to load config.toml", "path", *configPath, "err", err)
		os.Exit(1)
	}

	logLevel := new(slog.LevelVar)
	if strings.ToLower(cfg.Server.LogLevel) == "debug" {
		logLevel.Set(slog.LevelDebug)
	} else {
		logLevel.Set(slog.LevelInfo)
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))

	slog.Info("starting sing-hub engine",
		"listen", cfg.Server.Listen,
		"data_dir", cfg.Server.DataDir,
		"initial_log_level", cfg.Server.LogLevel,
	)

	_ = os.MkdirAll(cfg.Server.DataDir, 0755)
	dbPath := filepath.Join(cfg.Server.DataDir, "sing-hub.db")

	s, err := store.New(dbPath)
	if err != nil {
		slog.Error("fatal: open sqlite failed", "path", dbPath, "err", err)
		os.Exit(1)
	}
	defer s.Close()

	gitRepo, err := versioning.NewGitRepo(cfg.Server.DataDir)
	if err != nil {
		slog.Warn("gitops engine init warning (version audit disabled)", "err", err)
	}

	eng := assembler.New(s, gitRepo)
	// 关键：将 dbPath 作为参数传入 server.New
	srv := server.New(s, eng, gitRepo, dbPath, logLevel)

	httpServer := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: srv,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("sing-hub dual-stack service is running", "endpoint", cfg.Server.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("sing-hub shutdown gracefully completed")
}
