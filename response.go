package keratin

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/gowool/keratin/internal"
)

var (
	_ http.Flusher  = (*response)(nil)
	_ http.Hijacker = (*response)(nil)
	_ http.Pusher   = (*response)(nil)
	_ io.ReaderFrom = (*response)(nil)
	_ RWUnwrapper   = (*response)(nil)
	_ Committer     = (*response)(nil)
	_ StatusCoder   = (*response)(nil)
	_ Sizer         = (*response)(nil)
)

// RWUnwrapper specifies that http.ResponseWriter could be "unwrapped"
// (usually used with [http.ResponseController]).
type RWUnwrapper interface {
	Unwrap() http.ResponseWriter
}

type StatusCoder interface {
	StatusCode() int
}

type Sizer interface {
	Size() int64
}

type Committer interface {
	Committed() bool
}

func ResponseStatusCode(w http.ResponseWriter) int {
	if sc := ResponseStatusCoder(w); sc != nil {
		return sc.StatusCode()
	}
	return 0
}

func ResponseSize(w http.ResponseWriter) int64 {
	if sz := ResponseSizer(w); sz != nil {
		return sz.Size()
	}
	return 0
}

func ResponseCommitted(w http.ResponseWriter) bool {
	if c := ResponseCommitter(w); c != nil {
		return c.Committed()
	}
	return false
}

func ResponseCommitter(w http.ResponseWriter) Committer {
	for {
		switch t := w.(type) {
		case Committer:
			return t
		case RWUnwrapper:
			w = t.Unwrap()
			continue
		default:
			return nil
		}
	}
}

func ResponseStatusCoder(w http.ResponseWriter) StatusCoder {
	for {
		switch t := w.(type) {
		case StatusCoder:
			return t
		case RWUnwrapper:
			w = t.Unwrap()
			continue
		default:
			return nil
		}
	}
}

func ResponseSizer(w http.ResponseWriter) Sizer {
	for {
		switch t := w.(type) {
		case Sizer:
			return t
		case RWUnwrapper:
			w = t.Unwrap()
			continue
		default:
			return nil
		}
	}
}

func ResponseReaderFrom(w http.ResponseWriter) io.ReaderFrom {
	for {
		switch t := w.(type) {
		case io.ReaderFrom:
			return t
		case RWUnwrapper:
			w = t.Unwrap()
			continue
		default:
			return nil
		}
	}
}

type response struct {
	http.ResponseWriter
	committed bool
	code      int
	size      int64
}

func (r *response) reset(w http.ResponseWriter) {
	r.ResponseWriter = w
	r.committed = false
	r.code = 0
	r.size = 0
}

func (r *response) Size() int64 {
	return r.size
}

func (r *response) StatusCode() int {
	return r.code
}

func (r *response) Committed() bool {
	return r.committed
}

