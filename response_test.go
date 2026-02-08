package keratin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockWriterWithUnwrap struct {
	http.ResponseWriter
	inner http.ResponseWriter
}

func (m *mockWriterWithUnwrap) Unwrap() http.ResponseWriter {
	return m.inner
}

type mockStatusCoder struct {
	http.ResponseWriter
	statusCode int
}

func (m *mockStatusCoder) StatusCode() int {
	return m.statusCode
}

func (m *mockStatusCoder) Unwrap() http.ResponseWriter {
	return m.ResponseWriter
}

type mockSizer struct {
	http.ResponseWriter
	size int64
}

func (m *mockSizer) Size() int64 {
	return m.size
}

func (m *mockSizer) Unwrap() http.ResponseWriter {
	return m.ResponseWriter
}

type mockCommitter struct {
	http.ResponseWriter
	committed bool
}

func (m *mockCommitter) Committed() bool {
	return m.committed
}

func (m *mockCommitter) Unwrap() http.ResponseWriter {
	return m.ResponseWriter
}

type mockReaderFrom struct {
	http.ResponseWriter
	readFromCalled bool
}

func (m *mockReaderFrom) ReadFrom(r io.Reader) (n int64, err error) {
	m.readFromCalled = true
	return io.Copy(m.ResponseWriter, r)
}

func (m *mockReaderFrom) Unwrap() http.ResponseWriter {
	return m.ResponseWriter
}

type mockPusher struct {
	http.ResponseWriter
	pushTarget  string
	pushCalled  bool
	pushError   error
	pushOptions *http.PushOptions
}

func (m *mockPusher) Push(target string, opts *http.PushOptions) error {
	m.pushCalled = true
	m.pushTarget = target
	m.pushOptions = opts
	return m.pushError
}

func (m *mockPusher) Unwrap() http.ResponseWriter {
	return m.ResponseWriter
}

func TestResponseStatusCoder(t *testing.T) {
	tests := []struct {
		name         string
		w            http.ResponseWriter
		want         StatusCoder
		wantPanic    bool
		panicMessage string
	}{
		{
			name: "status coder implemented",
			w: &mockStatusCoder{
				ResponseWriter: httptest.NewRecorder(),
				statusCode:     http.StatusOK,
			},
			want: func() StatusCoder {
				return &mockStatusCoder{
					ResponseWriter: httptest.NewRecorder(),
					statusCode:     http.StatusOK,
				}
			}(),
		},
		{
			name: "status coder in nested unwrappers",
			w: &mockWriterWithUnwrap{
				ResponseWriter: httptest.NewRecorder(),
				inner: &mockWriterWithUnwrap{
					ResponseWriter: httptest.NewRecorder(),
					inner: &mockStatusCoder{
						ResponseWriter: httptest.NewRecorder(),
						statusCode:     http.StatusCreated,
					},
				},
			},
			want: func() StatusCoder {
				return &mockStatusCoder{
					ResponseWriter: httptest.NewRecorder(),
					statusCode:     http.StatusCreated,
				}
			}(),
		},
		{
			name:      "no status coder implementation",
			w:         httptest.NewRecorder(),
			want:      nil,
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.Panics(t, func() {
					ResponseStatusCoder(tt.w)
				})
			} else {
				got := ResponseStatusCoder(tt.w)
				assert.Equal(t, tt.want != nil, got != nil)
				if tt.want != nil {
					assert.Equal(t, tt.want.StatusCode(), got.StatusCode())
				}
			}
		})
	}
}

func TestResponseSizer(t *testing.T) {
	tests := []struct {
		name      string
		w         http.ResponseWriter
		want      Sizer
		wantPanic bool
	}{
		{
			name: "sizer implemented",
			w: &mockSizer{
				ResponseWriter: httptest.NewRecorder(),
				size:           1024,
			},
			want: func() Sizer {
				return &mockSizer{
					ResponseWriter: httptest.NewRecorder(),
					size:           1024,
				}
			}(),
		},
		{
			name: "sizer in nested unwrappers",
			w: &mockWriterWithUnwrap{
				ResponseWriter: httptest.NewRecorder(),
				inner: &mockSizer{
					ResponseWriter: httptest.NewRecorder(),
					size:           2048,
				},
			},
			want: func() Sizer {
				return &mockSizer{
					ResponseWriter: httptest.NewRecorder(),
					size:           2048,
				}
			}(),
		},
		{
			name:      "no sizer implementation",
			w:         httptest.NewRecorder(),
			want:      nil,
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.Panics(t, func() {
					ResponseSizer(tt.w)
				})
			} else {
				got := ResponseSizer(tt.w)
				assert.Equal(t, tt.want != nil, got != nil)
				if tt.want != nil {
					assert.Equal(t, tt.want.Size(), got.Size())
				}
			}
		})
	}
}

