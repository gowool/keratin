package ratelimit

import (
	"net/http"
	"time"
)

type Config struct {
	// TimestampFunc return current unix timestamp (seconds)
	// max value is 4294967295 -> Sun Feb 07 2106 06:28:15 GMT+0000
	//
	// Default: func() uint32 {
	//   return uint32(time.Now().Unix())
	// }
	TimestampFunc func() uint32 `json:"-" yaml:"-"`

	// IdentifierExtractor uses http.Request to extract the identifier, by a default req.RemoteAddr is used
	//
	// Default: func(req *http.Request) string {
	//   return req.RemoteAddr
	// }
	IdentifierExtractor func(*http.Request) (string, error) `json:"-" yaml:"-"`

	// Max number of recent connections during `Expiration` seconds before sending a 429 response
	//
	// Default: 5
	Max uint `env:"MAX" json:"max,omitempty" yaml:"max,omitempty"`

	// MaxFunc a function to dynamically calculate the max requests supported by the rate limiter middleware
	//
	// Default: func(*http.Request) int {
	//   return c.Max
	// }
	MaxFunc func(*http.Request) uint `json:"-" yaml:"-"`

	// Expiration is the time on how long to keep records of requests in memory
	//
	// Default: 1 * time.Minute
	Expiration time.Duration `env:"EXPIRATION" json:"expiration,omitempty,format:units" yaml:"expiration,omitempty"`

	// ExpirationFunc a function to dynamically calculate the expiration supported by the rate limiter middleware
	//
	// Default: func(*http.Request) time.Duration {
	//   return c.Expiration
	// }
	ExpirationFunc func(*http.Request) time.Duration `json:"-" yaml:"-"`

	// When set to true, the middleware will not include the rate limit headers (X-RateLimit-* and Retry-After) in the response.
	//
	// Default: false
	DisableHeaders bool `env:"DISABLE_HEADERS" json:"disableHeaders,omitempty" yaml:"disableHeaders,omitempty"`

	// DisableValueRedaction turns off masking limiter keys in logs and error messages when set to true.
	//
	// Default: false
	DisableValueRedaction bool `env:"DISABLE_VALUE_REDACTION" json:"disableValueRedaction,omitempty" yaml:"disableValueRedaction,omitempty"`
}

func (c *Config) SetDefaults() {
	if c.TimestampFunc == nil {
		c.TimestampFunc = timestampFunc
	}

	if c.IdentifierExtractor == nil {
		c.IdentifierExtractor = func(r *http.Request) (string, error) {
			return r.RemoteAddr, nil
		}
	}

	if c.Max == 0 {
		c.Max = 5
	}
	if c.MaxFunc == nil {
		c.MaxFunc = func(*http.Request) uint {
			return c.Max
		}
	}

	if c.Expiration == 0 {
		c.Expiration = 1 * time.Minute
	}
	if c.ExpirationFunc == nil {
		c.ExpirationFunc = func(*http.Request) time.Duration {
			return c.Expiration
		}
	}
}
