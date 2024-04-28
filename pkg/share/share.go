package share

import (
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/nimbolus/terraform-backend/pkg/auth"
	"github.com/nimbolus/terraform-backend/pkg/server"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
	log "github.com/sirupsen/logrus"
)

type session struct {
	contentType string
	src         io.Reader
	done        chan struct{}
}

type Server struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func NewServer() *Server {
	return &Server{
		sessions: make(map[string]*session),
	}
}

func (s *Server) takeSession(id string) (*session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
	}
	return sess, ok
}

var ErrSessionExists = errors.New("session already exists")

func (s *Server) startSession(id, contentType string, src io.Reader) (<-chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.sessions[id]
	if ok {
		return nil, ErrSessionExists
	}

	done := make(chan struct{})
	sess := &session{
		contentType: contentType,
		src:         src,
		done:        done,
	}
	s.sessions[id] = sess

	return done, nil
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	state := &terraform.State{
		ID:      terraform.GetStateID(vars["project"], vars["name"]),
		Project: vars["project"],
		Name:    vars["name"],
	}
	if ok, err := auth.Authenticate(req, state); err != nil {
		log.Warnf("failed process authentication for state id %s: %v", state.ID, err)
		server.HTTPResponse(rw, req, http.StatusForbidden, err.Error())
		return
	} else if !ok {
		log.Warnf("failed to authenticate request for state id %s", state.ID)
		server.HTTPResponse(rw, req, http.StatusForbidden, "Permission denied")
		return
	}

	id, ok := vars["id"]
	if !ok || id == "" {
		server.HTTPResponse(rw, req, http.StatusNotFound, "Missing ID param")
		return
	}

	if req.Method == http.MethodGet {
		sess, ok := s.takeSession(id)
		if !ok {
			server.HTTPResponse(rw, req, http.StatusNotFound, "Session not found")
			return
		}

		defer close(sess.done)

		rw.Header().Set("Content-Type", sess.contentType)
		rw.WriteHeader(http.StatusOK)

		_, _ = io.Copy(rw, sess.src)
		return
	}

	if req.Method == http.MethodPost {
		done, err := s.startSession(id, req.Header.Get("Content-Type"), req.Body)
		if err != nil {
			if errors.Is(err, ErrSessionExists) {
				server.HTTPResponse(rw, req, http.StatusConflict, "Session already exists")
				return
			}
			server.HTTPResponse(rw, req, http.StatusInternalServerError, err.Error())
			return
		}

		select {
		case <-req.Context().Done():
			return
		case <-done:
			rw.WriteHeader(http.StatusAccepted)
			return
		}
	}

	server.HTTPResponse(rw, req, http.StatusMethodNotAllowed, "invalid method, expected GET or POST")
}
