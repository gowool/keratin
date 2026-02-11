package keratin

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPError_NewHTTPError(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		message string
	}{
		{
			name:    "creates error with custom message",
			code:    http.StatusBadRequest,
			message: "invalid request",
		},
		{
			name:    "creates error with empty message",
			code:    http.StatusInternalServerError,
			message: "",
		},
		{
			name:    "creates error with standard status",
			code:    http.StatusNotFound,
			message: "resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewHTTPError(tt.code, tt.message)

			assert.Equal(t, tt.code, err.Code)
			assert.Equal(t, tt.message, err.Message)
			assert.Nil(t, err.err)
		})
	}
}

func TestHTTPError_StatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      *HTTPError
		expected int
	}{
		{
			name:     "returns 400",
			err:      NewHTTPError(http.StatusBadRequest, "bad request"),
			expected: http.StatusBadRequest,
		},
		{
			name:     "returns 404",
			err:      NewHTTPError(http.StatusNotFound, "not found"),
			expected: http.StatusNotFound,
		},
		{
			name:     "returns 500",
			err:      NewHTTPError(http.StatusInternalServerError, "internal error"),
			expected: http.StatusInternalServerError,
		},
		{
			name:     "returns custom status code",
			err:      NewHTTPError(418, "teapot"),
			expected: 418,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.StatusCode()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestHTTPError_Error(t *testing.T) {
	tests := []struct {
		name          string
		err           *HTTPError
		expectedError string
	}{
		{
			name:          "error with message only",
			err:           NewHTTPError(http.StatusBadRequest, "invalid input"),
			expectedError: "code=400, message=invalid input",
		},
		{
			name:          "error with empty message uses status text",
			err:           NewHTTPError(http.StatusNotFound, ""),
			expectedError: "code=404, message=Not Found",
		},
		{
			name: "error with wrapped error",
			err: &HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "server error",
				err:     errors.New("database connection failed"),
			},
			expectedError: "code=500, message=server error, err=database connection failed",
		},
		{
			name:          "error with empty message and wrapped error",
			err:           &HTTPError{Code: http.StatusConflict, err: errors.New("duplicate key")},
			expectedError: "code=409, message=Conflict, err=duplicate key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			assert.Equal(t, tt.expectedError, got)
		})
	}
}

func TestHTTPError_Wrap(t *testing.T) {
	tests := []struct {
		name       string
		baseErr    *HTTPError
		wrapErr    error
		wantCode   int
		wantMsg    string
		wantUnwrap error
	}{
		{
			name: "wraps error with base error code and message",
			baseErr: &HTTPError{
				Code:    http.StatusBadRequest,
				Message: "validation failed",
			},
			wrapErr:    errors.New("invalid email"),
			wantCode:   http.StatusBadRequest,
			wantMsg:    "validation failed",
			wantUnwrap: errors.New("invalid email"),
		},
		{
			name: "wraps nil error keeps base code and message",
			baseErr: &HTTPError{
				Code:    http.StatusNotFound,
				Message: "not found",
			},
			wrapErr:    nil,
			wantCode:   http.StatusNotFound,
			wantMsg:    "not found",
			wantUnwrap: nil,
		},
		{
			name: "wraps error with empty message",
			baseErr: &HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "",
			},
			wrapErr:    errors.New("system error"),
			wantCode:   http.StatusInternalServerError,
			wantMsg:    "",
			wantUnwrap: errors.New("system error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.baseErr.Wrap(tt.wrapErr)

			assert.IsType(t, &HTTPError{}, result)
			httpErr, ok := result.(*HTTPError)
			require.True(t, ok)

			assert.Equal(t, tt.wantCode, httpErr.Code)
			assert.Equal(t, tt.wantMsg, httpErr.Message)

			if tt.wantUnwrap != nil {
				require.NotNil(t, httpErr.err)
				assert.Equal(t, tt.wantUnwrap.Error(), httpErr.err.Error())
			} else {
				assert.Nil(t, httpErr.err)
			}
		})
	}
}

