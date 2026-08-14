package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// ServiceHallService 网上办事大厅业务 API（service.scu.edu.cn）。
// 认证为 SSO 中继（CAS 回跳下发 PHPSESSID/vjuid/vjvd/vt 会话 cookie），
// 业务请求无 Authorization header，统一信封 {"e":0,"m":"...","d":...}。
type ServiceHallService struct {
	auth *auth.SsoRelayAuth
}

func NewServiceHallService(a *auth.SsoRelayAuth) *ServiceHallService { return &ServiceHallService{auth: a} }

const serviceBase = "https://service.scu.edu.cn"

// serviceJSONHeaders 办事大厅 AJAX JSON 请求头（与 Bugaoshan 一致）。
var serviceJSONHeaders = map[string]string{
	"Accept":           "application/json, text/plain, */*",
	"Content-Type":     "application/json;charset=UTF-8",
	"Origin":           serviceBase,
	"Referer":          serviceBase,
	"User-Agent":       auth.DefaultUserAgent,
	"X-Requested-With": "XMLHttpRequest",
}

func (s *ServiceHallService) request(fn func(*auth.CookieClient) (interface{}, error)) (interface{}, error) {
	return auth.RetryOnUnauthenticated(s.auth.GetClient, fn, s.auth.Invalidate)
}

// decodeServiceResponse 办事大厅统一信封解析：
// 302/401/403/空 body 或 e==10042（数字或字符串）为会话失效；e==0 为成功。
func decodeServiceResponse(body string, statusCode int) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(body)
	if statusCode == 302 || statusCode == 401 || statusCode == 403 || trimmed == "" {
		return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
	}
	if strings.HasPrefix(trimmed, "<") && strings.Contains(body, "login") {
		return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, &auth.ServiceError{Msg: "[service] 响应解析失败"}
	}
	switch e := out["e"].(type) {
	case string:
		if e == "10042" {
			return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
		}
	case float64:
		if int(e) == 10042 {
			return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
		}
	}
	return out, nil
}

// serviceSuccess 判定 e==0（数字或字符串 "0"）。
func serviceSuccess(json map[string]interface{}) bool {
	switch e := json["e"].(type) {
	case float64:
		return int(e) == 0
	case string:
		return e == "0"
	}
	return false
}

func serviceBusinessError(json map[string]interface{}, fallback string) error {
	if m, ok := json["m"].(string); ok && m != "" {
		return &auth.ServiceError{Msg: m}
	}
	return &auth.ServiceError{Msg: fallback}
}

// serviceData 取 d 字段并校验业务成功。
func serviceData(json map[string]interface{}, fallback string) (interface{}, error) {
	if !serviceSuccess(json) {
		return nil, serviceBusinessError(json, fallback)
	}
	d := json["d"]
	if d == nil {
		return nil, serviceBusinessError(json, fallback)
	}
	return d, nil
}

// ─── 我的申请 ────────────────────────────────────────────────────

// FetchMyApplications 查询我的申请列表。
// status 为筛选档位而非实例状态值：0=全部，1=进行中+草稿，3=已完成（与服务端一致）。
// 展示状态用列表项的 inst_status 中文字段；page 从 1 开始，每页固定 20 条。
// 查询参数 key 必须全部占位（keyword/time_lower/time_upper/y/task_name 传空串）。
func (s *ServiceHallService) FetchMyApplications(status, page int) ([]interface{}, error) {
	q := url.Values{
		"p":          {fmt.Sprint(page)},
		"page_size":  {"20"},
		"status":     {fmt.Sprint(status)},
		"keyword":    {""},
		"time_lower": {""},
		"time_upper": {""},
		"y":          {""},
		"task_name":  {""},
	}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(serviceBase+"/site/process/inst-list?"+q.Encode(), serviceJSONHeaders)
		if err != nil {
			return nil, err
		}
		json, err := decodeServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := serviceData(json, "获取我的申请失败")
		if err != nil {
			return nil, err
		}
		// 兼容 d.list 与 d 本身为列表两种形态。
		if dm, ok := d.(map[string]interface{}); ok {
			if list, ok := dm["list"].([]interface{}); ok {
				return list, nil
			}
		}
		if list, ok := d.([]interface{}); ok {
			return list, nil
		}
		return []interface{}{}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}
