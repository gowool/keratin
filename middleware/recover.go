package middleware

import (
	"errors"
	"fmt"
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
