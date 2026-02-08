package middleware

import (
	"bytes"
	"io"
	"net/http"
	"sync"

	"github.com/gowool/keratin"
)

func BodyRereadable(skippers ...Skipper) func(keratin.Handler) keratin.Handler {
	skip := ChainSkipper(skippers...)

	pool := &sync.Pool{
		New: func() any { return new(rereadableReadCloser) },
	}

	return func(next keratin.Handler) keratin.Handler {
		return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if skip(r) {
				return next.ServeHTTP(w, r)
			}

			// wrap the request body to allow multiple reads
			read := pool.Get().(*rereadableReadCloser)
			read.Reset(r.Body)
			r.Body = read

			defer func() {
				r.Body = read.ReadCloser
				read.Reset(nil)
				pool.Put(read)
			}()

			return next.ServeHTTP(w, r)
		})
	}
}

// rereadableReadCloser defines a wrapper around a io.ReadCloser reader
// allowing to read the original reader multiple times.
type rereadableReadCloser struct {
	io.ReadCloser

	copy   *bytes.Buffer
	active io.Reader
}

// Read implements the standard io.Reader interface.
//
// It reads up to len(b) bytes into b and at at the same time writes
// the read data into an internal bytes buffer.
//
// On EOF the r is "rewinded" to allow reading from r multiple times.
func (r *rereadableReadCloser) Read(b []byte) (int, error) {
	if r.active == nil {
		if r.copy == nil {
			r.copy = new(bytes.Buffer)
		}
		r.active = io.TeeReader(r.ReadCloser, r.copy)
	}

	n, err := r.active.Read(b)
	if err == io.EOF {
		r.Reread()
	}

	return n, err
}

// Reread satisfies the [Rereader] interface and resets the r internal state to allow rereads.
//
// note: not named Reset to avoid conflicts with other reader interfaces.
func (r *rereadableReadCloser) Reread() {
	if r.copy == nil || r.copy.Len() == 0 {
		return // nothing to reset or it has been already reset
	}

	oldCopy := r.copy
	r.copy = new(bytes.Buffer)
	r.active = io.TeeReader(oldCopy, r.copy)
}

func (r *rereadableReadCloser) Reset(rc io.ReadCloser) {
	r.ReadCloser = rc
	r.copy = nil
	r.active = nil
}
