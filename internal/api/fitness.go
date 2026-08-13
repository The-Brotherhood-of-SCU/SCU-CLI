package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// FitnessService 体测系统业务 API：通知、成绩。
type FitnessService struct {
	auth *auth.SsoRelayAuth
}

func NewFitnessService(a *auth.SsoRelayAuth) *FitnessService { return &FitnessService{auth: a} }

const fitnessBase = "https://pead.scu.edu.cn/bdlp_h5_fitness_test/public/index.php"

var fitnessHeaders = map[string]string{
	"Accept":           "application/json, text/plain, */*",
	"Accept-Language":  "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
	"Cache-Control":    "no-cache",
	"Content-Type":     "application/x-www-form-urlencoded",
	"Origin":           "https://pead.scu.edu.cn",
	"Referer":          fitnessBase + "/index/index",
	"User-Agent":       auth.DefaultUserAgent,
	"X-Requested-With": "XMLHttpRequest",
}

func (s *FitnessService) request(fn func(*auth.CookieClient) (interface{}, error)) (interface{}, error) {
	return auth.RetryOnUnauthenticated(s.auth.GetClient, fn, s.auth.Invalidate)
}

// decodeFitnessResponse 解析体测响应：status=='1' 成功；
// info 含「登录信息失效」或「请重新登录」视为认证失效。
func decodeFitnessResponse(body string, api string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, &auth.ServiceError{Msg: fmt.Sprintf("[%s] JSON 解析失败", api)}
	}
	if fmt.Sprint(out["status"]) == "1" {
		return out, nil
	}
	msg, _ := out["info"].(string)
	if msg == "" {
		msg = "体测服务请求失败"
	}
	if strings.Contains(msg, "登录信息失效") || strings.Contains(msg, "请重新登录") {
		return nil, &auth.UnauthenticatedError{Msg: msg}
	}
	return nil, &auth.ServiceError{Msg: msg}
}

// FetchNotices 获取体测通知列表。
func (s *FitnessService) FetchNotices() ([]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.PostForm(fitnessBase+"/index/News/getSchoolNoticeList", fitnessHeaders, nil)
		if err != nil {
			return nil, err
		}
		json, err := decodeFitnessResponse(string(resp.Body), "getSchoolNoticeList")
		if err != nil {
			return nil, err
		}
		list, ok := json["data"].([]interface{})
		if !ok {
			return nil, &auth.ServiceError{Msg: "体测通知响应格式错误"}
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}

// FetchScore 获取指定年度体测成绩（data 可能为 null，表示当年无成绩）。
func (s *FitnessService) FetchScore(year int) (map[string]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.PostForm(fitnessBase+"/index/Report/getStudentScore", fitnessHeaders,
			map[string][]string{"year_num": {fmt.Sprint(year)}})
		if err != nil {
			return nil, err
		}
		json, err := decodeFitnessResponse(string(resp.Body), "getStudentScore")
		if err != nil {
			return nil, err
		}
		if json["data"] == nil {
			return nil, nil
		}
		data, ok := json["data"].(map[string]interface{})
		if !ok {
			return nil, &auth.ServiceError{Msg: "体测成绩响应格式错误"}
		}
		return data, nil
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.(map[string]interface{}), nil
}
