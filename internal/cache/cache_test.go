package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestReusesValue: пока значение свежее, источник не дёргается.
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
			t.Fatalf("получили %v, %v", v, err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("источник вызван %d раз, ожидался один", calls.Load())
	}
}

// TestRefreshesAfterTTL: по истечении срока значение обновляется.
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
		t.Error("значение не обновилось после истечения срока")
	}
}

// TestSingleFlight: одновременные запросы обращаются к источнику один раз.
//
// Ради этого кэш и заводился: при открытии экрана в приложении запросы идут
// пачкой, и каждый не должен превращаться в обращение к NAS.
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
		t.Errorf("источник вызван %d раз, ожидался один", calls.Load())
	}
	for i, v := range results {
		if v != 7 {
			t.Errorf("запрос %d получил %d", i, v)
		}
	}
}

// TestCachesError: ошибка тоже кэшируется, иначе недоступный NAS заставит
// каждый запрос ждать таймаута.
func TestCachesError(t *testing.T) {
	var calls atomic.Int32
	want := errors.New("NAS недоступен")
	c := New[int](time.Minute)
	load := func(context.Context) (int, error) {
		calls.Add(1)
		return 0, want
	}

	for i := 0; i < 3; i++ {
		if _, err := c.Get(context.Background(), load); !errors.Is(err, want) {
			t.Fatalf("ожидалась ошибка, получили %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("источник вызван %d раз, ожидался один", calls.Load())
	}
}

// TestInvalidate: после действия кэш сбрасывается и данные читаются заново.
func TestInvalidate(t *testing.T) {
	var calls atomic.Int32
	c := New[int](time.Minute)
	load := func(context.Context) (int, error) { return int(calls.Add(1)), nil }

	first, _ := c.Get(context.Background(), load)
	c.Invalidate()
	second, _ := c.Get(context.Background(), load)

	if first == second {
		t.Error("после сброса значение осталось прежним")
	}
}

// TestGetStaleReturnsImmediately: устаревшее значение отдаётся сразу, а
// обновление идёт в фоне. Ради этого кэш и переделывался: ожидание ответа
// NAS пользователь видит как подвисание экрана.
func TestGetStaleReturnsImmediately(t *testing.T) {
	var calls atomic.Int32
	slow := make(chan struct{})
	c := New[int](10 * time.Millisecond)

	load := func(context.Context) (int, error) {
		n := calls.Add(1)
		if n > 1 {
			<-slow // второй вызов намеренно висит
		}
		return int(n), nil
	}

	// Первый раз ждём по-честному: показывать ещё нечего.
	if v, _ := c.Get(context.Background(), load); v != 1 {
		t.Fatalf("первое значение %d", v)
	}
	time.Sleep(20 * time.Millisecond) // значение протухло

	start := time.Now()
	v, err := c.GetStale(context.Background(), load)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	if v != 1 {
		t.Errorf("вернулось %d, ожидалось прежнее значение 1", v)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("ждали %v, а должны были ответить сразу", elapsed)
	}

	close(slow)
	// Даём фоновому обновлению завершиться, чтобы не мешать другим тестам.
	time.Sleep(30 * time.Millisecond)
	if calls.Load() < 2 {
		t.Error("фоновое обновление не запустилось")
	}
}

// TestGetStaleWaitsWhenEmpty: без единого значения ждём, иначе показывать нечего.
func TestGetStaleWaitsWhenEmpty(t *testing.T) {
	c := New[int](time.Minute)
	v, err := c.GetStale(context.Background(), func(context.Context) (int, error) {
		return 5, nil
	})
	if err != nil || v != 5 {
		t.Errorf("получили %v, %v", v, err)
	}
}
