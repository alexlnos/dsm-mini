// Package cache is a small value cache with a time to live.
//
// It exists so that frequent polling from a phone does not turn into a storm
// of requests to the NAS: device details change once a month, the package
// list once a week, and asking for them on every screen refresh is pointless.
package cache

import (
	"context"
	"sync"
	"time"
)

type entry[T any] struct {
	value   T
	expires time.Time
	// err is remembered alongside the value: otherwise, with the NAS down,
	// every request would wait out the timeout again.
	err error
}

// Cache holds a single value and refreshes it no more often than once per ttl.
type Cache[T any] struct {
	ttl time.Duration

	mu      sync.Mutex
	current *entry[T]
	// loading keeps several concurrent requests from hitting the NAS at once:
	// the first one goes for the data, the rest wait for its result.
	loading chan struct{}
}

// New creates a cache with the given time to live.
func New[T any](ttl time.Duration) *Cache[T] {
	return &Cache[T]{ttl: ttl}
}

// Get returns the cached value or computes it through load.
func (c *Cache[T]) Get(ctx context.Context, load func(context.Context) (T, error)) (T, error) {
	for {
		c.mu.Lock()

		if c.current != nil && time.Now().Before(c.current.expires) {
			value, err := c.current.value, c.current.err
			c.mu.Unlock()
			return value, err
		}

		if c.loading != nil {
			// Someone already went for the data — wait and take their result.
			wait := c.loading
			c.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				var zero T
				return zero, ctx.Err()
			}
		}

		done := make(chan struct{})
		c.loading = done
		c.mu.Unlock()

		value, err := load(ctx)

		c.mu.Lock()
		c.current = &entry[T]{value: value, err: err, expires: time.Now().Add(c.ttl)}
		c.loading = nil
		c.mu.Unlock()
		close(done)

		return value, err
	}
}

// Invalidate drops the cache: needed after actions that change state.
func (c *Cache[T]) Invalidate() {
	c.mu.Lock()
	c.current = nil
	c.mu.Unlock()
}

// GetStale returns the last known value immediately and starts the refresh
// in the background.
//
// For the screen that matters more than being a second fresh: usage figures
// and the task list are updated by the next poll anyway, while waiting for
// the NAS reads as the app hanging. With no value yet it behaves like a
// plain Get and waits.
func (c *Cache[T]) GetStale(ctx context.Context, load func(context.Context) (T, error)) (T, error) {
	c.mu.Lock()
	current := c.current
	fresh := current != nil && time.Now().Before(current.expires)
	alreadyLoading := c.loading != nil
	c.mu.Unlock()

	if current == nil {
		// First request: there is nothing to show, so we have to wait.
		return c.Get(ctx, load)
	}
	if !fresh && !alreadyLoading {
		// Refresh in the background: the request context is no good for that —
		// it ends with the response and would cut the load off halfway.
		go func() {
			background, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, _ = c.Get(background, load)
		}()
	}
	return current.value, current.err
}
