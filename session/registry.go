package session

import "fmt"

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
