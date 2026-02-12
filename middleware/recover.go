package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"

	"github.com/gowool/keratin"
)

type RecoverConfig struct {
	// Size of the stack to be printed.
	// Optional. Default value 2KB.
	StackSize int `env:"STACK_SIZE" json:"stackSize,omitempty" yaml:"stackSize,omitempty"`
}

func (c *RecoverConfig) SetDefaults() {
	if c.StackSize == 0 {
		c.StackSize = 2 << 10 // 2KB
	}
}

func HTTPRecover(cfg RecoverConfig, logger *slog.Logger) func(next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	logger = logger.WithGroup("http_recover")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					recoverErr, ok := rec.(error)
					if !ok {
						recoverErr = fmt.Errorf("%v", rec)
					} else if errors.Is(recoverErr, http.ErrAbortHandler) {
						// don't recover ErrAbortHandler so the response to the client can be aborted
						panic(recoverErr)
					}

					stack := make([]byte, cfg.StackSize)
					length := runtime.Stack(stack, true)

					internal := fmt.Errorf("[PANIC RECOVER] %w %s", recoverErr, stack[:length])
					logger.Error("panic recovered", "error", internal)

					if committer := keratin.ResponseCommitter(w); committer != nil && committer.Committed() {
						return
					}

					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func Recover(cfg RecoverConfig) func(keratin.Handler) keratin.Handler {
	cfg.SetDefaults()

	return func(next keratin.Handler) keratin.Handler {
		return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					recoverErr, ok := rec.(error)
					if !ok {
						recoverErr = fmt.Errorf("%v", rec)
					} else if errors.Is(recoverErr, http.ErrAbortHandler) {
						// don't recover ErrAbortHandler so the response to the client can be aborted
						panic(recoverErr)
					}

					stack := make([]byte, cfg.StackSize)
					length := runtime.Stack(stack, true)
					internal := fmt.Errorf("[PANIC RECOVER] %w %s", recoverErr, stack[:length])
					err = keratin.ErrInternalServerError.Wrap(internal)
				}
			}()

			err = next.ServeHTTP(w, r)

			return err
		})
	}
}
