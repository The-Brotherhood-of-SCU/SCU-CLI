package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// PayAppService 缴费平台业务 API：校区/楼栋/单元、房间绑定、余额查询。
type PayAppService struct {
	auth *auth.SsoRelayAuth
}

func NewPayAppService(a *auth.SsoRelayAuth) *PayAppService { return &PayAppService{auth: a} }

func (s *PayAppService) request(fn func(*auth.CookieClient) (interface{}, error)) (interface{}, error) {
	return auth.RetryOnUnauthenticated(s.auth.GetClient, fn, s.auth.Invalidate)
}

const payBase = "https://payapp.scu.edu.cn/eleFees"

var payHeaders = map[string]string{
	"Accept":       "application/json, text/plain, */*",
	"Content-Type": "application/json;charset=UTF-8",
	"Origin":       payBase,
	"Referer":      payBase,
	"User-Agent":   auth.DefaultUserAgent,
}

// payAuthPaths 重定向到这些路径说明缴费 session 已失效。
var payAuthPaths = map[string]bool{
	"/eleFees/index.html":         true,
	"/eleFees/oauth/airWarrant":   true,
	"/eleFees/oauth/lightWarrant": true,
}

// checkPayExpiry 识别缴费平台认证失效（与 Bugaoshan 一致）：
//  1. 3xx 且 Location 指向 payapp 的已知认证路径；
//  2. HTML 页面含「登录超时」且含 warrant 表单。
func checkPayExpiry(resp *auth.Response) error {
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc := resp.Header.Get("Location"); loc != "" {
			if target, err := resp.URL.Parse(loc); err == nil {
				if strings.EqualFold(target.Host, "payapp.scu.edu.cn") && payAuthPaths[target.Path] {
					return &auth.UnauthenticatedError{Msg: "缴费平台登录状态已失效"}
				}
			}
		}
	}
	body := string(resp.Body)
	lower := strings.ToLower(body)
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	looksHTML := strings.Contains(contentType, "text/html") ||
		strings.Contains(lower, "<html") || strings.Contains(lower, "<!doctype html")
	hasWarrant := strings.Contains(lower, "/elefees/oauth/airwarrant") ||
		strings.Contains(lower, "/elefees/oauth/lightwarrant")
	if looksHTML && strings.Contains(body, "登录超时") && hasWarrant {
		return &auth.UnauthenticatedError{Msg: "缴费平台登录状态已失效"}
	}
	return nil
}

// post 是缴费平台 JSON POST 的公共实现。
func (s *PayAppService) post(path string, payload map[string]interface{}) (map[string]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		bodyBytes, _ := json.Marshal(payload)
		resp, err := c.Do("POST", payBase+path, payHeaders, bytesReaderOf(bodyBytes))
		if err != nil {
			return nil, err
		}
		if err := checkPayExpiry(resp); err != nil {
			return nil, err
		}
		var out map[string]interface{}
		if err := json.Unmarshal(resp.Body, &out); err != nil {
			return nil, &auth.ServiceError{Msg: fmt.Sprintf("[%s] 响应解析失败", path)}
		}
		if out["respCode"] != "00" {
			desc, _ := out["respDesc"].(string)
			if desc == "" {
				desc = "缴费平台请求失败"
			}
			return nil, &auth.ServiceError{Msg: desc}
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// datas 提取 data.datas 数组。
func datas(resp map[string]interface{}) []interface{} {
	data, _ := resp["data"].(map[string]interface{})
	list, _ := data["datas"].([]interface{})
	if list == nil {
		list = []interface{}{}
	}
	return list
}

// GetCampus 获取校区列表。
func (s *PayAppService) GetCampus() ([]interface{}, error) {
	resp, err := s.post("/electric/getCampus", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	return datas(resp), nil
}

// GetArchitecture 获取楼栋列表。
func (s *PayAppService) GetArchitecture(schoolCode string) ([]interface{}, error) {
	resp, err := s.post("/electric/getArchitecture", map[string]interface{}{"schoolCode": schoolCode})
	if err != nil {
		return nil, err
	}
	return datas(resp), nil
}

// GetUnit 获取单元列表。
func (s *PayAppService) GetUnit(schoolCode, regCode string) ([]interface{}, error) {
	resp, err := s.post("/electric/getUnit", map[string]interface{}{"schoolCode": schoolCode, "regCode": regCode})
	if err != nil {
		return nil, err
	}
	return datas(resp), nil
}

// VerificationRoom 验证/绑定房间（type: 1 照明电费, 2 空调电费）。
func (s *PayAppService) VerificationRoom(cusNo string, type_ int, cusName, schoolCode, regCode, unitCode, roomNo string) (bool, error) {
	resp, err := s.post("/electric/verificationRoom", map[string]interface{}{
		"cusNo": cusNo, "type": type_, "cusName": cusName,
		"schoolCode": schoolCode, "regCode": regCode, "unitCode": unitCode, "roomNo": roomNo,
	})
	if err != nil {
		return false, err
	}
	data, _ := resp["data"].(map[string]interface{})
	status, _ := data["status"].(bool)
	return status, nil
}

// QueryRoomInfo 查询房间信息（余额）。type: 1 照明电费, 2 空调电费。
func (s *PayAppService) QueryRoomInfo(cusNo string, type_ int, cusName string) (map[string]interface{}, error) {
	resp, err := s.post("/electric/queryRoomInfo", map[string]interface{}{
		"cusNo": cusNo, "type": type_, "cusName": cusName,
	})
	if err != nil {
		return nil, err
	}
	data, _ := resp["data"].(map[string]interface{})
	if data == nil {
		return nil, &auth.ServiceError{Msg: "查询数据为空"}
	}
	return data, nil
}

// bytesReaderOf 将字节切片包装为 Reader。
func bytesReaderOf(b []byte) *strings.Reader { return strings.NewReader(string(b)) }