func TestResponseCommitter(t *testing.T) {
	tests := []struct {
		name      string
		w         http.ResponseWriter
		want      Committer
		wantPanic bool
	}{
		{
			name: "committer implemented",
			w: &mockCommitter{
				ResponseWriter: httptest.NewRecorder(),
				committed:      true,
			},
			want: func() Committer {
				return &mockCommitter{
					ResponseWriter: httptest.NewRecorder(),
					committed:      true,
				}
			}(),
		},
		{
			name: "committer in nested unwrappers",
			w: &mockWriterWithUnwrap{
				ResponseWriter: httptest.NewRecorder(),
				inner: &mockWriterWithUnwrap{
					ResponseWriter: httptest.NewRecorder(),
					inner: &mockCommitter{
						ResponseWriter: httptest.NewRecorder(),
						committed:      false,
					},
				},
			},
			want: func() Committer {
				return &mockCommitter{
					ResponseWriter: httptest.NewRecorder(),
					committed:      false,
				}
			}(),
		},
		{
			name:      "no committer implementation",
			w:         httptest.NewRecorder(),
			want:      nil,
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.Panics(t, func() {
					ResponseCommitter(tt.w)
				})
			} else {
				got := ResponseCommitter(tt.w)
				assert.Equal(t, tt.want != nil, got != nil)
				if tt.want != nil {
					assert.Equal(t, tt.want.Committed(), got.Committed())
				}
			}
		})
	}
}

func TestResponseReaderFrom(t *testing.T) {
	tests := []struct {
		name string
		w    http.ResponseWriter
		want io.ReaderFrom
	}{
		{
			name: "reader from implemented",
			w: &mockReaderFrom{
				ResponseWriter: httptest.NewRecorder(),
			},
			want: func() io.ReaderFrom {
				return &mockReaderFrom{
					ResponseWriter: httptest.NewRecorder(),
				}
			}(),
		},
		{
			name: "reader from in nested unwrappers",
			w: &mockWriterWithUnwrap{
				ResponseWriter: httptest.NewRecorder(),
				inner: &mockReaderFrom{
					ResponseWriter: httptest.NewRecorder(),
				},
			},
			want: func() io.ReaderFrom {
				return &mockReaderFrom{
					ResponseWriter: httptest.NewRecorder(),
				}
			}(),
		},
		{
			name: "no reader from implementation",
			w:    httptest.NewRecorder(),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResponseReaderFrom(tt.w)
			assert.Equal(t, tt.want != nil, got != nil)
		})
	}
}

func TestResponseStatus(t *testing.T) {
	tests := []struct {
		name         string
		w            http.ResponseWriter
		wantStatus   int
		wantPanic    bool
		panicMessage string
	}{
		{
			name: "returns status code from status coder",
			w: &mockStatusCoder{
				ResponseWriter: httptest.NewRecorder(),
				statusCode:     http.StatusCreated,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "returns status code from nested wrapper",
			w: &mockWriterWithUnwrap{
				ResponseWriter: httptest.NewRecorder(),
				inner: &mockStatusCoder{
					ResponseWriter: httptest.NewRecorder(),
					statusCode:     http.StatusAccepted,
				},
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "returns status code 0 when no status coder",
			w:          httptest.NewRecorder(),
			wantStatus: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.PanicsWithValue(t, tt.panicMessage, func() {
					ResponseStatusCode(tt.w)
				})
			} else {
				got := ResponseStatusCode(tt.w)
				assert.Equal(t, tt.wantStatus, got)
			}
		})
	}
}

