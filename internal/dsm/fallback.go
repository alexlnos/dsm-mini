package dsm

import (
	"context"
	"fmt"
	"net/http"
)

// HTTPError — DSM ответил кодом, отличным от 200.
//
// Отдельный тип нужен, потому что сломанный обработчик API отдаёт 502, и это
// единственный признак, по которому можно решить понизить версию вызова.
type HTTPError struct {
	Status int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("DSM ответил HTTP %d", e.Status)
}

// retryLowerVersion сообщает, есть ли смысл повторить вызов версией ниже.
//
// Поводов два:
//
//   - HTTP 5xx: на части сборок обработчик конкретной версии API падает.
//     Наблюдалось на File Station 1.4.4-2221, где SYNO.FileStation.List
//     версии 2 отвечает 502 при любых параметрах, а версия 1 работает.
//   - Коды 103 и 104: метода или версии на этом NAS нет.
func retryLowerVersion(err error) bool {
	var httpErr *HTTPError
	if asHTTPError(err, &httpErr) {
		return httpErr.Status >= http.StatusInternalServerError
	}
	var apiErr *APIError
	if asAPIError(err, &apiErr) {
		return apiErr.Code == 103 || apiErr.Code == 104
	}
	return false
}

// CallVersioned вызывает метод, перебирая версии от первой к последней, пока
// одна не сработает. Версию, которая сработала, запоминает и в следующий раз
// начинает с неё.
//
// Порядок versions задаёт вызывающий: обычно от новой к старой.
func (c *Client) CallVersioned(ctx context.Context, api, method string, versions []int,
	params map[string]any, out any) error {

	if len(versions) == 0 {
		return fmt.Errorf("%s.%s: не указано ни одной версии", api, method)
	}

	key := api + "." + method
	c.mu.RLock()
	known, ok := c.goodVersion[key]
	c.mu.RUnlock()
	if ok {
		versions = append([]int{known}, without(versions, known)...)
	}

	var lastErr error
	for _, v := range versions {
		err := c.Call(ctx, api, method, v, params, out)
		if err == nil {
			c.mu.Lock()
			c.goodVersion[key] = v
			c.mu.Unlock()
			return nil
		}
		lastErr = err
		if !retryLowerVersion(err) {
			return err
		}
		c.log.Debug("версия API не сработала, пробуем ниже",
			"api", api, "method", method, "version", v, "err", err)
	}
	return lastErr
}

func without(xs []int, v int) []int {
	out := make([]int, 0, len(xs))
	for _, x := range xs {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func asHTTPError(err error, target **HTTPError) bool {
	for err != nil {
		if e, ok := err.(*HTTPError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
