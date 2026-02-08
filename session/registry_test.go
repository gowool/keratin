package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRegistry(t *testing.T) {
	tests := []struct {
		name         string
		sessions     []*Session
		expectPanic  bool
		panicMessage string
	}{
		{
			name:     "valid sessions",
			sessions: []*Session{createTestSession("session1"), createTestSession("session2")},
		},
		{
			name:     "single session",
			sessions: []*Session{createTestSession("session1")},
		},
		{
			name:     "empty registry",
			sessions: []*Session{},
		},
		{
			name:         "duplicate cookie names",
			sessions:     []*Session{createTestSession("session1"), createTestSession("session1")},
			expectPanic:  true,
			panicMessage: "duplicate cookie name: session1",
		},
		{
			name:         "multiple duplicates",
			sessions:     []*Session{createTestSession("session1"), createTestSession("session2"), createTestSession("session1")},
			expectPanic:  true,
			panicMessage: "duplicate cookie name: session1",
		},
		{
			name:     "three unique sessions",
			sessions: []*Session{createTestSession("session1"), createTestSession("session2"), createTestSession("session3")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.PanicsWithError(t, tt.panicMessage, func() {
					NewRegistry(tt.sessions...)
				})
			} else {
				registry := NewRegistry(tt.sessions...)
				assert.NotNil(t, registry)
				assert.Len(t, registry.sessions, len(tt.sessions))
				assert.Len(t, registry.index, len(tt.sessions))

				for i, session := range tt.sessions {
					assert.Equal(t, session, registry.sessions[i])
					assert.Contains(t, registry.index, session.config.Cookie.Name)
					assert.Equal(t, i, registry.index[session.config.Cookie.Name])
				}
			}
		})
	}
}

func TestRegistry_All(t *testing.T) {
	tests := []struct {
		name     string
		sessions []*Session
	}{
		{
			name:     "returns all sessions",
			sessions: []*Session{createTestSession("session1"), createTestSession("session2"), createTestSession("session3")},
		},
		{
			name:     "returns single session",
			sessions: []*Session{createTestSession("session1")},
		},
		{
			name:     "returns empty slice",
			sessions: []*Session{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry(tt.sessions...)
			got := registry.All()

			assert.Len(t, got, len(tt.sessions))
			if len(tt.sessions) > 0 {
				for i, session := range tt.sessions {
					assert.Equal(t, session, got[i])
				}
			} else {
				assert.Empty(t, got)
			}
		})
	}
}

func TestRegistry_All_PreservesOrder(t *testing.T) {
	sessions := []*Session{
		createTestSession("session1"),
		createTestSession("session2"),
		createTestSession("session3"),
	}
	registry := NewRegistry(sessions...)
	got := registry.All()

	for i := range sessions {
		assert.Same(t, sessions[i], got[i], "All() should preserve order and return same instances")
	}
}

func TestRegistry_Get(t *testing.T) {
	session1 := createTestSession("session1")
	session2 := createTestSession("session2")
	session3 := createTestSession("session3")
	registry := NewRegistry(session1, session2, session3)

	tests := []struct {
		name        string
		cookieName  string
		wantSession *Session
		expectPanic bool
		panicMsg    string
	}{
		{
			name:        "get first session",
			cookieName:  "session1",
			wantSession: session1,
		},
		{
			name:        "get middle session",
			cookieName:  "session2",
			wantSession: session2,
		},
		{
			name:        "get last session",
			cookieName:  "session3",
			wantSession: session3,
		},
		{
			name:        "unknown cookie name",
			cookieName:  "unknown",
			expectPanic: true,
			panicMsg:    "unknown cookie name: unknown",
		},
		{
			name:        "empty cookie name",
			cookieName:  "",
			expectPanic: true,
			panicMsg:    "unknown cookie name: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.PanicsWithError(t, tt.panicMsg, func() {
					registry.Get(tt.cookieName)
				})
			} else {
				got := registry.Get(tt.cookieName)
				assert.NotNil(t, got)
				assert.Same(t, tt.wantSession, got, "Get() should return the same session instance")
				assert.Equal(t, tt.cookieName, got.config.Cookie.Name)
			}
		})
	}
}

func TestRegistry_Get_MultipleTimes(t *testing.T) {
	session1 := createTestSession("session1")
	session2 := createTestSession("session2")
	registry := NewRegistry(session1, session2)

	got1 := registry.Get("session1")
	got2 := registry.Get("session1")
	got3 := registry.Get("session2")

	assert.Same(t, session1, got1)
	assert.Same(t, session1, got2)
	assert.Same(t, session1, got1, "Multiple Get() calls should return same instance")
	assert.Same(t, session2, got3)
	assert.NotSame(t, got1, got3, "Different sessions should be different instances")
}

func TestRegistry_Integration(t *testing.T) {
	sessions := []*Session{
		createTestSession("default"),
		createTestSession("admin"),
		createTestSession("user"),
	}
	registry := NewRegistry(sessions...)

	t.Run("All returns correct sessions", func(t *testing.T) {
		all := registry.All()
		assert.Len(t, all, 3)
	})

	t.Run("Get each session", func(t *testing.T) {
		assert.Same(t, sessions[0], registry.Get("default"))
		assert.Same(t, sessions[1], registry.Get("admin"))
		assert.Same(t, sessions[2], registry.Get("user"))
	})

	t.Run("index is correctly populated", func(t *testing.T) {
		assert.Equal(t, 0, registry.index["default"])
		assert.Equal(t, 1, registry.index["admin"])
		assert.Equal(t, 2, registry.index["user"])
	})
}

func TestRegistry_Empty(t *testing.T) {
	registry := NewRegistry()

	assert.NotNil(t, registry)
	assert.Empty(t, registry.All())
	assert.Empty(t, registry.index)

	assert.Panics(t, func() {
		registry.Get("any")
	})
}

func TestRegistry_SpecialCookieNames(t *testing.T) {
	tests := []struct {
		name       string
		cookieName string
	}{
		{"dash in name", "session-id"},
		{"underscore in name", "session_id"},
		{"dot in name", "session.id"},
		{"uppercase", "SESSION"},
		{"mixed case", "SessionID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := createTestSession(tt.cookieName)
			registry := NewRegistry(session)

			got := registry.Get(tt.cookieName)
			assert.Same(t, session, got)
			assert.Equal(t, tt.cookieName, got.config.Cookie.Name)
		})
	}
}

func createTestSession(cookieName string) *Session {
	config := Config{
		Cookie: Cookie{
			Name: cookieName,
		},
	}
	return New(config, &MockStore{})
}
