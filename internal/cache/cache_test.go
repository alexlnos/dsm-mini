package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestReusesValue: while the value is fresh, the source is left alone.
func TestReusesValue(t *testing.T) {
	var calls atomic.Int32
	c := New[int](time.Minute)
	load := func(context.Context) (int, error) {
		calls.Add(1)
		return 42, nil
	}

	for i := 0; i < 5; i++ {
		v, err := c.Get(context.Background(), load)
		if err != nil || v != 42 {
			t.Fatalf("got %v, %v", v, err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("source called %d times, expected one", calls.Load())
	}
}

// TestRefreshesAfterTTL: once the ttl is up, the value is refreshed.
func TestRefreshesAfterTTL(t *testing.T) {
	var calls atomic.Int32
	c := New[int](20 * time.Millisecond)
	load := func(context.Context) (int, error) {
		return int(calls.Add(1)), nil
	}

	first, _ := c.Get(context.Background(), load)
	time.Sleep(30 * time.Millisecond)
	second, _ := c.Get(context.Background(), load)

	if first == second {
		t.Error("value was not refreshed after the ttl expired")
	}
}

// TestSingleFlight: concurrent requests hit the source once.
//
// This is what the cache was built for: opening a screen in the app fires a
// burst of requests, and each of them must not become a call to the NAS.
func TestSingleFlight(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	c := New[int](time.Minute)

	load := func(context.Context) (int, error) {
		calls.Add(1)
		<-release
		return 7, nil
	}

	var wg sync.WaitGroup
	results := make([]int, 10)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _ = c.Get(context.Background(), load)
		}(i)
	}

	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()

	if calls.Load() != 1 {
		t.Errorf("source called %d times, expected one", calls.Load())
	}
	for i, v := range results {
		if v != 7 {
			t.Errorf("request %d got %d", i, v)
		}
	}
}

// TestCachesError: an error is cached too, otherwise an unreachable NAS makes
// every request wait out the timeout.
func TestCachesError(t *testing.T) {
	var calls atomic.Int32
	want := errors.New("NAS unreachable")
	c := New[int](time.Minute)
	load := func(context.Context) (int, error) {
		calls.Add(1)
		return 0, want
	}

	for i := 0; i < 3; i++ {
		if _, err := c.Get(context.Background(), load); !errors.Is(err, want) {
			t.Fatalf("expected an error, got %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("source called %d times, expected one", calls.Load())
	}
}

// TestInvalidate: after an action the cache is dropped and data is read anew.
func TestInvalidate(t *testing.T) {
	var calls atomic.Int32
	c := New[int](time.Minute)
	load := func(context.Context) (int, error) { return int(calls.Add(1)), nil }

	first, _ := c.Get(context.Background(), load)
	c.Invalidate()
	second, _ := c.Get(context.Background(), load)

	if first == second {
		t.Error("value stayed the same after invalidation")
	}
}

// TestGetStaleReturnsImmediately: a stale value is served at once while the
// refresh runs in the background. This is what the cache was reworked for:
// waiting for the NAS reads to the user as the screen hanging.
func TestGetStaleReturnsImmediately(t *testing.T) {
	var calls atomic.Int32
	slow := make(chan struct{})
	c := New[int](10 * time.Millisecond)

	load := func(context.Context) (int, error) {
		n := calls.Add(1)
		if n > 1 {
			<-slow // the second call hangs on purpose
		}
		return int(n), nil
	}

	// The first time we wait honestly: there is nothing to show yet.
	if v, _ := c.Get(context.Background(), load); v != 1 {
		t.Fatalf("first value %d", v)
	}
	time.Sleep(20 * time.Millisecond) // the value goes stale

	start := time.Now()
	v, err := c.GetStale(context.Background(), load)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if v != 1 {
		t.Errorf("got %d, expected the previous value 1", v)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("waited %v, should have answered immediately", elapsed)
	}

	close(slow)
	// Let the background refresh finish so it does not disturb other tests.
	time.Sleep(30 * time.Millisecond)
	if calls.Load() < 2 {
		t.Error("background refresh did not start")
	}
}

// TestGetStaleWaitsWhenEmpty: with no value at all we wait — there is nothing to show.
func TestGetStaleWaitsWhenEmpty(t *testing.T) {
	c := New[int](time.Minute)
	v, err := c.GetStale(context.Background(), func(context.Context) (int, error) {
		return 5, nil
	})
	if err != nil || v != 5 {
		t.Errorf("got %v, %v", v, err)
	}
}
