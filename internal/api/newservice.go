package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// NewServiceService 智慧线上服务平台（后勤 newservice）业务 API，
// 移植自 Bugaoshan new_service_api_service.dart。
//
// service.scu.edu.cn/newservice 的无感认证（passpoint）接口。
// 认证为 SSO 中继（会话 cookie process_uid / process_number）。
//
// 响应约定：与教务/微服务的 e 数字错误码不同，newservice 业务成功码是
// 字符串 e == "OK"；d 内的 errorCode/errorMessage 才是具体业务结果
// （0 成功，非 0 携带错误信息）。
type NewServiceService struct {
	auth *auth.SsoRelayAuth
}

func NewNewServiceService(a *auth.SsoRelayAuth) *NewServiceService {
	return &NewServiceService{auth: a}
}

const (
	newserviceBase = "https://service.scu.edu.cn"
	newservicePath = "/newservice"
)

func (s *NewServiceService) request(fn func(*auth.CookieClient) (interface{}, error)) (interface{}, error) {
	return auth.RetryOnUnauthenticated(s.auth.GetClient, fn, s.auth.Invalidate)
}

var newserviceHeaders = map[string]string{
	"Accept":           "application/json, text/plain, */*",
	"X-Requested-With": "XMLHttpRequest",
	"Origin":           newserviceBase,
	"Referer":          newserviceBase + newservicePath + "/fe/site/m_passpoint",
	"User-Agent":       auth.DefaultUserAgent,
}

// decodeNewServiceResponse newservice 信封解析：
// 302/401/403/空 body 或登录页强特征为会话失效；e=="OK" 为成功；
// e=="UN_AUTH" 也为会话失效。
func decodeNewServiceResponse(body string, statusCode int) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(body)
	if statusCode == 302 || statusCode == 401 || statusCode == 403 || trimmed == "" {
		return nil, &auth.UnauthenticatedError{Msg: "newservice 会话已失效"}
	}
	if auth.LooksLikeLoginPage(body) {
		return nil, &auth.UnauthenticatedError{Msg: "newservice 会话已失效"}
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, &auth.ServiceError{Msg: "[newservice] 响应解析失败"}
	}
	if e := zhhqFieldString(out["e"]); e != "OK" {
		if e == "UN_AUTH" {
			return nil, &auth.UnauthenticatedError{Msg: "newservice 登录已失效"}
		}
		msg := zhhqFieldString(out["m"])
		if msg == "" {
			msg = "服务暂不可用"
		}
		return nil, &auth.ServiceError{Msg: msg}
	}
	return out, nil
}

// checkNewServiceData 校验 d 内的业务错误码（0 成功），返回 d。
func checkNewServiceData(out map[string]interface{}) (map[string]interface{}, error) {
	d, _ := out["d"].(map[string]interface{})
	if d == nil {
		return nil, &auth.ServiceError{Msg: "[newservice] 响应缺少 d 字段"}
	}
	if code := zhhqFieldString(d["errorCode"]); code != "" && code != "0" {
		msg := zhhqFieldString(d["errorMessage"])
		if msg == "" {
			msg = "操作失败"
		}
		return nil, &auth.ServiceError{Msg: msg}
	}
	return d, nil
}

// ─── 模型（移植自 Bugaoshan passpoint.dart） ──────────────────────

// PasspointDevice 无感设备列表项（query-user-mab-info 的 d.data[] 元素）。
// macExpireTime 是到期日期字符串（YYYY-MM-DD），并非天数；
// 空/无法解析（对应"最长有效期 6 年"）时 ExpireTime 为空串。
type PasspointDevice struct {
	UserMac            string `json:"user_mac"`
	MacExpireTime      string `json:"mac_expire_time,omitempty"`
	DefaultServiceName string `json:"default_service_name"`
	IsOnline           bool   `json:"is_online"`
}

