package auth

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// SubsystemAuth 是子系统认证（L2）契约，对应 Bugaoshan 的 SubsystemAuth。
type SubsystemAuth interface {
	ModuleID() string
	// GetClient 返回已建立本子系统 session 的 CookieClient。
	GetClient() (*CookieClient, error)
	// Invalidate 清除本子系统缓存，下次 GetClient 重新建立 session。
	Invalidate()
}

// ─── 教务系统 ────────────────────────────────────────────────────

// ZhjwAuth 通过 SCU JWT SSO 获取 zhjw.scu.edu.cn 的 session cookie。
type ZhjwAuth struct {
	scu *ScuAuth

	mu        sync.Mutex
	cached    *CookieClient
	lastScuCl *CookieClient
}

func NewZhjwAuth(scu *ScuAuth) *ZhjwAuth { return &ZhjwAuth{scu: scu} }

func (z *ZhjwAuth) ModuleID() string { return "zhjw" }

func (z *ZhjwAuth) GetClient() (*CookieClient, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	scuClient, err := z.scu.GetClient()
	if err != nil {
		return nil, err
	}
	if scuClient != z.lastScuCl {
		// 根 client 身份变化：旧子系统 session cache 不可复用。
		z.lastScuCl = scuClient
		z.cached = nil
	}
	if z.cached != nil {
		return z.cached, nil
	}
	token, err := z.scu.GetAccessToken()
	if err != nil {
		return nil, err
	}
	resp, err := scuClient.FollowRedirects(
		"https://id.scu.edu.cn/enduser/sp/sso/scdxplugin_jwt23?enterpriseId=scdx&target_url=index",
		map[string]string{
			"Accept":        "text/html,application/xhtml+xml,*/*",
			"User-Agent":    DefaultUserAgent,
			"Authorization": "Bearer " + token,
		},
	)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, &UnauthenticatedError{Msg: "教务 SSO 登录已失效"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, &ServiceError{Msg: fmt.Sprintf("教务 SSO 登录失败(HTTP %d)", resp.StatusCode)}
	}
	z.cached = scuClient
	return scuClient, nil
}

func (z *ZhjwAuth) Invalidate() {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.cached = nil
	z.lastScuCl = nil
}

// ─── 微服务 ─────────────────────────────────────────────────────

// WfwAuth 通过统一认证 session 完成 wfw.scu.edu.cn 的 SSO 预热。
//
// 就绪判据必须是最终响应为 e==0 的 JSON——wfw 首页匿名访问也返回 200
// 并下发匿名 eai-sess cookie，不能作为就绪依据。
type WfwAuth struct {
	scu *ScuAuth

	mu        sync.Mutex
	ready     bool
	lastScuCl *CookieClient
}

func NewWfwAuth(scu *ScuAuth) *WfwAuth { return &WfwAuth{scu: scu} }

func (w *WfwAuth) ModuleID() string { return "wfw" }

