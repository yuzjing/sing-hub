package main

import (
	"context"
	"flag"
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
	} else if strings.ToLower(cfg.Server.LogLevel) == "warn" {
		logLevel.Set(slog.LevelWarn)
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

	binDir := filepath.Join(cfg.Server.DataDir, "bin")
	eng := assembler.New(s, gitRepo, binDir)
	upd := updater.New(s, binDir)
	// 👑 彻底剔除 upd.StartPeriodicTask()，全系统纯按需驱动，零后台协程空转！

	srv := server.New(s, eng, gitRepo, upd, dbPath, logLevel)

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
	slog.Info("shutting down sing-hub gracefully...")

	// 优雅关机超时控制：给正在处理的请求 5 秒退出时间，确保 SQLite 事务提交
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server forced to shutdown", "err", err)
	}
	slog.Info("sing-hub shutdown gracefully completed")
}
