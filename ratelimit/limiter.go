package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gowool/keratin"
)

// ErrRateLimitExceeded denotes an error raised when a rate limit is exceeded
var ErrRateLimitExceeded = keratin.NewHTTPError(http.StatusTooManyRequests, "Rate limit exceeded.")

// Storage is used to store the state of Limiter
type Storage interface {
	// Get gets the value for the given key with a context.
	// `nil, nil` is returned when the key does not exist
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores the given value for the given key with an expiration value.
	Set(ctx context.Context, key string, value []byte, exp time.Duration) error
}

// Limiter implements the sliding-window rate limiting strategy
type Limiter struct {
	cfg     Config
	manager *manager
	mu      *sync.RWMutex
}

func NewLimiter(cfg Config) *Limiter {
	return NewLimiterWithStorage(cfg, nil)
}

func NewLimiterWithStorage(cfg Config, storage Storage) *Limiter {
	cfg.SetDefaults()

	if storage == nil {
		storage = NewMemoryStorage(cfg.TimestampFunc)
	}

	return &Limiter{
		cfg:     cfg,
		mu:      new(sync.RWMutex),
		manager: newManager(storage, !cfg.DisableValueRedaction),
	}
}

func (l *Limiter) Allow(w http.ResponseWriter, r *http.Request) error {
	key, err := l.cfg.IdentifierExtractor(r)
	if err != nil {
		return keratin.ErrForbidden.Wrap(fmt.Errorf("rate_limiter: failed to extract identifier: %w", err))
	}

	maxRequests := l.maxFunc(r)
	expiration := l.expirationFunc(r)

	// Lock entry
	l.mu.Lock()

	// Get entry from pool and release when finished
	entry, err := l.manager.get(r.Context(), key)
	if err != nil {
		l.mu.Unlock()
		return err
	}

	// Get timestamp
	ts := uint64(l.cfg.TimestampFunc())

	// Set expiration if entry does not exist
	if entry.exp == 0 {
		entry.exp = ts + expiration
	} else if ts >= entry.exp {
		// The entry has expired, handle the expiration.
		// Set the prevHits to the current hits and reset the hits to 0.
		entry.prevHits = entry.currHits

		// Reset the current hits to 0.
		entry.currHits = 0

		// Check how much into the current window it currently is and sets the
		// expiry based on that; otherwise, this would only reset on
		// the next request and not show the correct expiry.
		elapsed := ts - entry.exp
		if elapsed >= expiration {
			entry.exp = ts + expiration
		} else {
			entry.exp = ts + expiration - elapsed
		}
	}

	// Increment hits
	entry.currHits++

	// Calculate when it resets in seconds
	resetInSec := entry.exp - ts

	// weight = time until current window reset / total window length
	weight := float64(resetInSec) / float64(expiration)

	// rate = request count in previous window - weight + request count in current window
	rate := int(float64(entry.prevHits)*weight) + entry.currHits

	// Calculate how many hits can be made based on the current rate
	remaining := maxRequests - rate

	// Update storage. Garbage collect when the next window ends.
	// |--------------------------|--------------------------|
	//               ^            ^               ^          ^
	//              ts         e.exp   End sample window   End next window
	//               <------------>
	// 				   Reset In Sec
	// resetInSec = e.exp - ts - time until end of current window.
	// duration + expiration = end of next window.
	// Because we don't want to garbage collect in the middle of a window
	// we add the expiration to the duration.
	// Otherwise, after the end of "sample window", attackers could launch
	// a new request with the full window length.
	if setErr := l.manager.set(r.Context(), key, entry, time.Duration(resetInSec+expiration)*time.Second); setErr != nil { //nolint:gosec // Not a concern
		l.mu.Unlock()
		return fmt.Errorf("rate_limiter: failed to persist state: %w", setErr)
	}

	// Unlock entry
	l.mu.Unlock()

	// Check if hits exceed the cfg.Max
	if remaining < 0 {
		// Return response with Retry-After header
		// https://tools.ietf.org/html/rfc6584
		if !l.cfg.DisableHeaders {
			w.Header().Set(keratin.HeaderRetryAfter, strconv.FormatUint(resetInSec, 10))
		}
		return ErrRateLimitExceeded
	}

	if !l.cfg.DisableHeaders {
		w.Header().Set(keratin.HeaderXRateLimitLimit, strconv.Itoa(maxRequests))
		w.Header().Set(keratin.HeaderXRateLimitRemaining, strconv.Itoa(remaining))
		w.Header().Set(keratin.HeaderXRateLimitReset, strconv.FormatUint(resetInSec, 10))
	}

	return nil
}

func (l *Limiter) maxFunc(r *http.Request) int {
	if m := l.cfg.MaxFunc(r); m > 0 {
		return int(m)
	}
	return int(l.cfg.Max)
}

func (l *Limiter) expirationFunc(r *http.Request) uint64 {
	if exp := l.cfg.ExpirationFunc(r); exp > 0 {
		return uint64(exp.Seconds())
	}
	return uint64(l.cfg.Expiration.Seconds())
}

func timestampFunc() uint32 {
	return uint32(time.Now().Unix())
}
