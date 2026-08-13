package auth

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
)

// DefaultUserAgent 与 Bugaoshan 一致，部分后端校验 UA。
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0"

// maxRedirects 是 SSO 手动重定向的最大跳数。
const maxRedirects = 10

// sensitiveHeaders 在跨源重定向时默认被剥离。
var sensitiveHeaders = []string{"Authorization", "Proxy-Authorization", "Cookie"}

// Response 是一次 HTTP 请求的结果。
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	// URL 是本次请求实际请求的 URL（重定向链中为当前跳）。
	URL *url.URL
}

// CookieClient 是 cookie 型认证的共享传输层。
//
//   - cookie 由 net/http cookiejar 按 RFC 6265 按域存储与发送；
//   - SSO 使用手动重定向，每一跳收集 Set-Cookie；
//   - 默认只在同源跳转中继续发送 Authorization 等敏感 header；
//   - 跨源转发敏感 header 必须通过 AllowSensitiveOrigin 明确允许；
//   - 重定向最多 10 跳。
type CookieClient struct {
	mu sync.Mutex
	rc *resty.Client
	jar *cookiejar.Jar

	sensitiveAllowedOrigins map[string]bool
}

// NewCookieClient 创建空的 CookieClient。
func NewCookieClient() (*CookieClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	c := &CookieClient{
		jar:                     jar,
		sensitiveAllowedOrigins: map[string]bool{},
	}
	c.rc = c.newRestyClient()
	return c, nil
}

func (c *CookieClient) newRestyClient() *resty.Client {
	rc := resty.New()
	rc.SetCookieJar(c.jar)
	rc.SetHeader("User-Agent", DefaultUserAgent)
	// 禁用自动重定向，改为手动逐跳跟随。
	rc.SetRedirectPolicy(resty.NoRedirectPolicy())
	return rc
}

// AllowSensitiveOrigin 允许跨源重定向时向该 origin 转发敏感 header。
func (c *CookieClient) AllowSensitiveOrigin(origin string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sensitiveAllowedOrigins[origin] = true
}

// SetCookies 手动写入 cookie（例如 session/save 后的统一认证 session）。
func (c *CookieClient) SetCookies(u *url.URL, cookies []*http.Cookie) {
	c.jar.SetCookies(u, cookies)
}

// Do 执行单次请求，不跟随重定向。传输层错误时重建底层 client 并重试一次。
func (c *CookieClient) Do(method, rawURL string, headers map[string]string, body io.Reader) (*Response, error) {
	resp, err := c.doOnce(method, rawURL, headers, body)
	if err != nil {
		// 传输层恢复：重建底层 client 后重发一次，第二次失败直接上抛。
		c.mu.Lock()
		c.rc = c.newRestyClient()
		c.mu.Unlock()
		if rb, ok := body.(interface{ Reset() }); ok {
			rb.Reset()
		}
		resp, err = c.doOnce(method, rawURL, headers, body)
	}
	return resp, err
}

func (c *CookieClient) doOnce(method, rawURL string, headers map[string]string, body io.Reader) (*Response, error) {
	c.mu.Lock()
	rc := c.rc
	c.mu.Unlock()

	req := rc.R()
	for k, v := range headers {
		req.SetHeader(k, v)
	}
	if body != nil {
		req.SetBody(body)
	}
	// resty 对 NoRedirectPolicy 返回的 3xx 不视为错误。
	restyResp, err := req.Execute(method, rawURL)
	if err != nil {
		return nil, err
	}
	return &Response{
		StatusCode: restyResp.StatusCode(),
		Header:     restyResp.Header(),
		Body:       restyResp.Body(),
		URL:        restyResp.Request.RawRequest.URL,
	}, nil
}

// Get 执行单次 GET。
func (c *CookieClient) Get(rawURL string, headers map[string]string) (*Response, error) {
	return c.Do(http.MethodGet, rawURL, headers, nil)
}

// PostForm 执行单次 application/x-www-form-urlencoded POST。
func (c *CookieClient) PostForm(rawURL string, headers map[string]string, form url.Values) (*Response, error) {
	h := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	for k, v := range headers {
		h[k] = v
	}
	return c.Do(http.MethodPost, rawURL, h, strings.NewReader(form.Encode()))
}

// FollowRedirects 从 rawURL 发起 GET 并手动跟随重定向，返回最终响应。
// 每一跳由 net/http cookiejar 自动收集 Set-Cookie、按域携带 Cookie；
// 跨源跳转剥离敏感 header（除非该 origin 被显式允许）。
func (c *CookieClient) FollowRedirects(rawURL string, headers map[string]string) (*Response, error) {
	current := rawURL
	currentHeaders := headers
	prevOrigin := ""
	for hop := 0; hop < maxRedirects; hop++ {
		u, err := url.Parse(current)
		if err != nil {
			return nil, &ServiceError{Msg: fmt.Sprintf("重定向 URL 非法: %v", err)}
		}
		origin := u.Scheme + "://" + u.Host
		if prevOrigin != "" && origin != prevOrigin {
			c.mu.Lock()
			allowed := c.sensitiveAllowedOrigins[origin]
			c.mu.Unlock()
			if !allowed {
				currentHeaders = stripHeaders(currentHeaders, sensitiveHeaders)
			}
		}
		resp, err := c.Get(current, currentHeaders)
		if err != nil {
			return nil, err
		}
		if !isRedirect(resp.StatusCode) {
			return resp, nil
		}
		loc := resp.Header.Get("Location")
		if loc == "" {
			return nil, &ServiceError{Msg: fmt.Sprintf("重定向缺失 Location (HTTP %d, %s)", resp.StatusCode, current)}
		}
		next, err := u.Parse(loc)
		if err != nil {
			return nil, &ServiceError{Msg: fmt.Sprintf("重定向 Location 非法: %q", loc)}
		}
		prevOrigin = origin
		current = next.String()
	}
	return nil, &ServiceError{Msg: fmt.Sprintf("重定向超过 %d 跳: %s", maxRedirects, rawURL)}
}

func isRedirect(code int) bool {
	switch code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func stripHeaders(headers map[string]string, names []string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = v
	}
	for _, n := range names {
		delete(out, n)
	}
	return out
}

// Cookies 返回指定 URL 应携带的 cookie（只读诊断用）。
func (c *CookieClient) Cookies(u *url.URL) []*http.Cookie {
	return c.jar.Cookies(u)
}
