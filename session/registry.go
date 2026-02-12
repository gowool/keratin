package session

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

type Registry struct {
	sessions []*Session
	index    map[string]int
}

func NewRegistry(sessions ...*Session) *Registry {
	index := make(map[string]int)
	for i, s := range sessions {
		if _, ok := index[s.config.Cookie.Name]; ok {
			panic(fmt.Errorf("duplicate cookie name: %s", s.config.Cookie.Name))
		}
		index[s.config.Cookie.Name] = i
	}

	return &Registry{
		sessions: sessions,
		index:    index,
	}
}

func (r *Registry) All() []*Session {
	return r.sessions
}

func (r *Registry) Get(name string) *Session {
	index, ok := r.index[name]
	if !ok {
		panic(fmt.Errorf("unknown cookie name: %s", name))
	}
	return r.sessions[index]
}

func (r *Registry) ReadSessions(req *http.Request) (_ *http.Request, err error) {
	for _, s := range r.All() {
		if req, err = s.ReadSessionCookie(req); err != nil {
			return req, fmt.Errorf("session %s: %w", s.config.Cookie.Name, err)
		}
	}
	return req, nil
}

func (r *Registry) WriteSessions(w http.ResponseWriter, req *http.Request) (err error) {
	ctx := req.Context()

	for _, s := range r.All() {
		switch s.Status(ctx) {
		case Modified:
			token, expiry, err1 := s.Commit(ctx)
			if err1 != nil {
				err = errors.Join(err, fmt.Errorf("session %s: %w", s.config.Cookie.Name, err1))
			} else {
				s.WriteSessionCookie(ctx, w, token, expiry)
			}
		case Destroyed:
			s.WriteSessionCookie(ctx, w, "", time.Time{})
		default:
		}
	}

	return
}
