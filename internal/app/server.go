package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"twentyfive/internal/assets"
)

type Server struct {
	store        *Store
	mux          *http.ServeMux
	indexHandler http.Handler
}

func NewServer(store *Store) *Server {
	s := &Server{
		store:        store,
		mux:          http.NewServeMux(),
		indexHandler: assets.IndexHandler(),
	}

	s.mux.HandleFunc("/api/board", s.handleBoard)
	s.mux.HandleFunc("/api/tasks", s.handleTasks)
	s.mux.HandleFunc("/api/tasks/", s.handleTaskByID)
	s.mux.HandleFunc("/api/categories", s.handleCategories)
	s.mux.HandleFunc("/api/categories/", s.handleCategoryByID)
	s.mux.HandleFunc("/api/board/focus", s.handleFocus)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		s.mux.ServeHTTP(w, r)
		return
	}
	s.indexHandler.ServeHTTP(w, r)
}

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		state := s.store.GetState()
		writeJSON(w, http.StatusOK, state)
	default:
		methodNotAllowed(w, http.MethodGet)
	}
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req CreateTaskRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		task, board, err := s.store.CreateTask(req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"task":  task,
			"board": board,
		})
	default:
		methodNotAllowed(w, http.MethodPost)
	}
}

func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(path, "/move") {
		id := strings.TrimSuffix(path, "/move")
		id = strings.TrimSuffix(id, "/")
		s.handleMoveTask(w, r, id)
		return
	}

	id := strings.Trim(path, "/")
	switch r.Method {
	case http.MethodPatch:
		var patch TaskPatch
		if err := decodeJSON(r, &patch); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		task, board, err := s.store.UpdateTask(id, patch)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"task":  task,
			"board": board,
		})
	case http.MethodDelete:
		board, err := s.store.DeleteTask(id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"board": board,
		})
	default:
		methodNotAllowed(w, http.MethodPatch, http.MethodDelete)
	}
}

func (s *Server) handleMoveTask(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var req MoveTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	task, board, err := s.store.MoveTask(id, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"task":  task,
		"board": board,
	})
}

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		cat, board, err := s.store.CreateCategory(payload.Name)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"category": cat,
			"board":    board,
		})
	default:
		methodNotAllowed(w, http.MethodPost)
	}
}

func (s *Server) handleCategoryByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/categories/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(path, "/move") {
		id := strings.TrimSuffix(path, "/move")
		id = strings.TrimSuffix(id, "/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		s.handleMoveCategory(w, r, id)
		return
	}
	id := strings.Trim(path, "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var patch CategoryPatch
		if err := decodeJSON(r, &patch); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		var (
			board BoardState
			cat   Category
			err   error
		)
		if patch.Name != nil {
			cat, board, err = s.store.RenameCategory(id, *patch.Name)
			if err != nil {
				writeDomainError(w, err)
				return
			}
		}
		if patch.Order != nil {
			cat, board, err = s.store.ReorderCategoryTasks(id, patch.Order)
			if err != nil {
				writeDomainError(w, err)
				return
			}
		}
		if patch.Name == nil && patch.Order == nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("%w: no fields to update", ErrInvalidRequest))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"category": cat,
			"board":    board,
		})
	default:
		methodNotAllowed(w, http.MethodPatch)
	}
}

func (s *Server) handleMoveCategory(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var req MoveCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cat, board, err := s.store.MoveCategory(id, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"category": cat,
		"board":    board,
	})
}

func (s *Server) handleFocus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var req FocusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	task, board, err := s.store.SetFocused(req.TaskID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"task":  task,
		"board": board,
	})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest),
		errors.Is(err, ErrInvalidState),
		errors.Is(err, ErrInvalidLocation),
		errors.Is(err, ErrInvalidTaskSize):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, ErrTaskNotFound),
		errors.Is(err, ErrCategoryNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, ErrCapacityExceeded),
		errors.Is(err, ErrCategoryLimit):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, ErrDuplicateCategory):
		writeError(w, http.StatusConflict, err)
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, errors.New("internal server error"))
	}
}
