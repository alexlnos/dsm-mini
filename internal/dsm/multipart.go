package dsm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
)

// UploadFile — файл, передаваемый в multipart-запросе.
type UploadFile struct {
	// Field — имя части. File Station ждёт "file", Download Station — "file".
	Field string
	Name  string
	Data  []byte
}

// CallUpload выполняет запрос multipart/form-data (RFC 1867).
//
// Отличие от обычного вызова: значения полей идут КАК ЕСТЬ, без кодирования
// в JSON, даже у API с requestFormat "JSON" — так задано в спецификации
// Synology и так работает на деле. Бинарные данные обязаны быть последней
// частью запроса, иначе DSM отвечает ошибкой 1800.
func (c *Client) CallUpload(ctx context.Context, api, method string, version int,
	fields map[string]string, file UploadFile, out any) error {

	if err := c.ensureSession(ctx); err != nil {
		return err
	}

	data, err := c.upload(ctx, api, method, version, fields, file)
	var apiErr *APIError
	if err != nil && asAPIError(err, &apiErr) && apiErr.needsRelogin() {
		if err := c.Login(ctx); err != nil {
			return err
		}
		data, err = c.upload(ctx, api, method, version, fields, file)
	}
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s.%s: не разобрать ответ: %w", api, method, err)
	}
	return nil
}

func (c *Client) upload(ctx context.Context, api, method string, version int,
	fields map[string]string, file UploadFile) (json.RawMessage, error) {

	info, err := c.apiInfo(ctx, api)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	sid := c.sid
	c.mu.RUnlock()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	// Порядок частей повторяет пример Synology: служебные поля, потом
	// прикладные, файл — последним.
	ordered := []struct{ k, v string }{
		{"api", api},
		{"version", strconv.Itoa(version)},
		{"method", method},
	}
	for _, f := range ordered {
		if err := w.WriteField(f.k, f.v); err != nil {
			return nil, err
		}
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}

	if file.Data != nil {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
				escapeQuotes(file.Field), escapeQuotes(file.Name)))
		h.Set("Content-Type", "application/octet-stream")
		part, err := w.CreatePart(h)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(file.Data); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	// Идентификатор сессии здесь идёт в строке запроса, а не частью формы:
	// из тела multipart DSM его не читает и отвечает 119 «SID не найден».
	endpoint := c.baseURL + "/webapi/" + info.Path
	if sid != "" {
		endpoint += "?" + url.Values{"_sid": {sid}}.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("загрузка на DSM не удалась: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("не прочитать ответ DSM: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{Status: resp.StatusCode}
	}

	var out response
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("ответ DSM не является JSON: %w", err)
	}
	if !out.Success {
		e := &APIError{API: api, Method: method}
		if out.Error != nil {
			e.Code = out.Error.Code
			e.Failed = out.Error.Errors.FailedTask
		}
		return nil, e
	}
	return out.Data, nil
}

func escapeQuotes(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, `\"`).Replace(s)
}
