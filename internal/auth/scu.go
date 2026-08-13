package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/config"
)

const (
	idBase        = "https://id.scu.edu.cn"
	idClientID    = "1371cbeda563697537f28d99b4744a973uDKtgYqL5B"
	idEnterprise  = "scdx"
	sessionTTLSec = 3600 // 本地 TTL，1 小时，与 Bugaoshan 一致
)

// ZhjwBase 是教务系统 base URL（该服务器不支持 HTTPS）。
const ZhjwBase = "http://zhjw.scu.edu.cn"

var idHeaders = map[string]string{
	"Accept":       "application/json, text/plain, */*",
	"Content-Type": "application/json;charset=UTF-8",
	"Origin":       idBase,
	"Referer":      idBase + "/frontend/login",
	"User-Agent":   DefaultUserAgent,
}

// Captcha 是统一认证验证码。
type Captcha struct {
	// Code 是验证码标识，登录时原样回传。
	Code string
	// ImageBase64 是验证码图片的 base64（可能带 data URL 前缀）。
	ImageBase64 string
}

// CaptchaSolver 在自动重新登录需要验证码时被调用。
// CLI 场景下通常保存图片并提示人工识别；无法识别时应返回 error。
type CaptchaSolver func(c *Captcha) (text string, err error)

// ScuAuth 是根认证（统一身份认证）的单一事实来源，对应 Bugaoshan 的 ScuAuth。
type ScuAuth struct {
	mu sync.Mutex

	creds *config.Credentials

	client *CookieClient // 已绑定 id.scu.edu.cn session 的 client

	// SolveCaptcha 用于 token 彻底失效后的自动重新登录；nil 表示不支持。
	SolveCaptcha CaptchaSolver
}

// NewScuAuth 加载持久化凭据并创建 ScuAuth。
func NewScuAuth() (*ScuAuth, error) {
	creds, err := config.LoadCredentials()
	if err != nil {
		return nil, err
	}
	return &ScuAuth{creds: creds}, nil
}

// Principal 返回当前账号（学号）。
func (a *ScuAuth) Principal() string { return a.creds.Principal }

// LoggedIn 报告是否存在可用 token（不代表未过期）。
func (a *ScuAuth) LoggedIn() bool { return a.creds.Token != "" }

// Expired 报告本地 TTL 是否已过期。
func (a *ScuAuth) Expired() bool {
	if a.creds.LoginTime.IsZero() {
		return true
	}
	return time.Since(a.creds.LoginTime) > sessionTTLSec*time.Second
}

// tokenFingerprint 校验持久化 token 与 principal 的绑定。
func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// FetchCaptcha 获取统一认证验证码图片。
func FetchCaptcha() (*Captcha, error) {
	client, err := NewCookieClient()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/public/bff/v1.2/one_time_login/captcha?_enterprise_id=%s&timestamp=%d",
		idBase, idEnterprise, time.Now().UnixMilli())
	resp, err := client.Get(url, idHeaders)
	if err != nil {
		return nil, &LoginError{Msg: fmt.Sprintf("验证码接口请求失败: %v", err)}
	}
	var body struct {
		Data struct {
			Captcha string `json:"captcha"`
			Code    string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, &LoginError{Msg: fmt.Sprintf("验证码接口返回非 JSON (HTTP %d)", resp.StatusCode)}
	}
	if body.Data.Captcha == "" || body.Data.Code == "" {
		return nil, &LoginError{Msg: "验证码字段解析失败"}
	}
	return &Captcha{Code: body.Data.Code, ImageBase64: body.Data.Captcha}, nil
}

// fetchSM2Key 获取 SM2 公钥（服务端偶发 500，重试 3 次）。
func fetchSM2Key(client *CookieClient) (publicKey, code string, err error) {
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}
		resp, reqErr := client.Do("POST", idBase+"/api/public/bff/v1.2/sm2_key", idHeaders, bytesReader([]byte("{}")))
		if reqErr != nil {
			continue
		}
		var body struct {
			Data struct {
				PublicKey string `json:"publicKey"`
				Code      string `json:"code"`
			} `json:"data"`
		}
		if json.Unmarshal(resp.Body, &body) == nil && body.Data.PublicKey != "" && body.Data.Code != "" {
			return body.Data.PublicKey, body.Data.Code, nil
		}
	}
	return "", "", &LoginError{Msg: "SM2 公钥接口返回异常"}
}

// Login 执行统一认证登录（SM2 加密密码 + 验证码），成功后持久化凭据。
func (a *ScuAuth) Login(username, password, captchaCode, captchaText string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	client, err := NewCookieClient()
	if err != nil {
		return err
	}
	publicKey, sm2Code, err := fetchSM2Key(client)
	if err != nil {
		return err
	}
	encrypted, err := SM2EncryptWithBase64Key(password, publicKey)
	if err != nil {
		return &LoginError{Msg: err.Error()}
	}

	payload, _ := json.Marshal(map[string]string{
		"client_id":      idClientID,
		"grant_type":     "password",
		"scope":          "read",
		"username":       username,
		"password":       encrypted,
		"_enterprise_id": idEnterprise,
		"sm2_code":       sm2Code,
		"cap_code":       captchaCode,
		"cap_text":       captchaText,
	})
	resp, err := client.Do("POST", idBase+"/api/public/bff/v1.2/rest_token", idHeaders, bytesReader(payload))
	if err != nil {
		return &LoginError{Msg: fmt.Sprintf("登录请求失败: %v", err)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &LoginError{Msg: fmt.Sprintf("登录请求失败(HTTP %d)", resp.StatusCode)}
	}
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Data    struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return &LoginError{Msg: "[rest_token] 响应解析失败"}
	}
	if !result.Success {
		msg := result.Message
		if msg == "" {
			msg = result.Msg
		}
		if msg == "" {
			msg = "登录失败"
		}
		return &LoginError{Msg: msg, InvalidCaptcha: msg == "invalid_captcha"}
	}
	if result.Data.AccessToken == "" {
		return &LoginError{Msg: "Token 字段缺失"}
	}

	a.creds.Token = result.Data.AccessToken
	a.creds.Principal = username
	a.creds.TokenFingerprint = tokenFingerprint(result.Data.AccessToken)
	a.creds.LoginTime = time.Now()
	// 保存账号密码用于过期自动重新登录（与 Bugaoshan 的自动登录对应）。
	a.creds.Username = username
	a.creds.Password = password
	a.client = nil
	if err := config.SaveCredentials(a.creds); err != nil {
		return err
	}
	return nil
}

