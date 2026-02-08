package keratin

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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
	panic("ResponseWriter does not implement Sizer interface")
}

func ResponseCommitted(w http.ResponseWriter) bool {
	if c := ResponseCommitter(w); c != nil {
		return c.Committed()
	}
	panic("ResponseWriter does not implement Committer interface")
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
