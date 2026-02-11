package keratin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerFunc_ServeHTTP(t *testing.T) {
	tests := []struct {
		name      string
		handler   HandlerFunc
		wantErr   error
		assertion func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful handler returns nil",
			handler: func(w http.ResponseWriter, r *http.Request) error {
				return nil
			},
			wantErr: nil,
			assertion: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
		{
			name: "handler writes response and returns nil",
			handler: func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("created"))
				return nil
			},
			wantErr: nil,
			assertion: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusCreated, w.Code)
				assert.Equal(t, "created", w.Body.String())
			},
		},
		{
			name: "handler returns error",
			handler: func(w http.ResponseWriter, r *http.Request) error {
				return ErrNotFound
			},
			wantErr: ErrNotFound,
			assertion: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Empty(t, w.Body.String())
			},
		},
		{
			name: "handler returns HTTPError",
			handler: func(w http.ResponseWriter, r *http.Request) error {
				return NewHTTPError(http.StatusBadRequest, "bad request")
			},
			wantErr: NewHTTPError(http.StatusBadRequest, "bad request"),
			assertion: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
		{
			name: "handler with different HTTP methods",
			handler: func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(r.Method))
				return nil
			},
			wantErr: nil,
			assertion: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			err := tt.handler.ServeHTTP(w, r)

			if tt.wantErr != nil {
				assert.Error(t, err)
				var (
					expectedHTTPErr *HTTPError
					actualHTTPErr   *HTTPError
				)
				if errors.As(tt.wantErr, &expectedHTTPErr) && errors.As(err, &actualHTTPErr) {
					assert.Equal(t, expectedHTTPErr.Code, actualHTTPErr.Code)
					assert.Equal(t, expectedHTTPErr.Message, actualHTTPErr.Message)
				} else {
					assert.ErrorIs(t, err, tt.wantErr)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.assertion != nil {
				tt.assertion(t, w)
			}
		})
	}
}

func TestHandlerFunc_HandlerInterface(t *testing.T) {
	tests := []struct {
		name    string
		handler HandlerFunc
	}{
		{
			name:    "HandlerFunc implements Handler interface",
			handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error { return nil }),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h Handler = tt.handler
			assert.NotNil(t, h)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			err := h.ServeHTTP(w, r)
			assert.NoError(t, err)
		})
	}
}

func TestErrorHandler_CommittedResponse(t *testing.T) {
	tests := []struct {
		name         string
		setupRequest func(*http.Request) *http.Request
		setupWriter  func() *response
		expectedCode int
		expectedBody string
	}{
		{
			name: "does not write if response already committed",
			setupRequest: func(r *http.Request) *http.Request {
				return r
			},
			setupWriter: func() *response {
				w := &response{}
				w.reset(httptest.NewRecorder())
				w.committed = true
				w.code = http.StatusOK
				return w
			},
			expectedCode: http.StatusOK,
			expectedBody: "",
		},
		{
			name: "does not write if response already committed with error code",
			setupRequest: func(r *http.Request) *http.Request {
				return r
			},
			setupWriter: func() *response {
				w := &response{}
				w.reset(httptest.NewRecorder())
				w.committed = true
				w.code = http.StatusNotFound
				return w
			},
			expectedCode: http.StatusNotFound,
			expectedBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := tt.setupWriter()
			r := tt.setupRequest(httptest.NewRequest(http.MethodGet, "/", nil))

			ErrorHandler(w, r, ErrBadRequest)

			assert.Equal(t, tt.expectedCode, w.code)
			assert.Equal(t, tt.expectedBody, w.ResponseWriter.(*httptest.ResponseRecorder).Body.String())
		})
	}
}

