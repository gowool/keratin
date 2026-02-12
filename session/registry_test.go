package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestRegistry_ReadSessions(t *testing.T) {
	session1 := createTestSession("session1")
	session2 := createTestSession("session2")
	session3 := createTestSession("session3")

	tests := []struct {
		name        string
		sessions    []*Session
		setup       func(*testing.T, *http.Request)
		expectError bool
		errorMsg    string
		cookieCount int
	}{
		{
			name:     "read all sessions successfully",
			sessions: []*Session{session1, session2, session3},
			setup: func(t *testing.T, req *http.Request) {
				req.AddCookie(&http.Cookie{Name: "session1", Value: "token1"})
				req.AddCookie(&http.Cookie{Name: "session2", Value: "token2"})
				req.AddCookie(&http.Cookie{Name: "session3", Value: "token3"})
			},
			cookieCount: 3,
		},
		{
			name:     "read with partial cookies",
			sessions: []*Session{session1, session2, session3},
			setup: func(t *testing.T, req *http.Request) {
				req.AddCookie(&http.Cookie{Name: "session2", Value: "token2"})
			},
			cookieCount: 1,
		},
		{
			name:        "read with no cookies",
			sessions:    []*Session{session1, session2, session3},
			setup:       func(t *testing.T, req *http.Request) {},
			cookieCount: 0,
		},
		{
			name:        "empty registry reads successfully",
			sessions:    []*Session{},
			setup:       func(t *testing.T, req *http.Request) {},
			cookieCount: 0,
		},
		{
			name:     "single session",
			sessions: []*Session{session1},
			setup: func(t *testing.T, req *http.Request) {
				req.AddCookie(&http.Cookie{Name: "session1", Value: "token1"})
			},
			cookieCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(MockStore)
			mockStore.On("Find", mock.Anything, mock.Anything).Return([]byte{}, false, nil)

			registry := NewRegistry(tt.sessions...)

			for _, s := range registry.All() {
				s.store = mockStore
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(t, req)

			got, err := registry.ReadSessions(req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}

			if len(tt.sessions) > 0 {
				mockStore.AssertNumberOfCalls(t, "Find", tt.cookieCount)
			}
		})
	}
}