func TestHTTPError_Unwrap(t *testing.T) {
	tests := []struct {
		name       string
		err        *HTTPError
		wantUnwrap error
	}{
		{
			name:       "nil wrapped error returns nil",
			err:        NewHTTPError(http.StatusBadRequest, "bad request"),
			wantUnwrap: nil,
		},
		{
			name: "returns wrapped error",
			err: &HTTPError{
				Code:    http.StatusInternalServerError,
				Message: "server error",
				err:     errors.New("database failed"),
			},
			wantUnwrap: errors.New("database failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Unwrap()
			if tt.wantUnwrap == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantUnwrap.Error(), got.Error())
			}
		})
	}
}

func TestHTTPError_ImplementsError(t *testing.T) {
	err := NewHTTPError(http.StatusBadRequest, "bad request")
	assert.Implements(t, (*error)(nil), err)
}

func TestHTTPError_ImplementsStatusCoder(t *testing.T) {
	var _ StatusCoder = (*HTTPError)(nil)
	err := NewHTTPError(http.StatusBadRequest, "bad request")
	got := err.StatusCode()
	assert.Equal(t, http.StatusBadRequest, got)
}

func TestHTTPError_Equal(t *testing.T) {
	err1 := NewHTTPError(http.StatusBadRequest, "bad request")
	err2 := NewHTTPError(http.StatusBadRequest, "bad request")
	err3 := NewHTTPError(http.StatusNotFound, "not found")

	assert.Equal(t, err1, err2)
	assert.NotEqual(t, err1, err3)
}

