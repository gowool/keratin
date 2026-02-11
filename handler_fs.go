package keratin

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

const (
	cacheControl          = "max-age=2592000, stale-while-revalidate=86400"
	contentSecurityPolicy = "default-src 'none'; connect-src 'self'; image-src 'self'; media-src 'self'; style-src 'unsafe-inline'; sandbox"
)

var ErrFileNotFound = ErrNotFound.Wrap(errors.New("file not found"))

type BeforeServeFunc func(http.ResponseWriter, *http.Request, fs.File, fs.FileInfo)

func ContentSecurityPolicy(data ...string) BeforeServeFunc {
	content := contentSecurityPolicy
	if len(data) > 0 {
		content = strings.Join(data, "; ")
	}

	return func(w http.ResponseWriter, r *http.Request, f fs.File, fi fs.FileInfo) {
		w.Header().Set(HeaderContentSecurityPolicy, content)
	}
}

func CacheControl(data ...string) BeforeServeFunc {
	content := cacheControl
	if len(data) > 0 {
		content = strings.Join(data, ", ")
	}

	return func(w http.ResponseWriter, r *http.Request, f fs.File, fi fs.FileInfo) {
		w.Header().Set(HeaderCacheControl, content)
	}
}

// Attachment set header to send a response as attachment, prompting client to save the file.
func Attachment(name string) BeforeServeFunc {
	return contentDisposition(name, "attachment")
}

// Inline set header to send a response as inline, opening the file in the browser.
func Inline(name string) BeforeServeFunc {
	return contentDisposition(name, "inline")
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func contentDisposition(name, dispositionType string) BeforeServeFunc {
	return func(w http.ResponseWriter, r *http.Request, f fs.File, fi fs.FileInfo) {
		w.Header().Set(HeaderContentDisposition, fmt.Sprintf(`%s; filename="%s"`, dispositionType, quoteEscaper.Replace(name)))
	}
}

// FileFS serves the specified filename from fsys.
func FileFS(fsys fs.FS, filename string, beforeServe ...BeforeServeFunc) HandlerFunc {
	if fsys == nil {
		panic("fsys is nil")
	}
	if filename == "" {
		panic("filename is empty")
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		f, err := fsys.Open(filename)
		if err != nil {
			return errors.Join(ErrFileNotFound, err)
		}
		defer func() { _ = f.Close() }()

		fi, err := f.Stat()
		if err != nil {
			return err
		}

		// if it is a directory try to open its index.html file
		if fi.IsDir() {
			filename = filepath.ToSlash(filepath.Join(filename, IndexPage))
			f, err = fsys.Open(filename)
			if err != nil {
				return errors.Join(ErrFileNotFound, err)
			}
			defer func() { _ = f.Close() }()

			fi, err = f.Stat()
			if err != nil {
				return err
			}
		}

		frs, ok := f.(io.ReadSeeker)
		if !ok {
			return errors.New("file does not implement io.ReadSeeker")
		}

		for _, bs := range beforeServe {
			bs(w, r, f, fi)
		}

		http.ServeContent(w, r, fi.Name(), fi.ModTime(), frs)

		return nil
	}
}

// StaticFS serve static directory content from fsys.
//
// If a file resource is missing and indexFallback is set, the request
// will be forwarded to the base index.html (useful for SPA with pretty urls).
//
// Special redirects:
//   - if "path" is a file that ends in index.html, it is redirected to its non-index.html version (eg. /test/index.html -> /test/)
//   - if "path" is a directory that has index.html, the index.html file is rendered,
//     otherwise if missing - returns ErrFileNotFound or fallback to the root index.html if indexFallback is set
//
// Example:
//
//	fsys := os.DirFS("./public")
//	router.GET("/files/{path...}", StaticFS(fsys, "path", false))
func StaticFS(fsys fs.FS, param string, indexFallback bool, beforeServe ...BeforeServeFunc) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		filename := r.PathValue(param)
		filename = filepath.ToSlash(filepath.Clean(strings.TrimPrefix(filename, "/")))

		// eagerly check for directory traversal
		//
		// note: this is just out of an abundance of caution because the fs.FS implementation could be non-std,
		// but usually shouldn't be necessary since os.DirFS.Open is expected to fail if the filename starts with dots
		if len(filename) > 2 && filename[0] == '.' && filename[1] == '.' && (filename[2] == '/' || filename[2] == '\\') {
			if indexFallback && filename != IndexPage {
				return FileFS(fsys, IndexPage, beforeServe...).ServeHTTP(w, r)
			}
			return ErrFileNotFound
		}

		fi, err := fs.Stat(fsys, filename)
		if err != nil {
			if indexFallback && filename != IndexPage {
				return FileFS(fsys, IndexPage, beforeServe...).ServeHTTP(w, r)
			}
			return errors.Join(ErrFileNotFound, err)
		}

		if fi.IsDir() {
			// redirect to a canonical dir url, aka. with trailing slash
			if !strings.HasSuffix(r.URL.Path, "/") {
				http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
				return nil
			}
		} else {
			urlPath := r.URL.Path
			if strings.HasSuffix(urlPath, "/") {
				// redirect to a non-trailing slash file route
				if urlPath = strings.TrimRight(urlPath, "/"); len(urlPath) > 0 {
					http.Redirect(w, r, safeRedirectPath(urlPath), http.StatusMovedPermanently)
					return nil
				}
			} else if stripped, ok := strings.CutSuffix(urlPath, IndexPage); ok {
				// redirect without the index.html
				http.Redirect(w, r, safeRedirectPath(stripped), http.StatusMovedPermanently)
				return nil
			}
		}

		fileErr := FileFS(fsys, filename, beforeServe...).ServeHTTP(w, r)

		if fileErr != nil && indexFallback && filename != IndexPage && errors.Is(fileErr, fs.ErrNotExist) {
			return FileFS(fsys, IndexPage, beforeServe...).ServeHTTP(w, r)
		}

		return fileErr
	}
}

// safeRedirectPath normalizes the path string by replacing all beginning slashes
// (`\\`, `//`, `\/`) with a single forward slash to prevent open redirect attacks
func safeRedirectPath(path string) string {
	if len(path) > 1 && (path[0] == '\\' || path[0] == '/') && (path[1] == '\\' || path[1] == '/') {
		path = "/" + strings.TrimLeft(path, `/\`)
	}
	return path
}
