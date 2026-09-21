package dsm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Content — бинарный ответ DSM: миниатюра, файл, архив.
type Content struct {
	Body        io.ReadCloser
	ContentType string
	// Length — размер, если DSM его сообщил; иначе -1.
	Length int64
	// Filename — имя из Content-Disposition, если было.
	Filename string
}

// Fetch выполняет GET и возвращает тело ответа, не разбирая его как JSON.
//
// Нужен для API, отдающих файлы: SYNO.FileStation.Thumb и
// SYNO.FileStation.Download. Ошибку DSM в таких ответах видно по типу
// содержимого: вместо картинки приходит JSON с полем error.
//
// Тело закрывает вызывающий.
func (c *Client) Fetch(ctx context.Context, api, method string, version int,
	params map[string]any) (*Content, error) {

	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}

	content, err := c.fetch(ctx, api, method, version, params)
	var apiErr *APIError
	if err != nil && asAPIError(err, &apiErr) && apiErr.needsRelogin() {
		if err := c.Login(ctx); err != nil {
			return nil, err
		}
		content, err = c.fetch(ctx, api, method, version, params)
	}
	return content, err
}

func (c *Client) fetch(ctx context.Context, api, method string, version int,
	params map[string]any) (*Content, error) {

	info, err := c.apiInfo(ctx, api)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	sid := c.sid
	c.mu.RUnlock()

	query := url.Values{}
	query.Set("api", api)
	query.Set("method", method)
	query.Set("version", strconv.Itoa(version))
	query.Set("_sid", sid)
	for k, v := range params {
		// У этих API requestFormat — JSON, поэтому значения кодируются так же,
		// как в обычных вызовах: строки в кавычках, списки в скобках.
		b, err := json.Marshal(v)
		if err != nil {
			continue
		}
		query.Set(k, string(b))
	}

	endpoint := c.baseURL + "/webapi/" + info.Path + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("запрос к DSM не удался: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &HTTPError{Status: resp.StatusCode}
	}

	ct := resp.Header.Get("Content-Type")
	// DSM сообщает об отказе тем же каналом, но уже в виде JSON.
	if strings.Contains(ct, "application/json") || strings.Contains(ct, "text/json") {
		defer resp.Body.Close()
		var out response
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
			return nil, fmt.Errorf("%s.%s: не разобрать ответ", api, method)
		}
		e := &APIError{API: api, Method: method}
		if out.Error != nil {
			e.Code = out.Error.Code
		}
		return nil, e
	}

	return &Content{
		Body:        resp.Body,
		ContentType: ct,
		Length:      resp.ContentLength,
		Filename:    filenameFrom(resp.Header.Get("Content-Disposition")),
	}, nil
}

func filenameFrom(disposition string) string {
	for _, part := range strings.Split(disposition, ";") {
		part = strings.TrimSpace(part)
		if rest, ok := strings.CutPrefix(part, "filename="); ok {
			return strings.Trim(rest, `"`)
		}
	}
	return ""
}