func TestHttpError_StatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      httpError
		expected int
	}{
		{
			name:     "ErrBadRequest returns 400",
			err:      *ErrBadRequest,
			expected: http.StatusBadRequest,
		},
		{
			name:     "ErrNotFound returns 404",
			err:      *ErrNotFound,
			expected: http.StatusNotFound,
		},
		{
			name:     "ErrInternalServerError returns 500",
			err:      *ErrInternalServerError,
			expected: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.StatusCode()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestHttpError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      httpError
		expected string
	}{
		{
			name:     "ErrBadRequest error message",
			err:      *ErrBadRequest,
			expected: "Bad Request",
		},
		{
			name:     "ErrNotFound error message",
			err:      *ErrNotFound,
			expected: "Not Found",
		},
		{
			name:     "ErrTeapot error message",
			err:      *ErrTeapot,
			expected: "I'm a teapot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestHttpError_Wrap(t *testing.T) {
	tests := []struct {
		name     string
		baseErr  httpError
		wrapErr  error
		wantCode int
		wantMsg  string
	}{
		{
			name:     "wraps ErrBadRequest with custom error",
			baseErr:  *ErrBadRequest,
			wrapErr:  errors.New("validation failed"),
			wantCode: http.StatusBadRequest,
			wantMsg:  "Bad Request",
		},
		{
			name:     "wraps ErrNotFound with custom error",
			baseErr:  *ErrNotFound,
			wrapErr:  errors.New("user not found"),
			wantCode: http.StatusNotFound,
			wantMsg:  "Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.baseErr.Wrap(tt.wrapErr)

			assert.IsType(t, &HTTPError{}, result)
			httpErr, ok := result.(*HTTPError)
			require.True(t, ok)

			assert.Equal(t, tt.wantCode, httpErr.Code)
			assert.Equal(t, tt.wantMsg, httpErr.Message)
			assert.Equal(t, tt.wrapErr.Error(), httpErr.err.Error())
		})
	}
}

func TestHttpError_ImplementsError(t *testing.T) {
	assert.Implements(t, (*error)(nil), ErrBadRequest)
}

func TestHttpError_ImplementsStatusCoder(t *testing.T) {
	var _ StatusCoder = (*httpError)(nil)
	got := ErrBadRequest.StatusCode()
	assert.Equal(t, http.StatusBadRequest, got)
}

func TestPredefinedErrors_StatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"ErrBadRequest", ErrBadRequest, http.StatusBadRequest},
		{"ErrUnauthorized", ErrUnauthorized, http.StatusUnauthorized},
		{"ErrPaymentRequired", ErrPaymentRequired, http.StatusPaymentRequired},
		{"ErrForbidden", ErrForbidden, http.StatusForbidden},
		{"ErrNotFound", ErrNotFound, http.StatusNotFound},
		{"ErrMethodNotAllowed", ErrMethodNotAllowed, http.StatusMethodNotAllowed},
		{"ErrNotAcceptable", ErrNotAcceptable, http.StatusNotAcceptable},
		{"ErrProxyAuthRequired", ErrProxyAuthRequired, http.StatusProxyAuthRequired},
		{"ErrRequestTimeout", ErrRequestTimeout, http.StatusRequestTimeout},
		{"ErrConflict", ErrConflict, http.StatusConflict},
		{"ErrGone", ErrGone, http.StatusGone},
		{"ErrLengthRequired", ErrLengthRequired, http.StatusLengthRequired},
		{"ErrPreconditionFailed", ErrPreconditionFailed, http.StatusPreconditionFailed},
		{"ErrRequestEntityTooLarge", ErrRequestEntityTooLarge, http.StatusRequestEntityTooLarge},
		{"ErrRequestURITooLong", ErrRequestURITooLong, http.StatusRequestURITooLong},
		{"ErrUnsupportedMediaType", ErrUnsupportedMediaType, http.StatusUnsupportedMediaType},
		{"ErrRequestedRangeNotSatisfiable", ErrRequestedRangeNotSatisfiable, http.StatusRequestedRangeNotSatisfiable},
		{"ErrExpectationFailed", ErrExpectationFailed, http.StatusExpectationFailed},
		{"ErrTeapot", ErrTeapot, http.StatusTeapot},
		{"ErrMisdirectedRequest", ErrMisdirectedRequest, http.StatusMisdirectedRequest},
		{"ErrUnprocessableEntity", ErrUnprocessableEntity, http.StatusUnprocessableEntity},
		{"ErrLocked", ErrLocked, http.StatusLocked},
		{"ErrFailedDependency", ErrFailedDependency, http.StatusFailedDependency},
		{"ErrTooEarly", ErrTooEarly, http.StatusTooEarly},
		{"ErrUpgradeRequired", ErrUpgradeRequired, http.StatusUpgradeRequired},
		{"ErrPreconditionRequired", ErrPreconditionRequired, http.StatusPreconditionRequired},
		{"ErrTooManyRequests", ErrTooManyRequests, http.StatusTooManyRequests},
		{"ErrRequestHeaderFieldsTooLarge", ErrRequestHeaderFieldsTooLarge, http.StatusRequestHeaderFieldsTooLarge},
		{"ErrUnavailableForLegalReasons", ErrUnavailableForLegalReasons, http.StatusUnavailableForLegalReasons},
		{"ErrInternalServerError", ErrInternalServerError, http.StatusInternalServerError},
		{"ErrNotImplemented", ErrNotImplemented, http.StatusNotImplemented},
		{"ErrBadGateway", ErrBadGateway, http.StatusBadGateway},
		{"ErrServiceUnavailable", ErrServiceUnavailable, http.StatusServiceUnavailable},
		{"ErrGatewayTimeout", ErrGatewayTimeout, http.StatusGatewayTimeout},
		{"ErrHTTPVersionNotSupported", ErrHTTPVersionNotSupported, http.StatusHTTPVersionNotSupported},
		{"ErrVariantAlsoNegotiates", ErrVariantAlsoNegotiates, http.StatusVariantAlsoNegotiates},
		{"ErrInsufficientStorage", ErrInsufficientStorage, http.StatusInsufficientStorage},
		{"ErrLoopDetected", ErrLoopDetected, http.StatusLoopDetected},
		{"ErrNotExtended", ErrNotExtended, http.StatusNotExtended},
		{"ErrNetworkAuthenticationRequired", ErrNetworkAuthenticationRequired, http.StatusNetworkAuthenticationRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, ok := tt.err.(interface{ StatusCode() int })
			require.True(t, ok, "error should implement StatusCode method")
			assert.Equal(t, tt.expected, sc.StatusCode())
		})
	}
}