func TestErrorHandler_HTTPErrorResponse(t *testing.T) {
	tests := []struct {
		name           string
		acceptHeader   string
		err            error
		expectedStatus int
		expectedJSON   bool
		expectedBody   string
	}{
		{
			name:           "returns HTTP status for HTTPError",
			acceptHeader:   MIMEApplicationJSON,
			err:            NewHTTPError(http.StatusBadRequest, "bad request"),
			expectedStatus: http.StatusBadRequest,
			expectedJSON:   true,
			expectedBody:   "{\"message\":\"bad request\"}\n",
		},
		{
			name:           "returns default HTTPError for non-HTTPError",
			acceptHeader:   MIMEApplicationJSON,
			err:            errors.New("some error"),
			expectedStatus: http.StatusInternalServerError,
			expectedJSON:   true,
			expectedBody:   "{\"message\":\"Internal Server Error\"}\n",
		},
		{
			name:           "returns JSON when Accept is application/json",
			acceptHeader:   MIMEApplicationJSON,
			err:            ErrNotFound,
			expectedStatus: http.StatusNotFound,
			expectedJSON:   true,
			expectedBody:   "{\"message\":\"Not Found\"}\n",
		},
		{
			name:           "returns JSON when Accept contains application/json",
			acceptHeader:   "text/html, application/json, */*",
			err:            ErrUnauthorized,
			expectedStatus: http.StatusUnauthorized,
			expectedJSON:   true,
			expectedBody:   "{\"message\":\"Unauthorized\"}\n",
		},
		{
			name:           "returns plain text when Accept is text/html",
			acceptHeader:   MIMETextHTML,
			err:            ErrNotFound,
			expectedStatus: http.StatusNotFound,
			expectedJSON:   false,
			expectedBody:   "Not Found\n",
		},
		{
			name:           "returns plain text when Accept is not application/json",
			acceptHeader:   "text/plain",
			err:            ErrForbidden,
			expectedStatus: http.StatusForbidden,
			expectedJSON:   false,
			expectedBody:   "Forbidden\n",
		},
		{
			name:           "returns plain text when Accept header is missing",
			acceptHeader:   "",
			err:            ErrBadRequest,
			expectedStatus: http.StatusBadRequest,
			expectedJSON:   false,
			expectedBody:   "Bad Request\n",
		},
		{
			name:           "returns JSON with custom message from HTTPError",
			acceptHeader:   MIMEApplicationJSON,
			err:            NewHTTPError(http.StatusTeapot, "I'm a teapot"),
			expectedStatus: http.StatusTeapot,
			expectedJSON:   true,
			expectedBody:   "{\"message\":\"I'm a teapot\"}\n",
		},
		{
			name:           "returns JSON with custom error message",
			acceptHeader:   MIMEApplicationJSON,
			err:            NewHTTPError(http.StatusConflict, "resource already exists"),
			expectedStatus: http.StatusConflict,
			expectedJSON:   true,
			expectedBody:   "{\"message\":\"resource already exists\"}\n",
		},
		{
			name:           "handles wrapped HTTPError",
			acceptHeader:   MIMEApplicationJSON,
			err:            ErrNotFound.Wrap(errors.New("details")),
			expectedStatus: http.StatusNotFound,
			expectedJSON:   true,
			expectedBody:   "{\"message\":\"Not Found\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			if tt.acceptHeader != "" {
				r.Header.Set(HeaderAccept, tt.acceptHeader)
			}

			wrapped := &response{}
			wrapped.reset(w)
			ErrorHandler(wrapped, r, tt.err)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())

			if tt.expectedJSON {
				assert.Equal(t, MIMEApplicationJSON, w.Header().Get(HeaderContentType))
			} else {
				assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get(HeaderContentType))
			}
		})
	}
}