// Unwrap returns the original http.ResponseWriter.
// ResponseController can be used to access the original http.ResponseWriter.
// See [https://go.dev/blog/go1.20]
func (r *response) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// WriteHeader sends an HTTP response header with Status code. If WriteHeader is
// not called explicitly, the first call to Write will trigger an implicit
// WriteHeader(http.StatusOK). Thus explicit calls to WriteHeader are mainly
// used to send error codes.
func (r *response) WriteHeader(statusCode int) {
	if r.committed {
		return
	}

	r.committed = true
	r.code = statusCode

	r.Header().Del(HeaderContentLength)
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write writes the data to the connection as part of an HTTP reply.
func (r *response) Write(b []byte) (n int, err error) {
	if !r.committed {
		r.WriteHeader(http.StatusOK)
	}

	n, err = r.ResponseWriter.Write(b)
	r.size += int64(n)
	return
}

// Flush implements the http.Flusher interface to allow an HTTP handler to flush
// buffered data to the client.
// See [http.Flusher](https://golang.org/pkg/net/http/#Flusher)
func (r *response) Flush() {
	if err := http.NewResponseController(r.ResponseWriter).Flush(); err != nil && errors.Is(err, http.ErrNotSupported) {
		panic(fmt.Errorf("response writer %T does not support flushing (http.Flusher interface)", r.ResponseWriter))
	}
}

// Hijack implements the http.Hijacker interface to allow an HTTP handler to
// take over the connection.
// See [http.Hijacker](https://golang.org/pkg/net/http/#Hijacker)
func (r *response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(r.ResponseWriter).Hijack()
}

// Push implements [http.Pusher] to indicate HTTP/2 server push support.
func (r *response) Push(target string, opts *http.PushOptions) error {
	w := r.ResponseWriter
	for {
		switch p := w.(type) {
		case http.Pusher:
			return p.Push(target, opts)
		case RWUnwrapper:
			w = p.Unwrap()
		default:
			return http.ErrNotSupported
		}
	}
}

// ReadFrom implements [io.ReaderFrom] by checking if the underlying writer supports it.
// Otherwise calls [io.Copy].
func (r *response) ReadFrom(reader io.Reader) (n int64, err error) {
	if !r.committed {
		r.WriteHeader(http.StatusOK)
	}

	w := r.ResponseWriter
	for {
		switch rf := w.(type) {
		case io.ReaderFrom:
			return rf.ReadFrom(reader)
		case RWUnwrapper:
			w = rf.Unwrap()
		default:
			n, err = io.Copy(r.ResponseWriter, reader)
			r.size = n
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, i any, indent string) error {
	w = newDelayedStatusWriter(w)

	w.Header().Set(HeaderContentType, MIMEApplicationJSON)
	w.WriteHeader(status)

	return internal.MarshalJSON(w, i, indent)
}

// JSON sends a JSON response with status code.
func JSON(w http.ResponseWriter, status int, i any) error {
	return writeJSON(w, status, i, "")
}

// JSONPretty sends a pretty-print JSON with status code.
func JSONPretty(w http.ResponseWriter, status int, i any, indent string) error {
	return writeJSON(w, status, i, indent)
}

func JSONBlob(w http.ResponseWriter, status int, b []byte) error {
	return Blob(w, status, MIMEApplicationJSON, b)
}

// HTML writes an HTML response.
func HTML(w http.ResponseWriter, status int, data string) error {
	return HTMLBlob(w, status, internal.StringToBytes(data))
}

// HTMLBlob sends an HTTP blob response with status code.
func HTMLBlob(w http.ResponseWriter, status int, b []byte) error {
	return Blob(w, status, MIMETextHTMLCharsetUTF8, b)
}

// TextPlain writes a plain string response.
func TextPlain(w http.ResponseWriter, status int, data string) error {
	return Blob(w, status, MIMETextPlainCharsetUTF8, internal.StringToBytes(data))
}

func writeXML(w http.ResponseWriter, status int, i any, indent string) (err error) {
	if w.Header().Get(HeaderContentType) == "" {
		w.Header().Set(HeaderContentType, MIMEApplicationXMLCharsetUTF8)
	}
	w.WriteHeader(status)

	enc := xml.NewEncoder(w)
	enc.Indent("", indent)

	defer func() { err = errors.Join(err, enc.Close()) }()

	if _, err = w.Write(internal.StringToBytes(xml.Header)); err != nil {
		return
	}

	err = enc.Encode(i)
	return
}

// XML writes an XML response.
// It automatically prepends the generic [xml.Header] string to the response.
func XML(w http.ResponseWriter, status int, i any) error {
	return writeXML(w, status, i, "")
}

// XMLPretty sends a pretty-print XML with status code.
// It automatically prepends the generic [xml.Header] string to the response.
func XMLPretty(w http.ResponseWriter, status int, i any, indent string) error {
	return writeXML(w, status, i, indent)
}

// XMLBlob sends an XML blob response with status code.
func XMLBlob(w http.ResponseWriter, status int, b []byte) error {
	return Blob(w, status, MIMEApplicationXMLCharsetUTF8, b)
}

// Blob writes a blob (bytes slice) response.
func Blob(w http.ResponseWriter, status int, contentType string, b []byte) error {
	w.Header().Set(HeaderContentType, contentType)
	w.WriteHeader(status)
	_, err := w.Write(b)
	return err
}

// Stream streams the specified reader into the response.
func Stream(w http.ResponseWriter, status int, contentType string, reader io.Reader) error {
	w.Header().Set(HeaderContentType, contentType)
	w.WriteHeader(status)
	_, err := io.Copy(w, reader)
	return err
}

// delayedStatusWriter is a wrapper around http.ResponseWriter that delays writing the status code until first Write is called.
// This allows (global) error handler to decide correct status code to be sent to the client.
type delayedStatusWriter struct {
	http.ResponseWriter
	committed bool
	status    int
}

func newDelayedStatusWriter(w http.ResponseWriter) *delayedStatusWriter {
	return &delayedStatusWriter{ResponseWriter: w}
}

func (w *delayedStatusWriter) WriteHeader(statusCode int) {
	// in case something else writes status code explicitly before us we need mark response committed
	w.status = statusCode
}

func (w *delayedStatusWriter) Write(data []byte) (int, error) {
	if !w.committed {
		w.committed = true
		if w.status == 0 {
			w.status = http.StatusOK
		}
		w.ResponseWriter.WriteHeader(w.status)
	}
	return w.ResponseWriter.Write(data)
}

func (w *delayedStatusWriter) Flush() {
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if err != nil && errors.Is(err, http.ErrNotSupported) {
		panic(errors.New("response writer flushing is not supported"))
	}
}

func (w *delayedStatusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *delayedStatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