func TestPredefinedErrors_ErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrBadRequest", ErrBadRequest, "Bad Request"},
		{"ErrNotFound", ErrNotFound, "Not Found"},
		{"ErrInternalServerError", ErrInternalServerError, "Internal Server Error"},
		{"ErrTeapot", ErrTeapot, "I'm a teapot"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestPredefinedErrors_Wrap(t *testing.T) {
	tests := []struct {
		name     string
		baseErr  error
		wrapErr  error
		wantCode int
		wantMsg  string
	}{
		{
			name:     "ErrBadRequest wraps custom error",
			baseErr:  ErrBadRequest,
			wrapErr:  errors.New("validation failed"),
			wantCode: http.StatusBadRequest,
			wantMsg:  "Bad Request",
		},
		{
			name:     "ErrNotFound wraps custom error",
			baseErr:  ErrNotFound,
			wrapErr:  errors.New("user not found"),
			wantCode: http.StatusNotFound,
			wantMsg:  "Not Found",
		},
		{
			name:     "ErrInternalServerError wraps custom error",
			baseErr:  ErrInternalServerError,
			wrapErr:  errors.New("database failed"),
			wantCode: http.StatusInternalServerError,
			wantMsg:  "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper, ok := tt.baseErr.(interface {
				Wrap(error) error
			})
			require.True(t, ok, "error should have Wrap method")

			result := wrapper.Wrap(tt.wrapErr)
			assert.IsType(t, &HTTPError{}, result)

			httpErr, ok := result.(*HTTPError)
			require.True(t, ok)

			assert.Equal(t, tt.wantCode, httpErr.Code)
			assert.Equal(t, tt.wantMsg, httpErr.Message)
			assert.Equal(t, tt.wrapErr.Error(), httpErr.err.Error())
		})
	}
}

func TestHTTPError_Integration(t *testing.T) {
	t.Run("create, wrap, and unwrap chain", func(t *testing.T) {
		baseErr := errors.New("database connection failed")
		wrapped1 := NewHTTPError(http.StatusInternalServerError, "server error").Wrap(baseErr)
		wrapped2 := NewHTTPError(http.StatusInternalServerError, "upstream error").Wrap(wrapped1)

		assert.Equal(t, "server error", wrapped1.(*HTTPError).Message)
		assert.Equal(t, "upstream error", wrapped2.(*HTTPError).Message)

		unwrapped := errors.Unwrap(wrapped2)
		assert.Equal(t, wrapped1, unwrapped)

		baseUnwrapped := errors.Unwrap(unwrapped)
		assert.Equal(t, baseErr, baseUnwrapped)
	})

	t.Run("errors.Is and errors.AsType", func(t *testing.T) {
		baseErr := errors.New("base error")
		httpErr := NewHTTPError(http.StatusBadRequest, "bad request").Wrap(baseErr)

		assert.True(t, errors.Is(httpErr, baseErr))

		unwrapped, ok := errors.AsType[*HTTPError](httpErr)

		assert.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, unwrapped.StatusCode())
	})
}

func TestHTTPStatusCoder_Interface(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"HTTPError implements HTTPStatusCoder", NewHTTPError(400, "error")},
		{"httpError implements HTTPStatusCoder", ErrBadRequest},
		{"predefined ErrNotFound implements HTTPStatusCoder", ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := tt.err.(interface{ StatusCode() int })
			assert.True(t, ok, "error should implement StatusCode method")
		})
	}
}

func TestHTTPError_UsageWithErrorHandler(t *testing.T) {
	t.Run("HTTPError can be used with StatusCode", func(t *testing.T) {
		err := NewHTTPError(http.StatusBadRequest, "validation failed")

		assert.Equal(t, http.StatusBadRequest, err.StatusCode())
	})

	t.Run("predefined error can be used with StatusCode", func(t *testing.T) {
		err := ErrNotFound

		assert.Equal(t, http.StatusNotFound, err.StatusCode())
	})
}

func TestHTTPError_MultipleWraps(t *testing.T) {
	t.Run("multiple wraps preserve chain", func(t *testing.T) {
		err1 := errors.New("original error")
		err2 := NewHTTPError(http.StatusInternalServerError, "layer 1").Wrap(err1)
		err3 := NewHTTPError(http.StatusInternalServerError, "layer 2").Wrap(err2)

		unwrapped2 := errors.Unwrap(err3)
		unwrapped1 := errors.Unwrap(unwrapped2)

		assert.Equal(t, "layer 2", err3.(*HTTPError).Message)
		assert.Equal(t, "layer 1", unwrapped2.(*HTTPError).Message)
		assert.Equal(t, err1, unwrapped1)
	})
}

