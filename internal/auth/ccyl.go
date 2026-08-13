package auth

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/config"
)

// CcylSpCode 是第二课堂的 sp_code 常量（与 Bugaoshan 一致）。
const CcylSpCode = "bDBhREE1WDMzK3llSzZyVFZNeE81czRDd1hESTI4NWxGaFdsTnlvcGt3eVdTb2cxSjN5a1FJTDVMWTBEQkFFd2k1bWZRMy82OXN6V21ZYzFLd2NlSDdUaWlVcVJ1emxVVnF4Q3RZNWxjWlVoTEZqUktVSWVmY1ZaKzBLYUlBWDYvaU5MS1E5Y25nT1BoSzRIM0FIOWVCQjMxMXd5b0JrenNuWDBDM1BKU0FwUVVnZHdoSWYrc0hKZmEwSHRQbFZDV1o2dzFtQ3Nuci9wV1ExZHRMMytueHpLZVg5djJJcGFRbkJxZFJCQWJZWHI2dlpQNHVxNFNhcHM3Y3RkK2g1dWFuUEtNT1JZblFXRFBLUEdrcGdxNHR5eEcxclh5YXQ5a2FXN3JSZ2g2OTAxWCt0TUdTNXJDRVdNeDNTU3duTk1nNW9RSyt4WkdzSjNkR3NvVEFDMzFCQmJHUVcrVitybmszQVd0djFpUUJ5dDJySlRTajZIem1qZFYwMjVWcVpEaUtKd1AwQzI3TUpZd3FyY1hqdkxUZkFCd3JwL3ltczdXcmlTUzhZYVJPR0QwOXk2aDJIdUlCUTAvbEJWd0xzcUZXSElxaENpR0pseG1XYTZRbWlFaklERTd6TlhBQkJLdTZGUS8rNTBBYWRkcDVrRXdBM0tqejMvd1AvTklkZW5oNll4MllINlFiNVRucXNhZWtzUlh3d1BOQzBrMERSM0tId3dyS1hONkF6VDZwRGl3S3h1aDNLSGVmcTBRTktXUXMxTTZxeW1lcmgzYVlGWDNmVHdvUnJkWXVhbHN0aEtHKzU5TnFuVm1NbXU4dnhZQk8zKzQrdnV3aTJEaGY4VXRnV3lHeTVBcFFnWlUyQTFsWjdsR1RyNHh1TjV5dUlVc1VNNTRlbEtETTVVYWZoYnFPTXFrM2MxUHVNSHVHLzRtUFk4cmZzaXNUVkovWlhuSkhWWXpYQUJ4UDE4bGt2NXJkMFlXZHM0cFlYVVduKy9ZWGNKTlBDNEVrSzE3R0NVWDNxcCtiQkVyaXMzaTRXam1wWTFzYkpWZTAxYzZ0VGlxcGkvcEYyLzJPND0="

const ccylAPIBase = "https://dekt.scu.edu.cn/ccyl-api"

var ccylBaseHeaders = map[string]string{
	"Accept":       "application/json, text/plain, */*",
	"Content-Type": "application/json;charset=UTF-8",
	"Origin":       "https://dekt.scu.edu.cn",
	"Referer":      "https://dekt.scu.edu.cn",
	"User-Agent":   DefaultUserAgent,
}

// CcylAuth 第二课堂认证（L2）。
//
// CCYL 拥有独立 OAuth token 体系：token 由当前 SCU 账号授权得到，
// 与 SCU principal 绑定持久化，不能跨账号恢复。
type CcylAuth struct {
	scu *ScuAuth

	mu sync.Mutex
}

func NewCcylAuth(scu *ScuAuth) *CcylAuth { return &CcylAuth{scu: scu} }

func (c *CcylAuth) ModuleID() string { return "ccyl" }

// Token 返回绑定到当前 SCU 账号的 CCYL token；未登录或不属于当前账号返回空。
func (c *CcylAuth) Token() string {
	creds := c.scu.creds
	if creds.CcylToken == "" || creds.CcylPrincipal == "" || creds.CcylPrincipal != creds.Principal {
		return ""
	}
	return creds.CcylToken
}

// UserID 返回 CCYL 用户 ID（同 Token 的绑定约束）。
func (c *CcylAuth) UserID() string {
	creds := c.scu.creds
	if c.Token() == "" {
		return ""
	}
	return creds.CcylUserID
}

// IsLoggedIn 报告 CCYL 是否已登录到当前 SCU 账号。
func (c *CcylAuth) IsLoggedIn() bool { return c.Token() != "" }

// EnsureAuthenticated 确保已登录：已登录直接返回，否则执行 OAuth 重登录。
func (c *CcylAuth) EnsureAuthenticated() error {
	if c.IsLoggedIn() {
		return nil
	}
	return c.reLogin()
}

