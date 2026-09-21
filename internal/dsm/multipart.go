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

// UploadFile is a file sent in a multipart request.
type UploadFile struct {
	// Field is the part name. File Station expects "file", Download Station too.
	Field string
	Name  string
	Data  []byte
}

// CallUpload performs a multipart/form-data request (RFC 1867).
//
// The difference from an ordinary call: field values go AS THEY ARE, without
// JSON encoding, even for APIs with requestFormat "JSON" — that is what the
// Synology specification says and what works in practice. Binary data must be
// the last part of the request, otherwise DSM answers with error 1800.
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
		return fmt.Errorf("%s.%s: cannot parse the response: %w", api, method, err)
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

	// The order of the parts follows Synology's example: service fields, then
	// application ones, the file last.
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

	// The session id goes in the query string here rather than as a form part:
	// DSM does not read it from a multipart body and answers 119 "SID not found".
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
		return nil, fmt.Errorf("the upload to DSM failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("cannot read the DSM response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{Status: resp.StatusCode}
	}

	var out response
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("the DSM response is not JSON: %w", err)
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