func TestResponseSize(t *testing.T) {
	tests := []struct {
		name         string
		w            http.ResponseWriter
		wantSize     int64
		wantPanic    bool
		panicMessage string
	}{
		{
			name: "returns size from sizer",
			w: &mockSizer{
				ResponseWriter: httptest.NewRecorder(),
				size:           4096,
			},
			wantSize: 4096,
		},
		{
			name: "returns size from nested wrapper",
			w: &mockWriterWithUnwrap{
				ResponseWriter: httptest.NewRecorder(),
				inner: &mockSizer{
					ResponseWriter: httptest.NewRecorder(),
					size:           8192,
				},
			},
			wantSize: 8192,
		},
		{
			name:         "panics when no sizer",
			w:            httptest.NewRecorder(),
			wantPanic:    true,
			panicMessage: "ResponseWriter does not implement Sizer interface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.PanicsWithValue(t, tt.panicMessage, func() {
					ResponseSize(tt.w)
				})
			} else {
				got := ResponseSize(tt.w)
				assert.Equal(t, tt.wantSize, got)
			}
		})
	}
}

func TestResponseCommitted(t *testing.T) {
	tests := []struct {
		name          string
		w             http.ResponseWriter
		wantCommitted bool
		wantPanic     bool
		panicMessage  string
	}{
		{
			name: "returns committed from committer",
			w: &mockCommitter{
				ResponseWriter: httptest.NewRecorder(),
				committed:      true,
			},
			wantCommitted: true,
		},
		{
			name: "returns committed from nested wrapper",
			w: &mockWriterWithUnwrap{
				ResponseWriter: httptest.NewRecorder(),
				inner: &mockCommitter{
					ResponseWriter: httptest.NewRecorder(),
					committed:      false,
				},
			},
			wantCommitted: false,
		},
		{
			name:         "panics when no committer",
			w:            httptest.NewRecorder(),
			wantPanic:    true,
			panicMessage: "ResponseWriter does not implement Committer interface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				assert.PanicsWithValue(t, tt.panicMessage, func() {
					ResponseCommitted(tt.w)
				})
			} else {
				got := ResponseCommitted(tt.w)
				assert.Equal(t, tt.wantCommitted, got)
			}
		})
	}
}

func TestResponse_Reset(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "resets response state",
		},
		{
			name: "resets after writes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &response{}
			rec1 := httptest.NewRecorder()
			rec2 := httptest.NewRecorder()

			r.reset(rec1)
			r.WriteHeader(http.StatusCreated)
			_, _ = r.Write([]byte("test"))
			r.reset(rec2)

			assert.Equal(t, rec2, r.ResponseWriter)
			assert.False(t, r.committed)
			assert.Equal(t, 0, r.code)
			assert.Equal(t, int64(0), r.size)
		})
	}
}

func TestResponse_Size(t *testing.T) {
	tests := []struct {
		name          string
		setupResponse func(*response)
		expectedSize  int64
	}{
		{
			name: "initial size is zero",
			setupResponse: func(r *response) {
				r.reset(httptest.NewRecorder())
			},
			expectedSize: 0,
		},
		{
			name: "size accumulates after writes",
			setupResponse: func(r *response) {
				rec := httptest.NewRecorder()
				r.reset(rec)
				_, _ = r.Write([]byte("hello"))
				_, _ = r.Write([]byte("world"))
			},
			expectedSize: 10,
		},
		{
			name: "size is zero before any write",
			setupResponse: func(r *response) {
				r.reset(httptest.NewRecorder())
				r.WriteHeader(http.StatusOK)
			},
			expectedSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &response{}
			tt.setupResponse(r)
			assert.Equal(t, tt.expectedSize, r.Size())
		})
	}
}

func TestResponse_StatusCode(t *testing.T) {
	tests := []struct {
		name               string
		setupResponse      func(*response)
		expectedStatusCode int
	}{
		{
			name: "initial status code is zero",
			setupResponse: func(r *response) {
				r.reset(httptest.NewRecorder())
			},
			expectedStatusCode: 0,
		},
		{
			name: "status code after WriteHeader",
			setupResponse: func(r *response) {
				rec := httptest.NewRecorder()
				r.reset(rec)
				r.WriteHeader(http.StatusCreated)
			},
			expectedStatusCode: http.StatusCreated,
		},
		{
			name: "implicit status code from first Write",
			setupResponse: func(r *response) {
				rec := httptest.NewRecorder()
				r.reset(rec)
				_, _ = r.Write([]byte("test"))
			},
			expectedStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &response{}
			tt.setupResponse(r)
			assert.Equal(t, tt.expectedStatusCode, r.StatusCode())
		})
	}
}

