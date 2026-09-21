// Package dsm — клиент Synology DSM Web API.
//
// Три особенности DSM, из-за которых нельзя просто слать form-запросы:
//
//  1. Путь и допустимые версии каждого API узнаются в рантайме через
//     SYNO.API.Info — они отличаются между DSM 6 и 7 и между моделями NAS.
//  2. У части API requestFormat == "JSON": тогда КАЖДОЕ значение параметра
//     должно быть закодировано как JSON (строки — в кавычках). Иначе DSM
//     отвечает 101/105 без объяснений.
//  3. Ошибки приходят в теле ответа с HTTP 200, а сессия молча протухает —
//     нужен прозрачный повторный вход.
package dsm

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client — потокобезопасный клиент DSM Web API.
type Client struct {
	baseURL string
	user    string
	pass    string
	otp     string
	http    *http.Client
	log     *slog.Logger

	mu   sync.RWMutex
	sid  string
	apis map[string]apiInfo
	// goodVersion помнит версию, которой вызов уже удавался: на некоторых
	// сборках DSM старшая версия API сломана, и перебирать её каждый раз
	// значит платить лишним запросом.
	goodVersion map[string]int
}

type apiInfo struct {
	Path          string `json:"path"`
	MinVersion    int    `json:"minVersion"`
	MaxVersion    int    `json:"maxVersion"`
	RequestFormat string `json:"requestFormat"`
}

// Options — параметры подключения к DSM.
type Options struct {
	BaseURL     string
	User        string
	Password    string
	OTP         string
	InsecureTLS bool
	Timeout     time.Duration
	Logger      *slog.Logger
}

// New создаёт клиент. Сетевых запросов не делает — вход происходит лениво
// при первом вызове или явно через Login.
func New(o Options) *Client {
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if o.InsecureTLS {
		// У DSM из коробки самоподписанный сертификат. Проверку отключаем
		// только по явному согласию — см. DSM_INSECURE_TLS.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		baseURL:     strings.TrimRight(o.BaseURL, "/"),
		user:        o.User,
		pass:        o.Password,
		otp:         o.OTP,
		http:        &http.Client{Timeout: o.Timeout, Transport: transport},
		log:         o.Logger,
		apis:        make(map[string]apiInfo),
		goodVersion: make(map[string]int),
	}
}

type response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code   int `json:"code"`
		Errors struct {
			// DownloadStation2 складывает сюда задачи, не принявшие действие.
			FailedTask []ItemFailure `json:"failed_task"`
		} `json:"errors"`
	} `json:"error"`
}

// ItemFailure — отказ по одному элементу пакетной операции.
type ItemFailure struct {
	ID   string `json:"id"`
	Code int    `json:"error"`
}

// Login выполняет вход и запоминает SID.
func (c *Client) Login(ctx context.Context) error {
	params := map[string]any{
		"account": c.user,
		"passwd":  c.pass,
		"session": "DownloadStation",
		"format":  "sid",
	}
	if c.otp != "" {
		params["otp_code"] = c.otp
	}

	// Вход идёт до того, как известен requestFormat, — параметры шлём как есть.
	raw, err := c.post(ctx, "entry.cgi", buildForm("SYNO.API.Auth", "login", 7, "", params, false))
	if err != nil {
		return err
	}
	if !raw.Success {
		code := 0
		if raw.Error != nil {
			code = raw.Error.Code
		}
		msg, ok := authErrorText[code]
		if !ok {
			msg = fmt.Sprintf("код %d", code)
		}
		return fmt.Errorf("%w: %s", ErrAuth, msg)
	}

	var out struct {
		SID string `json:"sid"`
	}
	if err := json.Unmarshal(raw.Data, &out); err != nil {
		return fmt.Errorf("не разобрать ответ входа: %w", err)
	}

	c.mu.Lock()
	c.sid = out.SID
	c.mu.Unlock()
	c.log.Info("вход в DSM выполнен", "user", c.user)
	return nil
}

// Logout закрывает сессию. Ошибку можно игнорировать — сессия всё равно
// протухнет сама.
func (c *Client) Logout(ctx context.Context) error {
	c.mu.RLock()
	sid := c.sid
	c.mu.RUnlock()
	if sid == "" {
		return nil
	}
	_, err := c.post(ctx, "entry.cgi", buildForm("SYNO.API.Auth", "logout", 7,
		sid, map[string]any{"session": "DownloadStation"}, false))
	c.mu.Lock()
	c.sid = ""
	c.mu.Unlock()
	return err
}