func (w *WfwAuth) GetClient() (*CookieClient, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	client, err := w.scu.GetClient()
	if err != nil {
		return nil, err
	}
	if client != w.lastScuCl {
		w.lastScuCl = client
		w.ready = false
	}
	if w.ready {
		return client, nil
	}
	// 预热：不带 AJAX header 访问需登录的 get-info。匿名时 wfw 会
	// 302 → /uc/wap/login → /a_scu/api/cas/login → id.scu.edu.cn CAS
	// → 回跳 wfw，跟随该链建立绑定用户的 session cookie。
	resp, err := client.FollowRedirects(
		"https://wfw.scu.edu.cn/uc/wap/user/get-info",
		map[string]string{"User-Agent": DefaultUserAgent},
	)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, &UnauthenticatedError{Msg: "微服务登录已失效"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, &ServiceError{Msg: fmt.Sprintf("微服务预热失败(HTTP %d)", resp.StatusCode)}
	}
	var body struct {
		E int `json:"e"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil || body.E != 0 {
		// 链路落在 id.scu.edu.cn/login HTML 或 e!=0 都视为未绑定。
		return nil, &UnauthenticatedError{Msg: "微服务 session 未建立"}
	}
	w.ready = true
	return client, nil
}

func (w *WfwAuth) Invalidate() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ready = false
	w.lastScuCl = nil
}

// ─── SSO 中继（缴费平台 / 体测）─────────────────────────────────

// SsoRelayAuth 用于"通过 SCU SSO 跳转到子站"的场景，
// 子站通过 SCU Bearer token 获取自己的 cookie/session。
type SsoRelayAuth struct {
	scu    *ScuAuth
	ssoURL string
	module string
	deps   []SubsystemAuth

	mu        sync.Mutex
	cached    *CookieClient
	lastScuCl *CookieClient
}

func NewSsoRelayAuth(scu *ScuAuth, module, ssoURL string, deps ...SubsystemAuth) *SsoRelayAuth {
	return &SsoRelayAuth{scu: scu, module: module, ssoURL: ssoURL, deps: deps}
}

func (s *SsoRelayAuth) ModuleID() string { return s.module }

func (s *SsoRelayAuth) GetClient() (*CookieClient, error) {
	for _, dep := range s.deps {
		if _, err := dep.GetClient(); err != nil {
			return nil, fmt.Errorf("%s 依赖 %s 认证失败: %w", s.module, dep.ModuleID(), err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	scuClient, err := s.scu.GetClient()
	if err != nil {
		return nil, err
	}
	if scuClient != s.lastScuCl {
		s.lastScuCl = scuClient
		s.cached = nil
	}
	if s.cached != nil {
		return s.cached, nil
	}
	token, err := s.scu.GetAccessToken()
	if err != nil {
		return nil, err
	}
	resp, err := scuClient.FollowRedirects(s.ssoURL, map[string]string{
		"Accept":        "text/html,application/xhtml+xml,*/*",
		"User-Agent":    DefaultUserAgent,
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, &UnauthenticatedError{Msg: s.module + " SSO 中继已失效"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, &ServiceError{Msg: fmt.Sprintf("%s SSO 中继失败(HTTP %d)", s.module, resp.StatusCode)}
	}
	s.cached = scuClient
	return scuClient, nil
}

func (s *SsoRelayAuth) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cached = nil
	s.lastScuCl = nil
}

// NewPayAppAuth 缴费平台（依赖 WFW 先就绪）。
func NewPayAppAuth(scu *ScuAuth, wfw *WfwAuth) *SsoRelayAuth {
	return NewSsoRelayAuth(scu, "payapp", "https://payapp.scu.edu.cn/eleFees/oauth/airWarrant", wfw)
}

// NewFitnessAuth 体测系统。
func NewFitnessAuth(scu *ScuAuth) *SsoRelayAuth {
	return NewSsoRelayAuth(scu, "fitness", "https://pead.scu.edu.cn/bdlp_h5_fitness_test/public/index.php/index/login/scuMsLogin")
}

// NewServiceHallAuth 网上办事大厅（service.scu.edu.cn）。
// SSO 链：/api/login/main（301）→ /site/login/cas-login → id.scu.edu.cn CAS
// → 回跳下发 PHPSESSID(=CAS 票据)/vjuid/vjvd/vt 会话 cookie。
func NewServiceHallAuth(scu *ScuAuth) *SsoRelayAuth {
	return NewSsoRelayAuth(scu, "service",
		"https://service.scu.edu.cn/api/login/main?redirect_url=https%3A%2F%2Fservice.scu.edu.cn%2Fv2%2Fmatter%2F")
}

// ─── 通用重试 ───────────────────────────────────────────────────

// RetryOnUnauthenticated 对应 Bugaoshan 的 retryOnUnauthenticated：
// 认证失效时 invalidate 当前子系统、重新 GetClient，业务请求只重放一次。
func RetryOnUnauthenticated[T any](getClient func() (*CookieClient, error), fn func(*CookieClient) (T, error), invalidate func()) (T, error) {
	var zero T
	client, err := getClient()
	if err == nil {
		var result T
		result, err = fn(client)
		if err == nil {
			return result, nil
		}
	}
	if !IsUnauthenticated(err) {
		return zero, err
	}
	invalidate()
	client, err = getClient()
	if err != nil {
		return zero, err
	}
	return fn(client)
}

// LooksLikeLoginPage 判断响应是否为登录页（供各 API 的过期识别复用）。
func LooksLikeLoginPage(body string) bool {
	trimmed := strings.TrimLeft(body, " \t\r\n")
	return strings.HasPrefix(trimmed, "<") && strings.Contains(body, "login")
}