func TestResponse_Committed(t *testing.T) {
	tests := []struct {
		name          string
		setupResponse func(*response)
		expected      bool
	}{
		{
			name: "not committed initially",
			setupResponse: func(r *response) {
				r.reset(httptest.NewRecorder())
			},
			expected: false,
		},
		{
			name: "committed after WriteHeader",
			setupResponse: func(r *response) {
				rec := httptest.NewRecorder()
				r.reset(rec)
				r.WriteHeader(http.StatusOK)
			},
			expected: true,
		},
		{
			name: "committed after Write",
			setupResponse: func(r *response) {
				rec := httptest.NewRecorder()
				r.reset(rec)
				_, _ = r.Write([]byte("test"))
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &response{}
			tt.setupResponse(r)
			assert.Equal(t, tt.expected, r.Committed())
		})
	}
}

func TestResponse_Unwrap(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "returns underlying ResponseWriter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := &response{}
			r.reset(rec)

			unwrapped := r.Unwrap()
			assert.Equal(t, rec, unwrapped)
		})
	}
}

func TestResponse_WriteHeader(t *testing.T) {
	tests := []struct {
		name              string
		setupResponse     func(*response)
		callWriteHeader   func(*response, int)
		expectedCode      int
		expectedCommitted bool
	}{
		{
			name: "writes header once",
			setupResponse: func(r *response) {
				r.reset(httptest.NewRecorder())
			},
			callWriteHeader: func(r *response, code int) {
				r.WriteHeader(code)
			},
			expectedCode:      http.StatusOK,
			expectedCommitted: true,
		},
		{
			name: "ignores subsequent WriteHeader calls",
			setupResponse: func(r *response) {
				r.reset(httptest.NewRecorder())
			},
			callWriteHeader: func(r *response, code int) {
				r.WriteHeader(http.StatusCreated)
				r.WriteHeader(http.StatusInternalServerError)
			},
			expectedCode:      http.StatusCreated,
			expectedCommitted: true,
		},
		{
			name: "removes Content-Length header",
			setupResponse: func(r *response) {
				r.reset(httptest.NewRecorder())
			},
			callWriteHeader: func(r *response, code int) {
				r.Header().Set(HeaderContentLength, "100")
				r.WriteHeader(code)
			},
			expectedCode:      http.StatusOK,
			expectedCommitted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &response{}
			tt.setupResponse(r)
			tt.callWriteHeader(r, http.StatusOK)

			assert.Equal(t, tt.expectedCode, r.code)
			assert.Equal(t, tt.expectedCommitted, r.committed)
			assert.Empty(t, r.Header().Get(HeaderContentLength))
		})
	}
}

func TestResponse_Write(t *testing.T) {
	tests := []struct {
		name              string
		setupResponse     func(*response, *httptest.ResponseRecorder)
		writeData         []byte
		expectedCode      int
		expectedSize      int64
		expectedBody      string
		expectedCommitted bool
	}{
		{
			name: "write triggers implicit WriteHeader",
			setupResponse: func(r *response, rec *httptest.ResponseRecorder) {
				r.reset(rec)
			},
			writeData:         []byte("hello"),
			expectedCode:      http.StatusOK,
			expectedSize:      5,
			expectedBody:      "hello",
			expectedCommitted: true,
		},
		{
			name: "write after explicit WriteHeader",
			setupResponse: func(r *response, rec *httptest.ResponseRecorder) {
				r.reset(rec)
				r.WriteHeader(http.StatusCreated)
			},
			writeData:         []byte("world"),
			expectedCode:      http.StatusCreated,
			expectedSize:      5,
			expectedBody:      "world",
			expectedCommitted: true,
		},
		{
			name: "multiple writes accumulate size",
			setupResponse: func(r *response, rec *httptest.ResponseRecorder) {
				r.reset(rec)
			},
			writeData:         []byte("hello"),
			expectedCode:      http.StatusOK,
			expectedSize:      10,
			expectedBody:      "hellohello",
			expectedCommitted: true,
		},
		{
			name: "empty write",
			setupResponse: func(r *response, rec *httptest.ResponseRecorder) {
				r.reset(rec)
			},
			writeData:         []byte{},
			expectedCode:      http.StatusOK,
			expectedSize:      0,
			expectedBody:      "",
			expectedCommitted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := &response{}
			tt.setupResponse(r, rec)

			if tt.expectedSize > 0 || tt.expectedBody == "" {
				_, _ = r.Write(tt.writeData)
			}
			if tt.expectedSize == 10 {
				_, _ = r.Write(tt.writeData)
			}

			assert.Equal(t, tt.expectedCode, r.code)
			assert.Equal(t, tt.expectedCommitted, r.committed)
			assert.Equal(t, tt.expectedSize, r.size)
			assert.Equal(t, tt.expectedBody, rec.Body.String())
		})
	}
}

