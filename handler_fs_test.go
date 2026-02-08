package keratin

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentSecurityPolicy(t *testing.T) {
	tests := []struct {
		name        string
		data        []string
		expectedCSP string
	}{
		{
			name:        "default CSP",
			data:        nil,
			expectedCSP: "default-src 'none'; connect-src 'self'; image-src 'self'; media-src 'self'; style-src 'unsafe-inline'; sandbox",
		},
		{
			name:        "empty data returns default",
			data:        []string{},
			expectedCSP: "default-src 'none'; connect-src 'self'; image-src 'self'; media-src 'self'; style-src 'unsafe-inline'; sandbox",
		},
		{
			name:        "custom CSP",
			data:        []string{"default-src 'self'", "script-src 'unsafe-inline'"},
			expectedCSP: "default-src 'self'; script-src 'unsafe-inline'",
		},
		{
			name:        "single custom directive",
			data:        []string{"default-src 'none'"},
			expectedCSP: "default-src 'none'",
		},
		{
			name:        "multiple directives with custom policy",
			data:        []string{"default-src 'self'", "img-src 'self'", "script-src 'none'"},
			expectedCSP: "default-src 'self'; img-src 'self'; script-src 'none'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cspFunc := ContentSecurityPolicy(tt.data...)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			cspFunc(w, r, nil, nil)

			assert.Equal(t, tt.expectedCSP, w.Header().Get("Content-Security-Policy"))
		})
	}
}

func TestCacheControl(t *testing.T) {
	tests := []struct {
		name                 string
		data                 []string
		expectedCacheControl string
	}{
		{
			name:                 "default cache control",
			data:                 nil,
			expectedCacheControl: "max-age=2592000, stale-while-revalidate=86400",
		},
		{
			name:                 "empty data returns default",
			data:                 []string{},
			expectedCacheControl: "max-age=2592000, stale-while-revalidate=86400",
		},
		{
			name:                 "custom cache control",
			data:                 []string{"no-cache", "no-store"},
			expectedCacheControl: "no-cache, no-store",
		},
		{
			name:                 "single custom directive",
			data:                 []string{"max-age=3600"},
			expectedCacheControl: "max-age=3600",
		},
		{
			name:                 "multiple directives",
			data:                 []string{"public", "max-age=31536000", "immutable"},
			expectedCacheControl: "public, max-age=31536000, immutable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheFunc := CacheControl(tt.data...)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			cacheFunc(w, r, nil, nil)

			assert.Equal(t, tt.expectedCacheControl, w.Header().Get("Cache-Control"))
		})
	}
}

