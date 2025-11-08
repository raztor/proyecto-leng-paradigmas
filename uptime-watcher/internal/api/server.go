package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"proyecto-leng-paradigmas/ejemplo/internal/db"
	"proyecto-leng-paradigmas/ejemplo/internal/model"
	"proyecto-leng-paradigmas/ejemplo/internal/service"
)

// Server expone endpoints HTTP para consultar y administrar el monitor.
type Server struct {
	svc *service.TargetService
	mux *http.ServeMux
}

// New crea un servidor API y registra los handlers necesarios.
func New(service *service.TargetService) *Server {
	s := &Server{
		svc: service,
		mux: http.NewServeMux(),
	}
	s.routes()
	return s
}

// Handler devuelve el multiplexer HTTP principal.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/targets", s.handleTargets)
	s.mux.HandleFunc("/api/targets/", s.handleTargetByID)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/history", s.handleHistory)
	s.mux.HandleFunc("/api/refresh", s.handleRefresh)
	s.mux.HandleFunc("/healthz", s.handleHealth)
}

func (s *Server) handleTargets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.svc.ListTargets())
	case http.MethodPost:
		s.createTarget(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTargetByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/targets/")
	id = strings.Trim(id, "/")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut, http.MethodPatch:
		s.updateTarget(w, r, id)
	case http.MethodDelete:
		s.deleteTarget(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) createTarget(w http.ResponseWriter, r *http.Request) {
	var req targetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido: "+err.Error())
		return
	}
	target, err := requestToTarget(req, "")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := s.svc.CreateTarget(r.Context(), target)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) updateTarget(w http.ResponseWriter, r *http.Request, id string) {
	var req targetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido: "+err.Error())
		return
	}
	target, err := requestToTarget(req, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.svc.UpdateTarget(r.Context(), target)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, db.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) deleteTarget(w http.ResponseWriter, r *http.Request, id string) {
	err := s.svc.DeleteTarget(r.Context(), id)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, db.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.svc.Status())
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	var limit int
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	results, err := s.svc.History(id, limit)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if ok := s.svc.Trigger(id); !ok {
		writeError(w, http.StatusNotFound, "target not found")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "triggered"})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type targetRequest struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Frequency string `json:"frequency"`
	Timeout   string `json:"timeout"`
}

func requestToTarget(req targetRequest, pathID string) (model.Target, error) {
	id := strings.TrimSpace(req.ID)
	if pathID != "" {
		id = pathID
	}
	if id == "" && pathID != "" {
		return model.Target{}, errors.New("id requerido")
	}
	freq, timeout, err := service.ParseDurations(req.Frequency, req.Timeout)
	if err != nil {
		return model.Target{}, err
	}
	target := model.Target{
		ID:        id,
		Name:      strings.TrimSpace(req.Name),
		Kind:      model.TargetKind(strings.ToLower(strings.TrimSpace(req.Kind))),
		URL:       strings.TrimSpace(req.URL),
		Host:      strings.TrimSpace(req.Host),
		Port:      req.Port,
		Frequency: freq,
		Timeout:   timeout,
	}
	return target, nil
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
