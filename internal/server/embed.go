// internal/server/embed.go
package server

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
)

//go:embed dist
var distFS embed.FS

// RegisterStatic 挂载静态 SPA 资源 (杜绝 301 重定向死循环)
func (s *Server) RegisterStatic() {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		slog.Error("failed to create static sub fs", "err", err)
		return
	}

	// 提前读入 index.html 缓存，SPA 路由直出
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

		// 1. 如果请求的是具体的静态物理文件（如 _app/immutable/...、favicon.png）
		if path != "" {
			if f, err := sub.Open(path); err == nil {
				stat, statErr := f.Stat()
				_ = f.Close()
				// 必须是普通文件而非目录，直接交给 fileServer 输出
				if statErr == nil && !stat.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
		}

		// 2. 访问根路径 / 或前端客户端路由 (如 /studio, /foundry 等)，直接吐出 index.html 内容
		if len(indexHTML) > 0 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexHTML)
			return
		}

		http.NotFound(w, r)
	})
}