// Call вызывает метод DSM API и раскладывает поле data в out.
//
// Сам выполняет вход, если сессии ещё нет, и повторяет запрос один раз,
// если DSM сообщил о протухшей сессии.
func (c *Client) Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error {
	if err := c.ensureSession(ctx); err != nil {
		return err
	}

	data, err := c.call(ctx, api, method, version, params)
	var apiErr *APIError
	if err != nil && asAPIError(err, &apiErr) && apiErr.needsRelogin() {
		c.log.Debug("сессия DSM недействительна, повторный вход", "api", api)
		if err := c.Login(ctx); err != nil {
			return err
		}
		data, err = c.call(ctx, api, method, version, params)
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

func (c *Client) call(ctx context.Context, api, method string, version int, params map[string]any) (json.RawMessage, error) {
	info, err := c.apiInfo(ctx, api)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	sid := c.sid
	c.mu.RUnlock()

	form := buildForm(api, method, version, sid, params, info.RequestFormat == "JSON")
	raw, err := c.post(ctx, info.Path, form)
	if err != nil {
		return nil, err
	}
	if !raw.Success {
		err := &APIError{API: api, Method: method}
		if raw.Error != nil {
			err.Code = raw.Error.Code
			err.Failed = raw.Error.Errors.FailedTask
		}
		return nil, err
	}
	return raw.Data, nil
}

// apiInfo возвращает путь и формат запроса для API, запрашивая их у NAS
// однократно и кэшируя.
func (c *Client) apiInfo(ctx context.Context, api string) (apiInfo, error) {
	c.mu.RLock()
	info, ok := c.apis[api]
	c.mu.RUnlock()
	if ok {
		return info, nil
	}

	raw, err := c.post(ctx, "query.cgi", buildForm("SYNO.API.Info", "query", 1, "",
		map[string]any{"query": api}, false))
	if err != nil {
		return apiInfo{}, err
	}
	if !raw.Success {
		return apiInfo{}, fmt.Errorf("не получить сведения об API %s", api)
	}

	var all map[string]apiInfo
	if err := json.Unmarshal(raw.Data, &all); err != nil {
		return apiInfo{}, fmt.Errorf("не разобрать SYNO.API.Info: %w", err)
	}
	info, ok = all[api]
	if !ok {
		return apiInfo{}, fmt.Errorf("API %s на этом NAS отсутствует", api)
	}

	c.mu.Lock()
	c.apis[api] = info
	c.mu.Unlock()
	return info, nil
}

// HasAPI сообщает, есть ли такой API на этом NAS. Нужно, чтобы выбирать между
// современным SYNO.DownloadStation2 и легаси SYNO.DownloadStation.
func (c *Client) HasAPI(ctx context.Context, api string) bool {
	_, err := c.apiInfo(ctx, api)
	return err == nil
}

// APIMaxVersion возвращает максимальную поддерживаемую версию API, 0 — если
// API нет.
func (c *Client) APIMaxVersion(ctx context.Context, api string) int {
	info, err := c.apiInfo(ctx, api)
	if err != nil {
		return 0
	}
	return info.MaxVersion
}

func (c *Client) ensureSession(ctx context.Context) error {
	c.mu.RLock()
	sid := c.sid
	c.mu.RUnlock()
	if sid != "" {
		return nil
	}
	return c.Login(ctx)
}

func (c *Client) post(ctx context.Context, path string, form url.Values) (*response, error) {
	endpoint := c.baseURL + "/webapi/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("запрос к DSM не удался: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("не прочитать ответ DSM: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{Status: resp.StatusCode}
	}

	var out response
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("ответ DSM не является JSON: %w", err)
	}
	return &out, nil
}

// buildForm собирает тело запроса.
//
// При jsonFormat каждое значение кодируется как JSON: строка "url" уходит как
// "\"url\"". Это требование API с requestFormat == "JSON" (все entry.cgi
// DownloadStation2 и Virtualization), и именно на нём чаще всего спотыкаются.
func buildForm(api, method string, version int, sid string, params map[string]any, jsonFormat bool) url.Values {
	form := url.Values{}
	form.Set("api", api)
	form.Set("method", method)
	form.Set("version", strconv.Itoa(version))
	if sid != "" {
		form.Set("_sid", sid)
	}
	for k, v := range params {
		if jsonFormat {
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			form.Set(k, string(b))
			continue
		}
		switch t := v.(type) {
		case string:
			form.Set(k, t)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			form.Set(k, string(b))
		}
	}
	return form
}

func asAPIError(err error, target **APIError) bool {
	for err != nil {
		if e, ok := err.(*APIError); ok {
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