func TestHTTPErrorStatusCode(t *testing.T) {
	t.Run("panics on nil error", func(t *testing.T) {
		assert.Panics(t, func() {
			HTTPErrorStatusCode(nil)
		})
	})

	t.Run("returns status code for HTTPError", func(t *testing.T) {
		err := NewHTTPError(http.StatusBadRequest, "bad request")
		got := HTTPErrorStatusCode(err)
		assert.Equal(t, http.StatusBadRequest, got)
	})

	t.Run("returns status code for httpError", func(t *testing.T) {
		got := HTTPErrorStatusCode(ErrNotFound)
		assert.Equal(t, http.StatusNotFound, got)
	})

	t.Run("returns 500 for non-status code error", func(t *testing.T) {
		err := errors.New("plain error")
		got := HTTPErrorStatusCode(err)
		assert.Equal(t, http.StatusInternalServerError, got)
	})

	t.Run("returns wrapped error status code", func(t *testing.T) {
		baseErr := ErrNotFound
		wrapped := fmt.Errorf("wrapped: %w", baseErr)
		got := HTTPErrorStatusCode(wrapped)
		assert.Equal(t, http.StatusNotFound, got)
	})

	t.Run("returns 500 for status code less than 400", func(t *testing.T) {
		err := &customStatusCoder{code: 200}
		got := HTTPErrorStatusCode(err)
		assert.Equal(t, http.StatusInternalServerError, got)
	})
}

func TestErrorStatusCode(t *testing.T) {
	t.Run("returns 0 for nil error", func(t *testing.T) {
		got := ErrorStatusCode(nil)
		assert.Equal(t, 0, got)
	})

	t.Run("returns status code for StatusCoder", func(t *testing.T) {
		err := NewHTTPError(http.StatusBadRequest, "bad request")
		got := ErrorStatusCode(err)
		assert.Equal(t, http.StatusBadRequest, got)
	})

	t.Run("returns status code for httpError", func(t *testing.T) {
		got := ErrorStatusCode(ErrNotFound)
		assert.Equal(t, http.StatusNotFound, got)
	})

	t.Run("unwraps single error to find status code", func(t *testing.T) {
		baseErr := ErrBadRequest
		wrapped := fmt.Errorf("context: %w", baseErr)
		got := ErrorStatusCode(wrapped)
		assert.Equal(t, http.StatusBadRequest, got)
	})

	t.Run("returns 0 for plain error without status code", func(t *testing.T) {
		err := errors.New("plain error")
		got := ErrorStatusCode(err)
		assert.Equal(t, 0, got)
	})

	t.Run("unwraps multiple errors in chain", func(t *testing.T) {
		baseErr := ErrNotFound
		midErr := fmt.Errorf("middle: %w", baseErr)
		topErr := fmt.Errorf("top: %w", midErr)
		got := ErrorStatusCode(topErr)
		assert.Equal(t, http.StatusNotFound, got)
	})

	t.Run("handles multi-error with Unwrap []error", func(t *testing.T) {
		baseErr := ErrBadRequest
		otherErr := errors.New("other error")
		multiErr := &multiError{errs: []error{otherErr, baseErr}}
		got := ErrorStatusCode(multiErr)
		assert.Equal(t, http.StatusBadRequest, got)
	})

	t.Run("returns 0 when multi-error has no status code", func(t *testing.T) {
		multiErr := &multiError{errs: []error{errors.New("err1"), errors.New("err2")}}
		got := ErrorStatusCode(multiErr)
		assert.Equal(t, 0, got)
	})

	t.Run("returns first non-zero status code in multi-error", func(t *testing.T) {
		baseErr1 := ErrNotFound
		baseErr2 := ErrBadRequest
		multiErr := &multiError{errs: []error{baseErr1, baseErr2}}
		got := ErrorStatusCode(multiErr)
		assert.Equal(t, http.StatusNotFound, got)
	})

	t.Run("returns 0 when reaching non-status code error in chain", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", errors.New("no status code"))
		got := ErrorStatusCode(err)
		assert.Equal(t, 0, got)
	})

	t.Run("handles custom StatusCoder", func(t *testing.T) {
		err := &customStatusCoder{code: 418}
		got := ErrorStatusCode(err)
		assert.Equal(t, 418, got)
	})
}

type customStatusCoder struct {
	code int
}

func (c *customStatusCoder) StatusCode() int {
	return c.code
}

func (c *customStatusCoder) Error() string {
	return http.StatusText(c.code)
}

type multiError struct {
	errs []error
}

func (m *multiError) Error() string {
	return "multiple errors"
}

func (m *multiError) Unwrap() []error {
	return m.errs
}