func TestAttachment(t *testing.T) {
	tests := []struct {
		name                string
		filename            string
		expectedDisposition string
	}{
		{
			name:                "simple filename",
			filename:            "document.pdf",
			expectedDisposition: `attachment; filename="document.pdf"`,
		},
		{
			name:                "filename with spaces",
			filename:            "my document.pdf",
			expectedDisposition: `attachment; filename="my document.pdf"`,
		},
		{
			name:                "filename with special chars",
			filename:            `file"name.pdf`,
			expectedDisposition: `attachment; filename="file\"name.pdf"`,
		},
		{
			name:                "filename with backslash",
			filename:            `path\file.pdf`,
			expectedDisposition: `attachment; filename="path\\file.pdf"`,
		},
		{
			name:                "filename with unicode",
			filename:            "文档.pdf",
			expectedDisposition: `attachment; filename="文档.pdf"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attachmentFunc := Attachment(tt.filename)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			attachmentFunc(w, r, nil, nil)

			assert.Equal(t, tt.expectedDisposition, w.Header().Get("Content-Disposition"))
		})
	}
}

func TestInline(t *testing.T) {
	tests := []struct {
		name                string
		filename            string
		expectedDisposition string
	}{
		{
			name:                "simple filename",
			filename:            "image.png",
			expectedDisposition: `inline; filename="image.png"`,
		},
		{
			name:                "filename with spaces",
			filename:            "my image.png",
			expectedDisposition: `inline; filename="my image.png"`,
		},
		{
			name:                "filename with special chars",
			filename:            `file"name.png`,
			expectedDisposition: `inline; filename="file\"name.png"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inlineFunc := Inline(tt.filename)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			inlineFunc(w, r, nil, nil)

			assert.Equal(t, tt.expectedDisposition, w.Header().Get("Content-Disposition"))
		})
	}
}

func TestFileFS(t *testing.T) {
	t.Run("nil fsys panics", func(t *testing.T) {
		assert.Panics(t, func() {
			FileFS(nil, "test.txt")
		})
	})

	t.Run("empty filename panics", func(t *testing.T) {
		fsys := os.DirFS(".")
		assert.Panics(t, func() {
			FileFS(fsys, "")
		})
	})

	t.Run("serves existing file", func(t *testing.T) {
		fsys := os.DirFS(".")
		handler := FileFS(fsys, "router.go")

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		err := handler.ServeHTTP(w, r)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Body.String())
		assert.Contains(t, w.Body.String(), "package keratin")
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		fsys := os.DirFS(".")
		handler := FileFS(fsys, "nonexistent.txt")

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		err := handler.ServeHTTP(w, r)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrFileNotFound))
	})

	t.Run("serves directory index.html", func(t *testing.T) {
		tmpDir := t.TempDir()

		indexFile := tmpDir + "/index.html"
		err := os.WriteFile(indexFile, []byte("<html>index</html>"), 0644)
		require.NoError(t, err)

		fsys := os.DirFS(tmpDir)
		handler := FileFS(fsys, ".")

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		err = handler.ServeHTTP(w, r)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "<html>index</html>", w.Body.String())
	})

	t.Run("applies beforeServe functions", func(t *testing.T) {
		fsys := os.DirFS(".")
		handler := FileFS(fsys, "router.go",
			ContentSecurityPolicy("default-src 'self'"),
			CacheControl("no-cache"),
		)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		err := handler.ServeHTTP(w, r)

		assert.NoError(t, err)
		assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	})
}

func TestStaticFS(t *testing.T) {
	t.Run("serves static file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := tmpDir + "/test.txt"
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		require.NoError(t, err)

		fsys := os.DirFS(tmpDir)
		handler := StaticFS(fsys, "path", false)

		router := NewRouter(ErrorHandlerFunc(func(w http.ResponseWriter, r *http.Request, err error) {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		router.GET("/files/{path...}", handler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/files/test.txt", nil)

		router.Build().ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "test content", w.Body.String())
	})

	t.Run("returns 404 for non-existent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		fsys := os.DirFS(tmpDir)
		handler := StaticFS(fsys, "path", false)

		router := NewRouter(ErrorHandlerFunc(func(w http.ResponseWriter, r *http.Request, err error) {
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		router.GET("/files/{path...}", handler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/files/nonexistent.txt", nil)

		router.Build().ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("handles missing files correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		fsys := os.DirFS(tmpDir)
		handler := StaticFS(fsys, "path", false)

		router := NewRouter(ErrorHandlerFunc(func(w http.ResponseWriter, r *http.Request, err error) {
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		router.GET("/files/{path...}", handler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/files/nonexistent.txt", nil)

		router.Build().ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("redirects directory without trailing slash", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := tmpDir + "/dir"
		err := os.Mkdir(subDir, 0755)
		require.NoError(t, err)

		fsys := os.DirFS(tmpDir)
		handler := StaticFS(fsys, "path", false)

		router := NewRouter(ErrorHandlerFunc(func(w http.ResponseWriter, r *http.Request, err error) {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		router.GET("/files/{path...}", handler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/files/dir", nil)

		router.Build().ServeHTTP(w, r)

		assert.Equal(t, http.StatusMovedPermanently, w.Code)
		assert.Equal(t, "/files/dir/", w.Header().Get("Location"))
	})

	t.Run("redirects file with trailing slash", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := tmpDir + "/test.txt"
		err := os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		fsys := os.DirFS(tmpDir)
		handler := StaticFS(fsys, "path", false)

		router := NewRouter(ErrorHandlerFunc(func(w http.ResponseWriter, r *http.Request, err error) {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		router.GET("/files/{path...}", handler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/files/test.txt/", nil)

		router.Build().ServeHTTP(w, r)

		assert.Equal(t, http.StatusMovedPermanently, w.Code)
		assert.Equal(t, "/files/test.txt", w.Header().Get("Location"))
	})

	t.Run("redirects index.html without suffix", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := tmpDir + "/index.html"
		err := os.WriteFile(testFile, []byte("<html></html>"), 0644)
		require.NoError(t, err)

		fsys := os.DirFS(tmpDir)
		handler := StaticFS(fsys, "path", false)

		router := NewRouter(ErrorHandlerFunc(func(w http.ResponseWriter, r *http.Request, err error) {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		router.GET("/files/{path...}", handler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/files/index.html", nil)

		router.Build().ServeHTTP(w, r)

		assert.Equal(t, http.StatusMovedPermanently, w.Code)
		assert.Equal(t, "/files/", w.Header().Get("Location"))
	})

	t.Run("applies beforeServe functions", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := tmpDir + "/test.txt"
		err := os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		fsys := os.DirFS(tmpDir)
		handler := StaticFS(fsys, "path", false,
			ContentSecurityPolicy("default-src 'self'"),
			CacheControl("no-cache"),
		)

		router := NewRouter(ErrorHandlerFunc(func(w http.ResponseWriter, r *http.Request, err error) {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		router.GET("/files/{path...}", handler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/files/test.txt", nil)

		router.Build().ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	})
}

func TestStaticFS_IndexFallback(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(string) error
		path           string
		indexFallback  bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "falls back to index.html for missing file",
			setup: func(tmpDir string) error {
				return os.WriteFile(tmpDir+"/index.html", []byte("<html>fallback</html>"), 0644)
			},
			path:           "/files/nonexistent",
			indexFallback:  true,
			expectedStatus: http.StatusOK,
			expectedBody:   "<html>fallback</html>",
		},
		{
			name:           "returns 404 without fallback",
			setup:          func(tmpDir string) error { return nil },
			path:           "/files/nonexistent",
			indexFallback:  false,
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "serves existing file with fallback enabled",
			setup: func(tmpDir string) error {
				return os.WriteFile(tmpDir+"/test.txt", []byte("existing"), 0644)
			},
			path:           "/files/test.txt",
			indexFallback:  true,
			expectedStatus: http.StatusOK,
			expectedBody:   "existing",
		},
		{
			name: "handles missing file with fallback",
			setup: func(tmpDir string) error {
				return os.WriteFile(tmpDir+"/index.html", []byte("<html>index</html>"), 0644)
			},
			path:           "/files/missing.html",
			indexFallback:  true,
			expectedStatus: http.StatusOK,
			expectedBody:   "<html>index</html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if tt.setup != nil {
				err := tt.setup(tmpDir)
				require.NoError(t, err)
			}

			fsys := os.DirFS(tmpDir)
			handler := StaticFS(fsys, "path", tt.indexFallback)

			router := NewRouter(ErrorHandlerFunc(func(w http.ResponseWriter, r *http.Request, err error) {
				if err != nil {
					if errors.Is(err, ErrFileNotFound) {
						w.WriteHeader(http.StatusNotFound)
					} else {
						w.WriteHeader(http.StatusInternalServerError)
					}
				}
			}))
			router.GET("/files/{path...}", handler)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)

			router.Build().ServeHTTP(w, r)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestSafeRedirectPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "normal path unchanged",
			path:     "/test/path",
			expected: "/test/path",
		},
		{
			name:     "single slash unchanged",
			path:     "/",
			expected: "/",
		},
		{
			name:     "double slash normalized",
			path:     "//test/path",
			expected: "/test/path",
		},
		{
			name:     "backslash forward slash normalized",
			path:     `\\/test/path`,
			expected: "/test/path",
		},
		{
			name:     "multiple leading slashes normalized",
			path:     "///test/path",
			expected: "/test/path",
		},
		{
			name:     "empty path unchanged",
			path:     "",
			expected: "",
		},
		{
			name:     "path without leading slash unchanged",
			path:     "test/path",
			expected: "test/path",
		},
		{
			name:     "single character path unchanged",
			path:     "/",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeRedirectPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileFS_EmbeddedFS(t *testing.T) {
	tests := []struct {
		name          string
		setupFS       func() fs.FS
		filename      string
		expectError   bool
		expectedError error
	}{
		{
			name: "serves file from embedded FS",
			setupFS: func() fs.FS {
				return os.DirFS(".")
			},
			filename:    "router.go",
			expectError: false,
		},
		{
			name: "returns error for missing file",
			setupFS: func() fs.FS {
				return os.DirFS(".")
			},
			filename:      "missing.go",
			expectError:   true,
			expectedError: ErrFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := tt.setupFS()
			handler := FileFS(fsys, tt.filename)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			err := handler.ServeHTTP(w, r)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.True(t, errors.Is(err, tt.expectedError))
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestFileFS_ReadSeeker(t *testing.T) {
	t.Run("file implements ReadSeeker", func(t *testing.T) {
		fsys := os.DirFS(".")
		f, err := fsys.Open("router.go")
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		_, ok := f.(io.ReadSeeker)
		assert.True(t, ok, "file should implement io.ReadSeeker")
	})

	t.Run("serves with Seek functionality", func(t *testing.T) {
		fsys := os.DirFS(".")
		handler := FileFS(fsys, "router.go")

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Range", "bytes=0-100")

		err := handler.ServeHTTP(w, r)

		assert.NoError(t, err)
		assert.NotEmpty(t, w.Body.String())
	})
}