func parsePasspointDevice(m map[string]interface{}) PasspointDevice {
	raw := zhhqFieldString(m["macExpireTime"])
	if raw != "" {
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			// 完整时间戳形态也接受，输出统一为日期。
			if t, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
				raw = t.Format("2006-01-02")
			} else {
				raw = ""
			}
		}
	}
	return PasspointDevice{
		UserMac:            zhhqFieldString(m["userMac"]),
		MacExpireTime:      raw,
		DefaultServiceName: zhhqFieldString(m["defaultServiceName"]),
		IsOnline:           m["isOnline"] == true || zhhqFieldString(m["isOnline"]) == "1" || zhhqFieldString(m["isOnline"]) == "true",
	}
}

// PasspointUserInfo 校园网账户信息（query-user 的 d.queryUserResult.data）。
type PasspointUserInfo struct {
	UserID        string `json:"user_id"`
	UserName      string `json:"user_name"`
	UserGroupName string `json:"user_group_name"`
	AccountState  int    `json:"account_state"`
	Mobile        string `json:"mobile,omitempty"`
	Email         string `json:"email,omitempty"`
}

// ─── 端点 ─────────────────────────────────────────────────────────

// FetchPasspointDevices 获取无感设备列表。
func (s *NewServiceService) FetchPasspointDevices() ([]PasspointDevice, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		u := newserviceBase + newservicePath + "/site/passpoint/query-user-mab-info?limit=100"
		resp, err := c.Get(u, newserviceHeaders)
		if err != nil {
			return nil, err
		}
		out, err := decodeNewServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := checkNewServiceData(out)
		if err != nil {
			return nil, err
		}
		data, _ := d["data"].([]interface{})
		list := make([]PasspointDevice, 0, len(data))
		for _, e := range data {
			if m, ok := e.(map[string]interface{}); ok {
				list = append(list, parsePasspointDevice(m))
			}
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]PasspointDevice), nil
}

// FetchPasspointUserInfo 获取校园网账户信息。
func (s *NewServiceService) FetchPasspointUserInfo() (*PasspointUserInfo, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		u := newserviceBase + newservicePath + "/site/passpoint/query-user"
		resp, err := c.Get(u, newserviceHeaders)
		if err != nil {
			return nil, err
		}
		out, err := decodeNewServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := checkNewServiceData(out)
		if err != nil {
			return nil, err
		}
		result, _ := d["queryUserResult"].(map[string]interface{})
		data, _ := result["data"].(map[string]interface{})
		if data == nil {
			return (*PasspointUserInfo)(nil), nil
		}
		return &PasspointUserInfo{
			UserID:        zhhqFieldString(data["userId"]),
			UserName:      zhhqFieldString(data["userName"]),
			UserGroupName: zhhqFieldString(data["userGroupName"]),
			AccountState:  toIntLoose(data["accountState"], 0),
			Mobile:        zhhqFieldString(data["mobile"]),
			Email:         zhhqFieldString(data["email"]),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*PasspointUserInfo), nil
}

// AddPasspointDevice 添加无感设备。
// defaultServiceName 空串表示校园网出口；学生身份可填运营商（如 中国电信）。
// macExpireTime 为 0-365 天，0 表示最长有效期 6 年。
func (s *NewServiceService) AddPasspointDevice(userMac string, macExpireTime int, defaultServiceName string) error {
	_, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.PostForm(newserviceBase+newservicePath+"/site/passpoint/add-user-mab-info", newserviceHeaders,
			url.Values{
				"userMac":            {userMac},
				"macExpireTime":      {fmt.Sprint(macExpireTime)},
				"defaultServiceName": {defaultServiceName},
			})
		if err != nil {
			return nil, err
		}
		out, err := decodeNewServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		if _, err := checkNewServiceData(out); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

// CancelPasspointDevice 取消指定设备的无感认证。
func (s *NewServiceService) CancelPasspointDevice(userMac string) error {
	_, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.PostForm(newserviceBase+newservicePath+"/site/passpoint/cancel-user-mab-info", newserviceHeaders,
			url.Values{"userMac": {userMac}})
		if err != nil {
			return nil, err
		}
		out, err := decodeNewServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		if _, err := checkNewServiceData(out); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}