// RecoverExpiredSession 处理业务返回的 token 过期：清 token 并重登录一次。
func (c *CcylAuth) RecoverExpiredSession() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearSessionLocked()
	return c.reLoginLocked()
}

// Invalidate 清除 CCYL 会话（内存与持久化）。
func (c *CcylAuth) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearSessionLocked()
	_ = config.SaveCredentials(c.scu.creds)
}

func (c *CcylAuth) clearSessionLocked() {
	c.scu.creds.CcylToken = ""
	c.scu.creds.CcylUserID = ""
	c.scu.creds.CcylPrincipal = ""
}

func (c *CcylAuth) reLogin() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reLoginLocked()
}

// reLoginLocked 通过 SCU OAuth 静默重登录（调用方需持有锁）。
func (c *CcylAuth) reLoginLocked() error {
	principal := c.scu.creds.Principal
	if principal == "" {
		return &UnauthenticatedError{Msg: "无法确认当前校园账号，请重新 login"}
	}
	code, err := c.getOAuthCode()
	if err != nil {
		return err
	}
	// OAuth 完成后账号仍须一致（CLI 单线程，防御性校验）。
	if c.scu.creds.Principal != principal {
		return &UnauthenticatedError{Msg: "校园账号已切换，请重新授权"}
	}
	token, userID, err := ccylLoginByCode(code)
	if err != nil {
		return err
	}
	c.scu.creds.CcylToken = token
	c.scu.creds.CcylUserID = userID
	c.scu.creds.CcylPrincipal = principal
	return config.SaveCredentials(c.scu.creds)
}

var (
	ccylMetaRefreshRe = regexp.MustCompile(`(?i)<meta[^>]+http-equiv=["']refresh["'][^>]+content=["'][^;]+;\s*url=([^"'>\s]+)`)
	ccylJSLocationRe  = regexp.MustCompile(`window\.location(?:\.href)?\s*=\s*["']([^"']+)["']`)
)

// getOAuthCode 从统一认证 sp_logged 链获取 OAuth code。
func (c *CcylAuth) getOAuthCode() (string, error) {
	token, err := c.scu.GetAccessToken()
	if err != nil {
		return "", err
	}
	client, err := c.scu.GetClient()
	if err != nil {
		return "", err
	}
	spLoggedURL := fmt.Sprintf(
		"%s/api/bff/v1.2/commons/sp_logged?access_token=%s&sp_code=%s&application_key=scdxplugin_cas_apereo17",
		idBase, url.QueryEscape(token), url.QueryEscape(CcylSpCode),
	)
	resp, err := client.FollowRedirects(spLoggedURL, map[string]string{
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*",
		"User-Agent": DefaultUserAgent,
	})
	if err != nil {
		return "", &ServiceError{Msg: fmt.Sprintf("第二课堂 OAuth 跳转失败: %v", err)}
	}
	// 主路径：最终 URL 携带 code 参数。
	if resp.URL != nil {
		if code := resp.URL.Query().Get("code"); code != "" {
			return code, nil
		}
	}
	// 兜底：从 body 的 meta refresh / window.location 中提取。
	body := string(resp.Body)
	for _, re := range []*regexp.Regexp{ccylMetaRefreshRe, ccylJSLocationRe} {
		if m := re.FindStringSubmatch(body); m != nil {
			if u, err := url.Parse(m[1]); err == nil {
				if code := u.Query().Get("code"); code != "" {
					return code, nil
				}
			}
		}
	}
	return "", &UnauthenticatedError{Msg: "第二课堂 OAuth 未获得 code（统一认证可能已失效）"}
}

// ccylLoginByCode 用 OAuth code 换取 CCYL token（loginByUc）。
func ccylLoginByCode(code string) (token, userID string, err error) {
	client, err := NewCookieClient()
	if err != nil {
		return "", "", err
	}
	payload, _ := json.Marshal(map[string]string{"code": code})
	resp, err := client.Do("POST", ccylAPIBase+"/app/auth/loginByUc", ccylBaseHeaders, strings.NewReader(string(payload)))
	if err != nil {
		return "", "", &ServiceError{Msg: fmt.Sprintf("第二课堂登录请求失败: %v", err)}
	}
	var out struct {
		Code  int    `json:"code"`
		Msg   string `json:"msg"`
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return "", "", &ServiceError{Msg: "[loginByUc] 响应解析失败"}
	}
	if out.Code != 0 {
		return "", "", &ServiceError{Msg: "第二课堂登录失败: " + out.Msg}
	}
	if out.Token == "" {
		return "", "", &ServiceError{Msg: "第二课堂 Token 字段缺失"}
	}
	return out.Token, out.User.ID, nil
}

// IsCcylAuthExpiredCode 判定 CCYL 业务码是否表示 token 过期。
func IsCcylAuthExpiredCode(code interface{}) bool {
	switch v := code.(type) {
	case string:
		return v == "401"
	case float64:
		return int(v) == 401
	}
	return false
}
