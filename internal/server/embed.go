// internal/server/embed.go
package server

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
)

// 核心修复：必须加 all: 前缀，强制嵌入下划线开头的 _app 静态资源目录！
//
//go:embed all:dist
var distFS embed.FS

func (s *Server) RegisterStatic() {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		slog.Error("failed to create static sub fs", "err", err)
		return
	}

	indexHTML, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		slog.Warn("index.html not found in dist, static frontend may fail", "err", err)
	}

	fileServer := http.FileServer(http.FS(sub))

	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// API 和订阅路由直接放行
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/sub") {
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")

		// 1. 如果请求具体的静态资源 (包含 _app/immutable/... 等)
		if path != "" {
			if f, err := sub.Open(path); err == nil {
				stat, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil && !stat.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
		}

		// 2. SPA 路由直出 index.html
		if len(indexHTML) > 0 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexHTML)
			return
		}

		http.NotFound(w, r)
	})
}