func TestResponse_Flush(t *testing.T) {
	tests := []struct {
		name          string
		setupResponse func(*response) *httptest.ResponseRecorder
		expectPanic   bool
		panicContains string
	}{
		{
			name: "flush with supported writer",
			setupResponse: func(r *response) *httptest.ResponseRecorder {
				rec := httptest.NewRecorder()
				r.reset(rec)
				return rec
			},
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &response{}
			_ = tt.setupResponse(r)

			if tt.expectPanic {
				assert.Panics(t, func() {
					r.Flush()
				})
			} else {
				assert.NotPanics(t, func() {
					r.Flush()
				})
			}
		})
	}
}

func TestResponse_Hijack(t *testing.T) {
	tests := []struct {
		name          string
		setupResponse func(*response) (*response, *httptest.ResponseRecorder)
		expectError   bool
	}{
		{
			name: "hijack not supported with httptest",
			setupResponse: func(r *response) (*response, *httptest.ResponseRecorder) {
				rec := httptest.NewRecorder()
				r.reset(rec)
				return r, rec
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &response{}
			_, rec := tt.setupResponse(r)

			conn, rw, err := r.Hijack()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, conn)
				assert.Nil(t, rw)
				_ = rec
			}
		})
	}
}

func TestResponse_Push(t *testing.T) {
	tests := []struct {
		name          string
		setupResponse func(*response) *response
		target        string
		opts          *http.PushOptions
		expectError   bool
	}{
		{
			name: "push with httptest not supported",
			setupResponse: func(r *response) *response {
				rec := httptest.NewRecorder()
				r.reset(rec)
				return r
			},
			target:      "/style.css",
			opts:        nil,
			expectError: true,
		},
		{
			name: "push with pusher wrapper",
			setupResponse: func(r *response) *response {
				pusher := &mockPusher{
					ResponseWriter: httptest.NewRecorder(),
					pushError:      http.ErrNotSupported,
				}
				r.reset(pusher)
				return r
			},
			target:      "/script.js",
			opts:        &http.PushOptions{Header: http.Header{"X-Test": []string{"value"}}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.setupResponse(&response{})

			err := r.Push(tt.target, tt.opts)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, http.ErrNotSupported, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResponse_ReadFrom(t *testing.T) {
	tests := []struct {
		name          string
		setupResponse func(*response) *response
		reader        io.Reader
		expectedSize  int64
		expectedError bool
	}{
		{
			name: "read from without ReaderFrom support",
			setupResponse: func(r *response) *response {
				rec := httptest.NewRecorder()
				r.reset(rec)
				return r
			},
			reader:        strings.NewReader("test data"),
			expectedSize:  9,
			expectedError: false,
		},
		{
			name: "read from with ReaderFrom support",
			setupResponse: func(r *response) *response {
				rec := httptest.NewRecorder()
				r.reset(rec)
				rf := &mockReaderFrom{
					ResponseWriter: rec,
				}
				r.reset(rf)
				return r
			},
			reader:        strings.NewReader("more data"),
			expectedSize:  9,
			expectedError: false,
		},
		{
			name: "read from triggers implicit WriteHeader",
			setupResponse: func(r *response) *response {
				rec := httptest.NewRecorder()
				r.reset(rec)
				return r
			},
			reader:        strings.NewReader("data"),
			expectedSize:  4,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.setupResponse(&response{})

			n, err := r.ReadFrom(tt.reader)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSize, n)
			}
			assert.True(t, r.committed)
		})
	}
}

func TestResponse_ReadFrom_AfterWriteHeader(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		reader       io.Reader
		expectedSize int64
	}{
		{
			name:         "read from after explicit status code",
			statusCode:   http.StatusCreated,
			reader:       strings.NewReader("content"),
			expectedSize: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := &response{}
			r.reset(rec)
			r.WriteHeader(tt.statusCode)

			n, err := r.ReadFrom(tt.reader)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedSize, n)
			assert.Equal(t, tt.statusCode, r.code)
		})
	}
}