// bindSession 调用 session/save，建立 id.scu.edu.cn cookie session。
// 调用方需持有并发保护（CLI 为单进程，直接用互斥锁）。
func (a *ScuAuth) bindSession() (*CookieClient, error) {
	if a.creds.Token == "" {
		return nil, &UnauthenticatedError{Msg: "未登录"}
	}
	if a.client != nil {
		return a.client, nil
	}
	client, err := NewCookieClient()
	if err != nil {
		return nil, err
	}
	headers := map[string]string{"Authorization": "Bearer " + a.creds.Token}
	for k, v := range idHeaders {
		headers[k] = v
	}
	resp, err := client.Do("POST", idBase+"/api/bff/v1.2/commons/session/save", headers, bytesReader([]byte("{}")))
	if err != nil {
		return nil, &ServiceError{Msg: fmt.Sprintf("session/save 请求失败: %v", err)}
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, &UnauthenticatedError{Msg: "统一认证 token 已失效"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ServiceError{Msg: fmt.Sprintf("session/save 请求失败(HTTP %d)", resp.StatusCode)}
	}
	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, &ServiceError{Msg: "[session/save] 响应解析失败"}
	}
	if strings.EqualFold(result.Error, "invalid_token") {
		return nil, &UnauthenticatedError{Msg: "统一认证 token 已失效"}
	}
	if !result.Success {
		return nil, &ServiceError{Msg: fmt.Sprintf("session/save 失败: %s", string(resp.Body))}
	}
	a.client = client
	return client, nil
}

// GetClient 返回已绑定统一认证 session 的 CookieClient。
//
// 恢复顺序（与 Bugaoshan 一致）：
//  1. 本地 TTL 未过期：bindSession（命中缓存则直接返回）；
//  2. 已过期或服务端拒绝：single-flight 刷新——先用现有 token 重新 bindSession，
//     失败则用保存的凭据 + CaptchaSolver 自动重新登录；
//  3. 全部失败：抛 UnauthenticatedError。
func (a *ScuAuth) GetClient() (*CookieClient, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.Expired() && a.creds.Token != "" {
		client, err := a.bindSession()
		if err == nil {
			return client, nil
		}
		if !IsUnauthenticated(err) {
			if _, ok := err.(*ServiceError); !ok {
				return nil, err
			}
		}
		// 认证失效或服务错误：进入刷新流程。
	}
	if err := a.refreshLocked(); err != nil {
		return nil, err
	}
	return a.bindSession()
}

// GetAccessToken 返回有效的 access token（供需要 Bearer 的子系统使用）。
func (a *ScuAuth) GetAccessToken() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.creds.Token == "" {
		return "", &UnauthenticatedError{Msg: "未登录"}
	}
	if a.Expired() {
		if err := a.refreshLocked(); err != nil {
			return "", err
		}
	}
	return a.creds.Token, nil
}

// refreshLocked 在持有锁的前提下刷新会话。
func (a *ScuAuth) refreshLocked() error {
	if a.creds.Token == "" {
		return &UnauthenticatedError{Msg: "未登录或登录已过期"}
	}
	// 1. 用现有 token 重新 bindSession（服务端 token 可能仍有效）。
	a.client = nil
	if _, err := a.bindSession(); err == nil {
		a.creds.LoginTime = time.Now()
		_ = config.SaveCredentials(a.creds)
		return nil
	}
	// 2. 用保存的凭据自动重新登录（需要验证码）。
	a.client = nil
	if a.creds.Username == "" || a.creds.Password == "" {
		return &UnauthenticatedError{Msg: "登录已过期，且没有保存的凭据，请重新 login"}
	}
	if a.SolveCaptcha == nil {
		return &UnauthenticatedError{Msg: "登录已过期，需要重新 login（需要验证码）"}
	}
	captcha, err := FetchCaptcha()
	if err != nil {
		return err
	}
	text, err := a.SolveCaptcha(captcha)
	if err != nil {
		return &UnauthenticatedError{Msg: fmt.Sprintf("登录已过期，自动重新登录失败: %v", err)}
	}
	// 注意：Login 会重新取锁，这里直接调用内部实现会死锁，
	// 因此临时解锁；CLI 为单线程调用，无并发风险。
	a.mu.Unlock()
	err = a.Login(a.creds.Username, a.creds.Password, captcha.Code, text)
	a.mu.Lock()
	if err != nil {
		return &UnauthenticatedError{Msg: fmt.Sprintf("自动重新登录失败: %v", err)}
	}
	return nil
}

// Logout 清除所有凭据与缓存。
func (a *ScuAuth) Logout() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.creds = &config.Credentials{}
	a.client = nil
	return config.ClearCredentials()
}

// bytesReader 包装字节切片为 io.Reader。
func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
