package api

import (
	"encoding/json"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// WfwService 微服务业务 API：用户信息、标签、校园网设备。
type WfwService struct {
	auth *auth.WfwAuth
}

func NewWfwService(a *auth.WfwAuth) *WfwService { return &WfwService{auth: a} }

func (s *WfwService) request(fn func(*auth.CookieClient) (interface{}, error)) (interface{}, error) {
	return auth.RetryOnUnauthenticated(s.auth.GetClient, fn, s.auth.Invalidate)
}

const wfwBase = "https://wfw.scu.edu.cn"

// checkWfwExpiry 识别微服务会话过期：302/401/403、空 body、登录页、e==10013。
func checkWfwExpiry(body string, statusCode int) error {
	trimmed := strings.TrimSpace(body)
	if statusCode == 302 || statusCode == 401 || statusCode == 403 || trimmed == "" {
		return &auth.UnauthenticatedError{Msg: "微服务登录已失效"}
	}
	if strings.HasPrefix(trimmed, "<") && strings.Contains(body, "login") {
		return &auth.UnauthenticatedError{Msg: "微服务登录已失效"}
	}
	return nil
}

// decodeWfwResponse 过期检查后解析 JSON，e==10013 视为认证失效。
func decodeWfwResponse(body string, statusCode int) (map[string]interface{}, error) {
	if err := checkWfwExpiry(body, statusCode); err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, &auth.ServiceError{Msg: "[wfw] 响应解析失败"}
	}
	if e, ok := out["e"]; ok {
		if s, ok := e.(string); ok && s == "10013" {
			return nil, &auth.UnauthenticatedError{Msg: "微服务登录已失效"}
		}
		if n, ok := e.(float64); ok && int(n) == 10013 {
			return nil, &auth.UnauthenticatedError{Msg: "微服务登录已失效"}
		}
	}
	return out, nil
}

func wfwSuccess(json map[string]interface{}) bool {
	switch e := json["e"].(type) {
	case float64:
		return int(e) == 0
	case string:
		return e == "0"
	}
	return false
}

func wfwBusinessError(json map[string]interface{}, fallback string) error {
	if m, ok := json["m"].(string); ok && m != "" {
		return &auth.ServiceError{Msg: m}
	}
	return &auth.ServiceError{Msg: fallback}
}

// FetchUserProfile 获取用户基本信息（d.base 原始 JSON）。
func (s *WfwService) FetchUserProfile() (map[string]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(wfwBase+"/uc/wap/user/get-info", nil)
		if err != nil {
			return nil, err
		}
		json, err := decodeWfwResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		if !wfwSuccess(json) {
			return nil, wfwBusinessError(json, "获取用户信息失败")
		}
		d, _ := json["d"].(map[string]interface{})
		base, _ := d["base"].(map[string]interface{})
		return base, nil
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.(map[string]interface{}), nil
}

// FetchProfileLabels 获取用户信息标签（d.labels 原始 JSON）。
func (s *WfwService) FetchProfileLabels() ([]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(wfwBase+"/mashupapp/wap/real/user", map[string]string{
			"Accept":           "application/json, text/plain, */*",
			"X-Requested-With": "XMLHttpRequest",
			"Referer":          wfwBase,
		})
		if err != nil {
			return nil, err
		}
		json, err := decodeWfwResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		if !wfwSuccess(json) {
			return nil, wfwBusinessError(json, "获取用户标签失败")
		}
		d, _ := json["d"].(map[string]interface{})
		labels, _ := d["labels"].([]interface{})
		if labels == nil {
			labels = []interface{}{}
		}
		return labels, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}

var wfwNetworkHeaders = map[string]string{
	"Accept":           "application/json, text/plain, */*",
	"Content-Type":     "application/json; charset=UTF-8",
	"Origin":           wfwBase,
	"Referer":          wfwBase,
	"User-Agent":       auth.DefaultUserAgent,
	"X-Requested-With": "XMLHttpRequest",
}

// FetchNetworkDevices 获取校园网在线设备列表。
func (s *WfwService) FetchNetworkDevices() ([]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Do("POST", wfwBase+"/netclient/wap/default/get-index", wfwNetworkHeaders, nil)
		if err != nil {
			return nil, err
		}
		json, err := decodeWfwResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		if !wfwSuccess(json) {
			return nil, wfwBusinessError(json, "获取设备信息失败")
		}
		d, _ := json["d"].(map[string]interface{})
		list, _ := d["list"].([]interface{})
		if list == nil {
			list = []interface{}{}
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}