func TestResponse_InterfaceCompliance(t *testing.T) {
	tests := []struct {
		name    string
		checker func(http.ResponseWriter)
	}{
		{
			name: "implements http.Flusher",
			checker: func(w http.ResponseWriter) {
				if _, ok := w.(http.Flusher); !ok {
					t.Error("response should implement http.Flusher")
				}
			},
		},
		{
			name: "implements http.Hijacker",
			checker: func(w http.ResponseWriter) {
				if _, ok := w.(http.Hijacker); !ok {
					t.Error("response should implement http.Hijacker")
				}
			},
		},
		{
			name: "implements http.Pusher",
			checker: func(w http.ResponseWriter) {
				if _, ok := w.(http.Pusher); !ok {
					t.Error("response should implement http.Pusher")
				}
			},
		},
		{
			name: "implements io.ReaderFrom",
			checker: func(w http.ResponseWriter) {
				if _, ok := w.(io.ReaderFrom); !ok {
					t.Error("response should implement io.ReaderFrom")
				}
			},
		},
		{
			name: "implements RWUnwrapper",
			checker: func(w http.ResponseWriter) {
				if _, ok := w.(RWUnwrapper); !ok {
					t.Error("response should implement RWUnwrapper")
				}
			},
		},
		{
			name: "implements Committer",
			checker: func(w http.ResponseWriter) {
				if _, ok := w.(Committer); !ok {
					t.Error("response should implement Committer")
				}
			},
		},
		{
			name: "implements StatusCoder",
			checker: func(w http.ResponseWriter) {
				if _, ok := w.(StatusCoder); !ok {
					t.Error("response should implement StatusCoder")
				}
			},
		},
		{
			name: "implements Sizer",
			checker: func(w http.ResponseWriter) {
				if _, ok := w.(Sizer); !ok {
					t.Error("response should implement Sizer")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &response{}
			r.reset(httptest.NewRecorder())
			tt.checker(r)
		})
	}
}

func TestResponse_Integration(t *testing.T) {
	tests := []struct {
		name     string
		handler  func(http.ResponseWriter)
		wantCode int
		wantBody string
		wantSize int64
	}{
		{
			name: "standard response flow",
			handler: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("hello"))
			},
			wantCode: http.StatusOK,
			wantBody: "hello",
			wantSize: 5,
		},
		{
			name: "implicit status code",
			handler: func(w http.ResponseWriter) {
				_, _ = w.Write([]byte("automatic 200"))
			},
			wantCode: http.StatusOK,
			wantBody: "automatic 200",
			wantSize: 13,
		},
		{
			name: "multiple writes",
			handler: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("part1 "))
				_, _ = w.Write([]byte("part2"))
			},
			wantCode: http.StatusCreated,
			wantBody: "part1 part2",
			wantSize: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := &response{}
			r.reset(rec)

			tt.handler(r)

			assert.Equal(t, tt.wantCode, r.StatusCode())
			assert.Equal(t, tt.wantSize, r.Size())
			assert.True(t, r.Committed())
			assert.Equal(t, tt.wantBody, rec.Body.String())
		})
	}
}

func TestResponse_WriteHeader_NoOpAfterCommit(t *testing.T) {
	t.Run("WriteHeader is no-op after committed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r := &response{}
		r.reset(rec)

		r.WriteHeader(http.StatusCreated)
		assert.True(t, r.committed)
		assert.Equal(t, http.StatusCreated, r.code)
		assert.Equal(t, http.StatusCreated, rec.Code)

		r.WriteHeader(http.StatusInternalServerError)
		assert.Equal(t, http.StatusCreated, r.code)
		assert.Equal(t, http.StatusCreated, rec.Code)
	})
}

func TestResponse_Write_WithNilWriter(t *testing.T) {
	t.Run("write with nil underlying writer", func(t *testing.T) {
		r := &response{}
		assert.Panics(t, func() {
			_, _ = r.Write([]byte("test"))
		})
	})
}
