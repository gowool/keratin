package keratin

import (
	"errors"
	"net/http"
	"strings"
)

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request) error
}

type HandlerFunc func(http.ResponseWriter, *http.Request) error

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return f(w, r)
}

type ErrorHandlerFunc func(http.ResponseWriter, *http.Request, error)

func ErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if ResponseCommitted(w) {
		return
	}

	code := HTTPErrorStatusCode(err)

	httpErr, ok := errors.AsType[*HTTPError](err)
	if !ok {
		httpErr = NewHTTPError(code, http.StatusText(code))
	}

	if strings.Contains(r.Header.Get(HeaderAccept), MIMEApplicationJSON) {
		if err := JSON(w, code, httpErr); err == nil || ResponseCommitted(w) {
			return
		}
	}

	http.Error(w, httpErr.Message, code)
}
