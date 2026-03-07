package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/muxi-ai/skills-rce/pkg/cache"
	"github.com/muxi-ai/skills-rce/pkg/config"
	"github.com/muxi-ai/skills-rce/pkg/executor"
)

type Server struct {
	router    *mux.Router
	cache     *cache.Manager
	config    *config.Config
	logger    *zerolog.Logger
	startTime time.Time
	version   string
}

func NewServer(cfg *config.Config, cm *cache.Manager, logger *zerolog.Logger, version string) *Server {
	s := &Server{
		router:    mux.NewRouter(),
		cache:     cm,
		config:    cfg,
		logger:    logger,
		startTime: time.Now(),
		version:   version,
	}
	s.setupRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) setupRoutes() {
	s.router.Use(LoggingMiddleware(s.logger))
	if s.config.AuthToken != "" {
		s.router.Use(AuthMiddleware(s.config.AuthToken, s.logger))
	}

	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/run", s.handleRun).Methods("POST")
	s.router.HandleFunc("/skill/{id}", s.handleSkillUpload).Methods("POST")
	s.router.HandleFunc("/skill/{id}", s.handleSkillGet).Methods("GET")
	s.router.HandleFunc("/skill/{id}", s.handleSkillUpdate).Methods("PATCH")
	s.router.HandleFunc("/skill/{id}", s.handleSkillDelete).Methods("DELETE")
	s.router.HandleFunc("/skill/{id}/run", s.handleSkillRun).Methods("POST")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := int(time.Since(s.startTime).Seconds())
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "healthy",
		"version":       s.version,
		"languages":     []string{"python", "node", "bash"},
		"cached_skills": s.cache.List(),
		"uptime_seconds": uptime,
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string            `json:"id"`
		Language string            `json:"language"`
		Code     string            `json:"code"`
		Files    map[string]string `json:"files"`
		Timeout  int               `json:"timeout"`
		Env      map[string]string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ID == "" || req.Language == "" || req.Code == "" {
		RespondError(w, http.StatusBadRequest, "id, language, and code are required")
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = s.config.DefaultTimeout
	}
	if req.Timeout > s.config.MaxTimeout {
		req.Timeout = s.config.MaxTimeout
	}

	s.logger.Info().Str("id", req.ID).Str("language", req.Language).Msg("executing ad-hoc code")

	result := executor.Run(&executor.RunRequest{
		ID:       req.ID,
		Language: req.Language,
		Code:     req.Code,
		Files:    req.Files,
		Timeout:  req.Timeout,
		Env:      req.Env,
	})

	s.logger.Info().Str("id", req.ID).Str("status", result.Status).Int64("duration_ms", result.DurationMs).Msg("execution complete")
	RespondJSON(w, http.StatusOK, result)
}

func (s *Server) handleSkillUpload(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		Hash  string            `json:"hash"`
		Files map[string]string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Hash == "" || len(req.Files) == 0 {
		RespondError(w, http.StatusBadRequest, "hash and files are required")
		return
	}

	info, err := s.cache.Upload(id, req.Hash, req.Files)
	if err != nil {
		s.logger.Error().Err(err).Str("skill", id).Msg("skill upload failed")
		RespondError(w, http.StatusInternalServerError, "upload failed: "+err.Error())
		return
	}

	s.logger.Info().Str("skill", id).Str("hash", req.Hash).Int("files", info.FileCount).Msg("skill cached")
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"name":       info.Name,
		"hash":       info.Hash,
		"status":     "cached",
		"file_count": info.FileCount,
	})
}

func (s *Server) handleSkillGet(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	info := s.cache.Get(id)
	if info == nil {
		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"name":   id,
			"cached": false,
		})
		return
	}
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"name":       info.Name,
		"cached":     true,
		"hash":       info.Hash,
		"file_count": info.FileCount,
	})
}

func (s *Server) handleSkillUpdate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		Hash  string            `json:"hash"`
		Files map[string]string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Hash == "" || len(req.Files) == 0 {
		RespondError(w, http.StatusBadRequest, "hash and files are required")
		return
	}

	info, err := s.cache.Update(id, req.Hash, req.Files)
	if err != nil {
		s.logger.Error().Err(err).Str("skill", id).Msg("skill update failed")
		RespondError(w, http.StatusInternalServerError, "update failed: "+err.Error())
		return
	}
	if info == nil {
		RespondError(w, http.StatusNotFound, "skill not cached")
		return
	}

	s.logger.Info().Str("skill", id).Str("hash", req.Hash).Msg("skill updated")
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"name":       info.Name,
		"hash":       info.Hash,
		"status":     "updated",
		"file_count": info.FileCount,
	})
}

func (s *Server) handleSkillDelete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if !s.cache.Delete(id) {
		RespondError(w, http.StatusNotFound, "skill not cached")
		return
	}

	s.logger.Info().Str("skill", id).Msg("skill deleted")
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"name":   id,
		"status": "deleted",
	})
}

func (s *Server) handleSkillRun(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	info := s.cache.Get(id)
	if info == nil {
		RespondError(w, http.StatusNotFound, "skill not cached")
		return
	}

	var req struct {
		ID         string            `json:"id"`
		Command    string            `json:"command"`
		InputFiles map[string]string `json:"input_files"`
		Timeout    int               `json:"timeout"`
		Env        map[string]string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ID == "" || req.Command == "" {
		RespondError(w, http.StatusBadRequest, "id and command are required")
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = s.config.DefaultTimeout
	}
	if req.Timeout > s.config.MaxTimeout {
		req.Timeout = s.config.MaxTimeout
	}

	s.logger.Info().Str("id", req.ID).Str("skill", id).Str("command", req.Command).Msg("executing skill command")

	result := executor.RunSkill(&executor.SkillRunRequest{
		ID:         req.ID,
		Command:    req.Command,
		InputFiles: req.InputFiles,
		Timeout:    req.Timeout,
		Env:        req.Env,
		SkillDir:   s.cache.Dir(id),
	})

	s.logger.Info().Str("id", req.ID).Str("status", result.Status).Int64("duration_ms", result.DurationMs).Msg("skill execution complete")
	RespondJSON(w, http.StatusOK, result)
}
