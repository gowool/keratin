package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gowool/keratin"
)

type RequestMetadata struct {
	StatusCode int
	Error      error
	StartTime  time.Time
	EndTime    time.Time
}

// RequestLoggerAttrsFunc defines a function type for generating logging attributes based on HTTP request and response.
type RequestLoggerAttrsFunc func(w http.ResponseWriter, r *http.Request, metadata RequestMetadata) []slog.Attr

// ErrorStatusFunc return an error code.
type ErrorStatusFunc func(context.Context, error) int

type RequestLoggerConfig struct {
	// RequestLoggerAttrsFunc defines a function type for generating logging attributes based on HTTP request and response.
	RequestLoggerAttrsFunc `json:"-" yaml:"-"`

	// ErrorStatusFunc return an error code.
	ErrorStatusFunc `json:"-" yaml:"-"`

	// Logger is the logger used to log the request.
	Logger *slog.Logger `json:"-" yaml:"-"`
}

func (c *RequestLoggerConfig) SetDefaults() {
	if c.RequestLoggerAttrsFunc == nil {
		c.RequestLoggerAttrsFunc = RequestLoggerAttrs()
	}

	if c.ErrorStatusFunc == nil {
		c.ErrorStatusFunc = func(_ context.Context, err error) int {
			return keratin.HTTPErrorStatusCode(err)
		}
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}

func RequestLogger(cfg RequestLoggerConfig, skippers ...Skipper) func(keratin.Handler) keratin.Handler {
	cfg.SetDefaults()

	skip := ChainSkipper(skippers...)

	return func(next keratin.Handler) keratin.Handler {
		return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if skip(r) {
				return next.ServeHTTP(w, r)
			}

			startTime := time.Now().UTC()

			err := next.ServeHTTP(w, r)

			endTime := time.Now().UTC()

			var code int
			if err == nil {
				code = keratin.ResponseStatusCode(w)
			} else {
				code = cfg.ErrorStatusFunc(r.Context(), err)
			}

			var level slog.Level
			switch {
			case code >= http.StatusBadRequest && code < http.StatusInternalServerError:
				level = slog.LevelWarn
			case code >= http.StatusInternalServerError:
				level = slog.LevelError
			default:
				level = slog.LevelInfo
			}

			cfg.Logger.LogAttrs(
				context.Background(),
				level,
				"incoming request",
				cfg.RequestLoggerAttrsFunc(w, r, RequestMetadata{
					StatusCode: code,
					Error:      err,
					StartTime:  startTime,
					EndTime:    endTime,
				})...,
			)

			return err
		})
	}
}

func RequestLoggerAttrs() RequestLoggerAttrsFunc {
	return func(w http.ResponseWriter, r *http.Request, metadata RequestMetadata) []slog.Attr {
		size := 13

		id := r.Header.Get(keratin.HeaderXRequestID)
		if id == "" {
			id = w.Header().Get(keratin.HeaderXRequestID)
		}
		if id != "" {
			size++
		}

		referer := r.Referer()
		if referer != "" {
			size++
		}

		contentLength := r.Header.Get(keratin.HeaderContentLength)
		if contentLength != "" {
			size++
		}

		sizer := keratin.ResponseSizer(w)
		if sizer != nil {
			size++
		}

		if metadata.Error != nil {
			size++
		}

		attrs := make([]slog.Attr, 0, size)
		attrs = append(attrs,
			slog.String("latency", metadata.EndTime.Sub(metadata.StartTime).String()),
			slog.String("method", r.Method),
			slog.Int("status_code", metadata.StatusCode),
			slog.String("protocol", r.Proto),
			slog.String("host", r.Host),
			slog.String("path", r.URL.Path),
			slog.String("uri", r.RequestURI),
			slog.String("pattern", r.Pattern),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("real_ip", keratin.FromContext(r.Context()).RealIP()),
			slog.String("user_agent", r.UserAgent()),
			slog.Time("start_time", metadata.StartTime),
			slog.Time("end_time", metadata.EndTime),
		)

		if id != "" {
			attrs = append(attrs, slog.String("request_id", id))
		}

		if referer != "" {
			attrs = append(attrs, slog.String("referer", referer))
		}

		if contentLength != "" {
			attrs = append(attrs, slog.String("request_size", contentLength))
		}

		if sizer != nil {
			attrs = append(attrs, slog.Int64("response_size", sizer.Size()))
		}

		if metadata.Error != nil {
			attrs = append(attrs, slog.Any("error", metadata.Error))
		}

		return attrs
	}
}
