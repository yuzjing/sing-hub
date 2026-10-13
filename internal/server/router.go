// internal/server/router.go
package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"sing-hub/internal/assembler"
	"sing-hub/internal/model"
	"sing-hub/internal/store"
)

type Server struct {
	store     *store.Store
	assembler *assembler.Engine
	mux       *http.ServeMux
}

func New(s *store.Store, a *assembler.Engine) *Server {
	srv := &Server{
		store:     s,
		assembler: a,
		mux:       http.NewServeMux(),
	}
	srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// --- 客户端订阅专用端点 (电视/VPS 访问) ---
	s.mux.HandleFunc("GET /sub/{token}", s.handleSubscribe)

	// --- 实时预览编译结果 ---
	s.mux.HandleFunc("GET /api/profiles/{id}/compile", s.handleCompileProfile)

	// --- 全局变量 API ---
	s.mux.HandleFunc("GET /api/variables", s.handleListVariables)
	s.mux.HandleFunc("POST /api/variables", s.handleSetVariable)
	s.mux.HandleFunc("DELETE /api/variables/{key}", s.handleDeleteVariable)

	// --- 组件资产 API ---
	s.mux.HandleFunc("GET /api/components", s.handleListComponents)
	s.mux.HandleFunc("POST /api/components", s.handleSaveComponent)
	s.mux.HandleFunc("DELETE /api/components/{id}", s.handleDeleteComponent)

	// --- 设备档案 API ---
	s.mux.HandleFunc("GET /api/profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /api/profiles/{id}", s.handleGetProfile)
	s.mux.HandleFunc("POST /api/profiles", s.handleSaveProfile)
	s.mux.HandleFunc("DELETE /api/profiles/{id}", s.handleDeleteProfile)
}

// ==================== 订阅与编译处理器 ====================

// handleSubscribe 电视端或 VPS 客户端定时拉取配置的出口
func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	profile, err := s.store.GetProfileByToken(r.Context(), token)
	if err != nil {
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}

	data, err := s.assembler.Assemble(r.Context(), profile.ID)
	if err != nil {
		slog.Error("assemble profile failed", "profile_id", profile.ID, "err", err)
		http.Error(w, "internal assemble error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleCompileProfile WebUI 点击“实时编译预览”调用的接口
func (s *Server) handleCompileProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, err := s.assembler.Assemble(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ==================== RESTful 处理器 ====================

func (s *Server) handleListVariables(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListVariables(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSetVariable(w http.ResponseWriter, r *http.Request) {
	var v model.Variable
	if err := readJSON(r, &v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SetVariable(r.Context(), v); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteVariable(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if err := s.store.DeleteVariable(r.Context(), key); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListComponents(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListComponents(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSaveComponent(w http.ResponseWriter, r *http.Request) {
	var c model.Component
	if err := readJSON(r, &c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SaveComponent(r.Context(), c); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteComponent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteComponent(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListProfiles(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.store.GetProfile(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	var p model.Profile
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SaveProfile(r.Context(), p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteProfile(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ==================== 辅助泛型工具 ====================

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func readJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1024*1024)).Decode(target) // 限制最大 1MB payload
}