func TestRegistry_WriteSessions(t *testing.T) {
	tests := []struct {
		name          string
		sessions      []*Session
		modifySession func(*testing.T, *http.Request, *Registry)
		expectError   bool
		verifyCookies func(*testing.T, *http.Response)
	}{
		{
			name:     "write modified session",
			sessions: []*Session{createTestSession("session1")},
			modifySession: func(t *testing.T, req *http.Request, registry *Registry) {
				registry.All()[0].Put(req.Context(), "key", "value")
			},
			verifyCookies: func(t *testing.T, resp *http.Response) {
				cookies := resp.Cookies()
				assert.Len(t, cookies, 1)
				assert.Equal(t, "session1", cookies[0].Name)
			},
		},
		{
			name:     "write destroyed session",
			sessions: []*Session{createTestSession("session1")},
			modifySession: func(t *testing.T, req *http.Request, registry *Registry) {
				err := registry.All()[0].Destroy(req.Context())
				assert.NoError(t, err)
			},
			verifyCookies: func(t *testing.T, resp *http.Response) {
				cookies := resp.Cookies()
				assert.Len(t, cookies, 1)
				assert.Equal(t, "session1", cookies[0].Name)
				assert.Equal(t, -1, cookies[0].MaxAge)
			},
		},
		{
			name:          "unmodified session - default case",
			sessions:      []*Session{createTestSession("session1")},
			modifySession: func(t *testing.T, req *http.Request, registry *Registry) {},
			verifyCookies: func(t *testing.T, resp *http.Response) {
				cookies := resp.Cookies()
				assert.Len(t, cookies, 0)
			},
		},
		{
			name:     "multiple sessions with mixed statuses",
			sessions: []*Session{createTestSession("session1"), createTestSession("session2"), createTestSession("session3")},
			modifySession: func(t *testing.T, req *http.Request, registry *Registry) {
				sessions := registry.All()
				sessions[0].Put(req.Context(), "key1", "value1")
				err := sessions[1].Destroy(req.Context())
				assert.NoError(t, err)
			},
			verifyCookies: func(t *testing.T, resp *http.Response) {
				cookies := resp.Cookies()
				assert.Len(t, cookies, 2)
			},
		},
		{
			name:          "empty registry writes successfully",
			sessions:      []*Session{},
			modifySession: func(t *testing.T, req *http.Request, registry *Registry) {},
			verifyCookies: func(t *testing.T, resp *http.Response) {
				cookies := resp.Cookies()
				assert.Len(t, cookies, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(MockStore)
			mockStore.On("Find", mock.Anything, mock.Anything).Return([]byte{}, false, nil)
			mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockStore.On("Delete", mock.Anything, mock.Anything).Return(nil)

			registry := NewRegistry(tt.sessions...)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			for _, s := range registry.All() {
				s.store = mockStore
			}

			req, err := registry.ReadSessions(req)
			assert.NoError(t, err)

			tt.modifySession(t, req, registry)

			err = registry.WriteSessions(w, req)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			tt.verifyCookies(t, w.Result())
		})
	}
}

func TestRegistry_WriteSessions_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		modifySession func(*testing.T, *http.Request, *Registry)
		errorContains string
	}{
		{
			name: "commit error is joined and returned",
			modifySession: func(t *testing.T, req *http.Request, registry *Registry) {
				registry.All()[0].Put(req.Context(), "key", "value")
			},
			errorContains: "session session1",
		},
		{
			name: "multiple commit errors are joined",
			modifySession: func(t *testing.T, req *http.Request, registry *Registry) {
				sessions := registry.All()
				sessions[0].Put(req.Context(), "key1", "value1")
				sessions[1].Put(req.Context(), "key2", "value2")
			},
			errorContains: "session session1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(MockStore)
			mockStore.On("Find", mock.Anything, mock.Anything).Return([]byte{}, false, nil)
			mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)

			registry := NewRegistry(createTestSession("session1"), createTestSession("session2"))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			for _, s := range registry.All() {
				s.store = mockStore
			}

			req, err := registry.ReadSessions(req)
			assert.NoError(t, err)

			tt.modifySession(t, req, registry)

			err = registry.WriteSessions(w, req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

func TestRegistry_Integration_ReadWriteSessions(t *testing.T) {
	mockStore := new(MockStore)
	mockStore.On("Find", mock.Anything, mock.Anything).Return([]byte{}, false, nil)
	mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	sessions := []*Session{createTestSession("session1"), createTestSession("session2")}
	registry := NewRegistry(sessions...)

	for _, s := range registry.All() {
		s.store = mockStore
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	req, err := registry.ReadSessions(req)
	assert.NoError(t, err)

	sessions[0].Put(req.Context(), "key1", "value1")

	err = registry.WriteSessions(w, req)
	assert.NoError(t, err)

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "session1", cookies[0].Name)
}

func TestRegistry_WriteSessions_DestroyedExpiry(t *testing.T) {
	mockStore := new(MockStore)
	mockStore.On("Find", mock.Anything, mock.Anything).Return([]byte{}, false, nil)
	mockStore.On("Delete", mock.Anything, mock.Anything).Return(nil)

	session := createTestSession("session1")
	session.store = mockStore

	registry := NewRegistry(session)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	req, err := registry.ReadSessions(req)
	assert.NoError(t, err)

	err = session.Destroy(req.Context())
	assert.NoError(t, err)

	err = registry.WriteSessions(w, req)
	assert.NoError(t, err)

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, -1, cookies[0].MaxAge)
	assert.True(t, cookies[0].Expires.Equal(time.Unix(1, 0).UTC()))
}

func createTestSession(cookieName string) *Session {
	config := Config{
		Cookie: Cookie{
			Name: cookieName,
		},
	}
	return New(config, &MockStore{})
}