func TestErrorHandler_VariousErrorTypes(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "ErrBadRequest",
			err:            ErrBadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "ErrUnauthorized",
			err:            ErrUnauthorized,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "ErrPaymentRequired",
			err:            ErrPaymentRequired,
			expectedStatus: http.StatusPaymentRequired,
		},
		{
			name:           "ErrForbidden",
			err:            ErrForbidden,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "ErrNotFound",
			err:            ErrNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "ErrMethodNotAllowed",
			err:            ErrMethodNotAllowed,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "ErrConflict",
			err:            ErrConflict,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "ErrInternalServerError",
			err:            ErrInternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "ErrServiceUnavailable",
			err:            ErrServiceUnavailable,
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "ErrGatewayTimeout",
			err:            ErrGatewayTimeout,
			expectedStatus: http.StatusGatewayTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			wrapped := &response{}
			wrapped.reset(w)
			ErrorHandler(wrapped, r, tt.err)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestErrorHandler_JSONResponse(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		err          error
		expectedJSON string
	}{
		{
			name:         "simple HTTPError",
			acceptHeader: MIMEApplicationJSON,
			err:          NewHTTPError(http.StatusBadRequest, "invalid input"),
			expectedJSON: `{"message":"invalid input"}`,
		},
		{
			name:         "empty message HTTPError",
			acceptHeader: MIMEApplicationJSON,
			err:          NewHTTPError(http.StatusNotFound, ""),
			expectedJSON: `{"message":""}`,
		},
		{
			name:         "HTTPError with special characters",
			acceptHeader: MIMEApplicationJSON,
			err:          NewHTTPError(http.StatusBadRequest, `"quoted" & <angled>`),
			expectedJSON: `{"message":"\"quoted\" & <angled>"}`,
		},
		{
			name:         "HTTPError with unicode",
			acceptHeader: MIMEApplicationJSON,
			err:          NewHTTPError(http.StatusBadRequest, "错误信息"),
			expectedJSON: `{"message":"错误信息"}`,
		},
		{
			name:         "ErrNotFound wrapped with error",
			acceptHeader: MIMEApplicationJSON,
			err:          ErrNotFound.Wrap(errors.New("resource id not found")),
			expectedJSON: `{"message":"Not Found"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set(HeaderAccept, tt.acceptHeader)

			wrapped := &response{}
			wrapped.reset(w)
			ErrorHandler(wrapped, r, tt.err)

			assert.Equal(t, MIMEApplicationJSON, w.Header().Get(HeaderContentType))
			assert.JSONEq(t, tt.expectedJSON, w.Body.String())
		})
	}
}

func TestErrorHandler_PlainTextResponse(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		err          error
		expectedBody string
	}{
		{
			name:         "simple HTTPError",
			acceptHeader: MIMETextHTML,
			err:          NewHTTPError(http.StatusBadRequest, "invalid input"),
			expectedBody: "invalid input\n",
		},
		{
			name:         "empty message HTTPError uses status text",
			acceptHeader: MIMETextHTML,
			err:          NewHTTPError(http.StatusNotFound, ""),
			expectedBody: "\n",
		},
		{
			name:         "predefined error",
			acceptHeader: "",
			err:          ErrNotFound,
			expectedBody: "Not Found\n",
		},
		{
			name:         "predefined error with custom wrap",
			acceptHeader: MIMETextPlain,
			err:          ErrUnauthorized.Wrap(errors.New("user not authenticated")),
			expectedBody: "Unauthorized\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptHeader != "" {
				r.Header.Set(HeaderAccept, tt.acceptHeader)
			}

			wrapped := &response{}
			wrapped.reset(w)
			ErrorHandler(wrapped, r, tt.err)

			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestErrorHandler_CaseInsensitiveAccept(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		expectJSON   bool
	}{
		{
			name:         "lowercase application/json",
			acceptHeader: "application/json",
			expectJSON:   true,
		},
		{
			name:         "uppercase APPLICATION/JSON",
			acceptHeader: "APPLICATION/JSON",
			expectJSON:   false,
		},
		{
			name:         "mixed case Application/Json",
			acceptHeader: "Application/Json",
			expectJSON:   false,
		},
		{
			name:         "application/json with charset",
			acceptHeader: "application/json; charset=utf-8",
			expectJSON:   true,
		},
		{
			name:         "json in multiple accept values",
			acceptHeader: "text/html, application/json",
			expectJSON:   true,
		},
		{
			name:         "json with quality value",
			acceptHeader: "application/json; q=0.9",
			expectJSON:   true,
		},
		{
			name:         "text/html",
			acceptHeader: "text/html",
			expectJSON:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set(HeaderAccept, tt.acceptHeader)

			wrapped := &response{}
			wrapped.reset(w)
			ErrorHandler(wrapped, r, ErrBadRequest)

			if tt.expectJSON {
				assert.Equal(t, MIMEApplicationJSON, w.Header().Get(HeaderContentType))
				assert.JSONEq(t, `{"message":"Bad Request"}`, w.Body.String())
			} else {
				assert.Equal(t, "Bad Request\n", w.Body.String())
			}
		})
	}
}

func TestErrorHandler_NilErrorPanic(t *testing.T) {
	t.Run("nil error causes panic in HTTPErrorStatusCode", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		wrapped := &response{}
		wrapped.reset(w)
		assert.Panics(t, func() {
			ErrorHandler(wrapped, r, nil)
		})
	})
}

func TestErrorHandler_AcceptHeaderValues(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		err          error
		checkJSON    func(*testing.T, string)
		checkText    func(*testing.T, string)
	}{
		{
			name:         "wildcard accept",
			acceptHeader: "*/*",
			err:          ErrNotFound,
			checkText: func(t *testing.T, body string) {
				assert.Equal(t, "Not Found\n", body)
			},
		},
		{
			name:         "multiple accepts with json last",
			acceptHeader: "text/html, text/plain, application/json",
			err:          ErrBadRequest,
			checkJSON: func(t *testing.T, body string) {
				assert.JSONEq(t, `{"message":"Bad Request"}`, body)
			},
		},
		{
			name:         "json with wildcards",
			acceptHeader: "application/*",
			err:          ErrForbidden,
			checkText: func(t *testing.T, body string) {
				assert.Equal(t, "Forbidden\n", body)
			},
		},
		{
			name:         "empty accept header",
			acceptHeader: "",
			err:          ErrUnauthorized,
			checkText: func(t *testing.T, body string) {
				assert.Equal(t, "Unauthorized\n", body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptHeader != "" {
				r.Header.Set(HeaderAccept, tt.acceptHeader)
			}

			wrapped := &response{}
			wrapped.reset(w)
			ErrorHandler(wrapped, r, tt.err)

			body := w.Body.String()
			if tt.checkJSON != nil {
				tt.checkJSON(t, body)
			}
			if tt.checkText != nil {
				tt.checkText(t, body)
			}
		})
	}
}

func TestErrorHandler_StatusCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{"400", ErrBadRequest, http.StatusBadRequest},
		{"401", ErrUnauthorized, http.StatusUnauthorized},
		{"403", ErrForbidden, http.StatusForbidden},
		{"404", ErrNotFound, http.StatusNotFound},
		{"405", ErrMethodNotAllowed, http.StatusMethodNotAllowed},
		{"409", ErrConflict, http.StatusConflict},
		{"429", ErrTooManyRequests, http.StatusTooManyRequests},
		{"500", ErrInternalServerError, http.StatusInternalServerError},
		{"503", ErrServiceUnavailable, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set(HeaderAccept, MIMEApplicationJSON)

			wrapped := &response{}
			wrapped.reset(w)
			ErrorHandler(wrapped, r, tt.err)

			assert.Equal(t, tt.code, w.Code)
			assert.Equal(t, MIMEApplicationJSON, w.Header().Get(HeaderContentType))
			body := w.Body.String()
			assert.Contains(t, body, `"message"`)
		})
	}
}

func TestErrorHandler_ErrorMessagePriority(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedMsg string
	}{
		{
			name:        "HTTPError with custom message",
			err:         NewHTTPError(http.StatusBadRequest, "custom message"),
			expectedMsg: "custom message",
		},
		{
			name:        "HTTPError with empty message uses status text",
			err:         NewHTTPError(http.StatusNotFound, ""),
			expectedMsg: "",
		},
		{
			name:        "predefined error uses status text",
			err:         ErrBadRequest,
			expectedMsg: "Bad Request",
		},
		{
			name:        "wrapped error preserves original message",
			err:         ErrNotFound.Wrap(errors.New("wrapped")),
			expectedMsg: "Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set(HeaderAccept, MIMEApplicationJSON)

			wrapped := &response{}
			wrapped.reset(w)
			ErrorHandler(wrapped, r, tt.err)

			assert.JSONEq(t, `{"message":"`+tt.expectedMsg+`"}`, w.Body.String())
		})
	}
}

func TestHandler_ErrorHandlingFlow(t *testing.T) {
	t.Run("complete error handling flow", func(t *testing.T) {
		handlerCalled := false
		handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			handlerCalled = true
			return ErrNotFound
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set(HeaderAccept, MIMEApplicationJSON)

		err := handler.ServeHTTP(w, r)

		assert.True(t, handlerCalled, "handler should have been called")
		assert.Error(t, err, "handler should return error")
		assert.ErrorIs(t, err, ErrNotFound, "error should be ErrNotFound")

		wrapped := &response{}
		wrapped.reset(w)
		ErrorHandler(wrapped, r, err)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.JSONEq(t, `{"message":"Not Found"}`, w.Body.String())
	})

	t.Run("successful flow with no error", func(t *testing.T) {
		handlerCalled := false
		handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
			return nil
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/test", nil)

		err := handler.ServeHTTP(w, r)

		assert.True(t, handlerCalled, "handler should have been called")
		assert.NoError(t, err, "handler should not return error")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})
}

func TestHandlerFunc_ErrorPropagation(t *testing.T) {
	tests := []struct {
		name        string
		handler     HandlerFunc
		expectError error
	}{
		{
			name: "propagates predefined error",
			handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				return ErrNotFound
			}),
			expectError: ErrNotFound,
		},
		{
			name: "propagates HTTPError",
			handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				return NewHTTPError(http.StatusTeapot, "tea time")
			}),
			expectError: &HTTPError{Code: http.StatusTeapot, Message: "tea time"},
		},
		{
			name: "propagates nil",
			handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				return nil
			}),
			expectError: nil,
		},
		{
			name: "propagates wrapped error",
			handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				return ErrBadRequest.Wrap(errors.New("invalid input"))
			}),
			expectError: ErrBadRequest.Wrap(errors.New("invalid input")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			err := tt.handler.ServeHTTP(w, r)

			if tt.expectError != nil {
				require.Error(t, err)
				var (
					expectedHTTPErr *HTTPError
					actualHTTPErr   *HTTPError
				)
				if errors.As(tt.expectError, &expectedHTTPErr) && errors.As(err, &actualHTTPErr) {
					assert.Equal(t, expectedHTTPErr.Code, actualHTTPErr.Code)
					assert.Equal(t, expectedHTTPErr.Message, actualHTTPErr.Message)
				} else {
					assert.ErrorIs(t, err, tt.expectError)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestErrorHandler_HTTPStatusCodes(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode int
	}{
		{
			name:         "HTTPError with 400",
			err:          NewHTTPError(http.StatusBadRequest, ""),
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "HTTPError with 404",
			err:          NewHTTPError(http.StatusNotFound, ""),
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "HTTPError with 500",
			err:          NewHTTPError(http.StatusInternalServerError, ""),
			expectedCode: http.StatusInternalServerError,
		},
		{
			name:         "HTTPError with 418",
			err:          NewHTTPError(http.StatusTeapot, ""),
			expectedCode: http.StatusTeapot,
		},
		{
			name:         "standard error defaults to 500",
			err:          errors.New("generic error"),
			expectedCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			wrapped := &response{}
			wrapped.reset(w)
			ErrorHandler(wrapped, r, tt.err)

			assert.Equal(t, tt.expectedCode, w.Code)
		})
	}
}
