// cmd/sing-hub/main.go
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"sing-hub/internal/assembler"
	"sing-hub/internal/server"
	"sing-hub/internal/store"
)

func main() {
	// 命令行参数
	addr := flag.String("addr", ":8080", "HTTP service listen address")
	dbPath := flag.String("db", "./data.db", "SQLite database file path")
	flag.Parse()

	// 结构化日志
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("starting sing-hub engine", "addr", *addr, "db", *dbPath)

	// 1. 初始化持久层
	s, err := store.New(*dbPath)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	// 2. 初始化拼装引擎
	eng := assembler.New(s)

	// 3. 初始化 HTTP 服务
	srv := server.New(s, eng)

	// 4. 优雅停机 (Graceful Shutdown) 监听
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	httpServer := &http.Server{
		Addr:    *addr,
		Handler: srv,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	slog.Info("sing-hub is running and ready for traffic", "endpoint", "http://localhost"+*addr)

	<-stop
	slog.Info("shutting down sing-hub gracefully...")
}
