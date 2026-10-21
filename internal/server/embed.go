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

// RegisterStatic 挂载静态 SPA 资源
func (s *Server) RegisterStatic() {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		slog.Error("failed to create static sub fs", "err", err)
		return
	}

	fileServer := http.FileServer(http.FS(sub))

	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// API 和订阅路由直接放行
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/sub") {
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		// 1. 如果请求的是真实物理文件（如 /favicon.png、/_app/immutable/...），且不是空路径
		if path != "" {
			if f, err := sub.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// 2. 访问根路径 / 或前端客户端路由时，统一送给 index.html
		if indexFile, err := sub.Open("index.html"); err == nil {
			_ = indexFile.Close()
			r.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r)
			return
		}

		slog.Warn("static index.html not found in embedded dist", "path", r.URL.Path)
		http.NotFound(w, r)
	})
}
