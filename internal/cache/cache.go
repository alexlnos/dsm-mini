// Package cache — небольшой кэш значений с временем жизни.
//
// Нужен, чтобы частые опросы с телефона не превращались в шквал запросов к
// NAS: сведения об устройстве меняются раз в месяц, список пакетов — раз в
// неделю, и запрашивать их при каждом обновлении экрана незачем.
package cache

import (
	"context"
	"sync"
	"time"
)

type entry[T any] struct {
	value   T
	expires time.Time
	// err запоминается вместе со значением: иначе при недоступном NAS
	// каждый запрос снова ждал бы таймаута.
	err error
}

// Cache хранит одно значение, обновляя его не чаще, чем раз в ttl.
type Cache[T any] struct {
	ttl time.Duration

	mu      sync.Mutex
	current *entry[T]
	// loading не даёт нескольким одновременным запросам дёргать NAS разом:
	// первый идёт за данными, остальные ждут его результата.
	loading chan struct{}
}

// New создаёт кэш с заданным временем жизни.
func New[T any](ttl time.Duration) *Cache[T] {
	return &Cache[T]{ttl: ttl}
}

// Get возвращает значение из кэша или вычисляет его через load.
func (c *Cache[T]) Get(ctx context.Context, load func(context.Context) (T, error)) (T, error) {
	for {
		c.mu.Lock()

		if c.current != nil && time.Now().Before(c.current.expires) {
			value, err := c.current.value, c.current.err
			c.mu.Unlock()
			return value, err
		}

		if c.loading != nil {
			// Кто-то уже пошёл за данными — ждём его и смотрим результат.
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

// Invalidate сбрасывает кэш: нужен после действий, меняющих состояние.
func (c *Cache[T]) Invalidate() {
	c.mu.Lock()
	c.current = nil
	c.mu.Unlock()
}

// GetStale возвращает последнее известное значение немедленно, а обновление
// запускает в фоне.
//
// Для экрана это важнее свежести до секунды: показатели загрузки и список
// задач всё равно обновляются на следующем опросе, а ожидание ответа NAS
// пользователь видит как подвисание. Пока значения ещё нет, ведёт себя как
// обычный Get и ждёт.
func (c *Cache[T]) GetStale(ctx context.Context, load func(context.Context) (T, error)) (T, error) {
	c.mu.Lock()
	current := c.current
	fresh := current != nil && time.Now().Before(current.expires)
	alreadyLoading := c.loading != nil
	c.mu.Unlock()

	if current == nil {
		// Первый запрос: показывать нечего, придётся подождать.
		return c.Get(ctx, load)
	}
	if !fresh && !alreadyLoading {
		// Обновляем в фоне: контекст запроса для этого не годится — он
		// завершится вместе с ответом, оборвав загрузку на середине.
		go func() {
			background, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, _ = c.Get(background, load)
		}()
	}
	return current.value, current.err
}
