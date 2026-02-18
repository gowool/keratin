package keratin

import (
	"net/http"
	"strings"

	"github.com/gowool/keratin/internal"
)

func Pattern(r *http.Request) string {
	pattern := r.Pattern
	if index := strings.IndexRune(pattern, ' '); index > -1 {
		pattern = pattern[index+1:]
	}
	return pattern
}

// Scheme returns the HTTP protocol scheme, `http` or `https`.
func Scheme(r *http.Request) string {
	// Can't use `r.Request.URL.Scheme`
	// See: https://groups.google.com/forum/#!topic/golang-nuts/pMUkBlQBDF0
	if r.TLS != nil {
		return "https"
	}
	if scheme := r.Header.Get(HeaderXForwardedProto); scheme != "" {
		return scheme
	}
	if scheme := r.Header.Get(HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}
	if ssl := r.Header.Get(HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}
	if scheme := r.Header.Get(HeaderXUrlScheme); scheme != "" {
		return scheme
	}
	return "http"
}

// ParseAcceptLanguage parses the 'Accept-Language' HTTP header
// into a slice of language codes sorted by their order of appearance.
func ParseAcceptLanguage(acceptLanguageHeader string) []string {
	if acceptLanguageHeader == "" {
		return make([]string, 0)
	}

	options := strings.Split(acceptLanguageHeader, ",")
	l := len(options)
	languages := make([]string, l)

	for i := range l {
		locale := strings.SplitN(options[i], ";", 2)
		languages[i] = strings.Trim(locale[0], " ")
	}

	return languages
}

// NegotiateFormat returns an acceptable Accept format.
func NegotiateFormat(acceptHeader string, offered ...string) string {
	accepted := internal.ParseAcceptHeader(acceptHeader)

	return internal.NegotiateFormat(accepted, offered...)
}
