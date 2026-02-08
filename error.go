package keratin

import (
	"fmt"
	"net/http"
)

// Following errors can produce HTTP status code by implementing HTTPStatusCoder interface
var (
	ErrBadRequest                    = &httpError{http.StatusBadRequest}                    // HTTP 400 Bad Request
	ErrUnauthorized                  = &httpError{http.StatusUnauthorized}                  // HTTP 401 Unauthorized
	ErrPaymentRequired               = &httpError{http.StatusPaymentRequired}               // HTTP 402 Payment Required
	ErrForbidden                     = &httpError{http.StatusForbidden}                     // HTTP 403 Forbidden
	ErrNotFound                      = &httpError{http.StatusNotFound}                      // HTTP 404 Not Found
	ErrMethodNotAllowed              = &httpError{http.StatusMethodNotAllowed}              // HTTP 405 Method Not Allowed
	ErrNotAcceptable                 = &httpError{http.StatusNotAcceptable}                 // HTTP 406 Not Acceptable
	ErrProxyAuthRequired             = &httpError{http.StatusProxyAuthRequired}             // HTTP 407 Proxy AuthRequired
	ErrRequestTimeout                = &httpError{http.StatusRequestTimeout}                // HTTP 408 Request Timeout
	ErrConflict                      = &httpError{http.StatusConflict}                      // HTTP 409 Conflict
	ErrGone                          = &httpError{http.StatusGone}                          // HTTP 410 Gone
	ErrLengthRequired                = &httpError{http.StatusLengthRequired}                // HTTP 411 Length Required
	ErrPreconditionFailed            = &httpError{http.StatusPreconditionFailed}            // HTTP 412 Precondition Failed
	ErrRequestEntityTooLarge         = &httpError{http.StatusRequestEntityTooLarge}         // HTTP 413 Payload Too Large
	ErrRequestURITooLong             = &httpError{http.StatusRequestURITooLong}             // HTTP 414 URI Too Long
	ErrUnsupportedMediaType          = &httpError{http.StatusUnsupportedMediaType}          // HTTP 415 Unsupported Media Type
	ErrRequestedRangeNotSatisfiable  = &httpError{http.StatusRequestedRangeNotSatisfiable}  // HTTP 416 Range Not Satisfiable
	ErrExpectationFailed             = &httpError{http.StatusExpectationFailed}             // HTTP 417 Expectation Failed
	ErrTeapot                        = &httpError{http.StatusTeapot}                        // HTTP 418 I'm a teapot
	ErrMisdirectedRequest            = &httpError{http.StatusMisdirectedRequest}            // HTTP 421 Misdirected Request
	ErrUnprocessableEntity           = &httpError{http.StatusUnprocessableEntity}           // HTTP 422 Unprocessable Entity
	ErrLocked                        = &httpError{http.StatusLocked}                        // HTTP 423 Locked
	ErrFailedDependency              = &httpError{http.StatusFailedDependency}              // HTTP 424 Failed Dependency
	ErrTooEarly                      = &httpError{http.StatusTooEarly}                      // HTTP 425 Too Early
	ErrUpgradeRequired               = &httpError{http.StatusUpgradeRequired}               // HTTP 426 Upgrade Required
	ErrPreconditionRequired          = &httpError{http.StatusPreconditionRequired}          // HTTP 428 Precondition Required
	ErrTooManyRequests               = &httpError{http.StatusTooManyRequests}               // HTTP 429 Too Many Requests
	ErrRequestHeaderFieldsTooLarge   = &httpError{http.StatusRequestHeaderFieldsTooLarge}   // HTTP 431 Request Header Fields Too Large
	ErrUnavailableForLegalReasons    = &httpError{http.StatusUnavailableForLegalReasons}    // HTTP 451 Unavailable For Legal Reasons
	ErrInternalServerError           = &httpError{http.StatusInternalServerError}           // HTTP 500 Internal Server Error
	ErrNotImplemented                = &httpError{http.StatusNotImplemented}                // HTTP 501 Not Implemented
	ErrBadGateway                    = &httpError{http.StatusBadGateway}                    // HTTP 502 Bad Gateway
	ErrServiceUnavailable            = &httpError{http.StatusServiceUnavailable}            // HTTP 503 Service Unavailable
	ErrGatewayTimeout                = &httpError{http.StatusGatewayTimeout}                // HTTP 504 Gateway Timeout
	ErrHTTPVersionNotSupported       = &httpError{http.StatusHTTPVersionNotSupported}       // HTTP 505 HTTP Version Not Supported
	ErrVariantAlsoNegotiates         = &httpError{http.StatusVariantAlsoNegotiates}         // HTTP 506 Variant Also Negotiates
	ErrInsufficientStorage           = &httpError{http.StatusInsufficientStorage}           // HTTP 507 Insufficient Storage
	ErrLoopDetected                  = &httpError{http.StatusLoopDetected}                  // HTTP 508 Loop Detected
	ErrNotExtended                   = &httpError{http.StatusNotExtended}                   // HTTP 510 Not Extended
	ErrNetworkAuthenticationRequired = &httpError{http.StatusNetworkAuthenticationRequired} // HTTP 511 Network Authentication Required
)

type HTTPError struct {
	Code    int    `json:"-"`
	Message string `json:"message"`
	err     error
}

// NewHTTPError creates a new instance of HTTPError
func NewHTTPError(code int, message string) *HTTPError {
	return &HTTPError{
		Code:    code,
		Message: message,
	}
}

// StatusCode returns status code for HTTP response
func (he *HTTPError) StatusCode() int {
	return he.Code
}

// Error makes it compatible with an `error` interface.
func (he *HTTPError) Error() string {
	msg := he.Message
	if msg == "" {
		msg = http.StatusText(he.Code)
	}
	if he.err == nil {
		return fmt.Sprintf("code=%d, message=%v", he.Code, msg)
	}
	return fmt.Sprintf("code=%d, message=%v, err=%v", he.Code, msg, he.err.Error())
}

// Wrap returns a new HTTPError with given errors wrapped inside
func (he *HTTPError) Wrap(err error) error {
	return &HTTPError{
		Code:    he.Code,
		Message: he.Message,
		err:     err,
	}
}

func (he *HTTPError) Unwrap() error {
	return he.err
}

type httpError struct {
	code int
}

func (he httpError) StatusCode() int {
	return he.code
}

func (he httpError) Error() string {
	return http.StatusText(he.code) // does not include status code
}

func (he httpError) Wrap(err error) error {
	return &HTTPError{
		Code:    he.code,
		Message: http.StatusText(he.code),
		err:     err,
	}
}

func HTTPErrorStatusCode(err error) int {
	if err == nil {
		panic("cannot get status code from nil error")
	}

	if code := ErrorStatusCode(err); code >= 400 {
		return code
	}

	return http.StatusInternalServerError
}

func ErrorStatusCode(err error) int {
	if err == nil {
		return 0
	}

	for {
		switch t := err.(type) {
		case StatusCoder:
			return t.StatusCode()
		case interface{ Unwrap() error }:
			err = t.Unwrap()
			continue
		case interface{ Unwrap() []error }:
			for _, e := range t.Unwrap() {
				if code := ErrorStatusCode(e); code != 0 {
					return code
				}
			}
			return 0
		default:
			return 0
		}
	}
}
