package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// CcylService 第二课堂业务 API：活动、报名、学分。
type CcylService struct {
	auth *auth.CcylAuth
}

func NewCcylService(a *auth.CcylAuth) *CcylService { return &CcylService{auth: a} }

const ccylBase = "https://dekt.scu.edu.cn/ccyl-api"

func ccylHeaders(token string) map[string]string {
	return map[string]string{
		"Accept":       "application/json, text/plain, */*",
		"Content-Type": "application/json;charset=UTF-8",
		"Origin":       "https://dekt.scu.edu.cn",
		"Referer":      "https://dekt.scu.edu.cn",
		"User-Agent":   auth.DefaultUserAgent,
		"token":        token,
	}
}

// do 是 CCYL 的恢复边界（对应 retryOnCcylAuthError）：
// 仅在明确的 token 过期（业务码 401）时重登录并重放一次；
// 普通业务错误直接上抛，防止报名/取消等非幂等操作被重复提交。
func (s *CcylService) do(fn func(token string) (interface{}, error)) (interface{}, error) {
	if err := s.auth.EnsureAuthenticated(); err != nil {
		return nil, err
	}
	result, err := fn(s.auth.Token())
	if err == nil {
		return result, nil
	}
	var expired *auth.CcylAuthExpiredError
	if !errors.As(err, &expired) {
		return nil, err
	}
	if rerr := s.auth.RecoverExpiredSession(); rerr != nil {
		return nil, &auth.UnauthenticatedError{Msg: "第二课堂 token 过期，重新登录失败: " + rerr.Error()}
	}
	return fn(s.auth.Token())
}

// post 发送 JSON POST 并做通用解析（HTTP 200 + 业务码）。
func (s *CcylService) post(api, path string, body map[string]interface{}) (map[string]interface{}, error) {
	v, err := s.do(func(token string) (interface{}, error) {
		payload, _ := json.Marshal(body)
		client, err := auth.NewCookieClient()
		if err != nil {
			return nil, err
		}
		resp, err := client.Do("POST", ccylBase+path, ccylHeaders(token), strings.NewReader(string(payload)))
		if err != nil {
			return nil, &auth.ServiceError{Msg: fmt.Sprintf("[%s] 网络请求失败: %v", api, err)}
		}
		return parseCcylResponse(api, resp)
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// get 发送 GET 并做通用解析。
func (s *CcylService) get(api, path string) (map[string]interface{}, error) {
	v, err := s.do(func(token string) (interface{}, error) {
		client, err := auth.NewCookieClient()
		if err != nil {
			return nil, err
		}
		resp, err := client.Get(ccylBase+path, ccylHeaders(token))
		if err != nil {
			return nil, &auth.ServiceError{Msg: fmt.Sprintf("[%s] 网络请求失败: %v", api, err)}
		}
		return parseCcylResponse(api, resp)
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// parseCcylResponse 解析 CCYL 响应：HTTP 必须 200；业务码 401 视为 token 过期。
func parseCcylResponse(api string, resp *auth.Response) (map[string]interface{}, error) {
	if resp.StatusCode != 200 {
		return nil, &auth.ServiceError{Msg: fmt.Sprintf("[%s] HTTP 错误: %d", api, resp.StatusCode)}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, &auth.ServiceError{Msg: fmt.Sprintf("[%s] 响应解析失败", api)}
	}
	if auth.IsCcylAuthExpiredCode(out["code"]) {
		msg, _ := out["msg"].(string)
		if msg == "" {
			msg = "第二课堂 token 已失效"
		}
		return nil, &auth.CcylAuthExpiredError{Msg: msg}
	}
	return out, nil
}

// requireSuccess 校验业务码为 0，否则返回业务错误。
func requireSuccess(json map[string]interface{}, fallback string) error {
	if code, ok := json["code"].(float64); ok && int(code) == 0 {
		return nil
	}
	msg, _ := json["msg"].(string)
	if msg == "" {
		msg = fallback
	}
	return &auth.ServiceError{Msg: msg}
}

func millis() string { return fmt.Sprint(time.Now().UnixMilli()) }

func listField(json map[string]interface{}, key string) []interface{} {
	list, _ := json[key].([]interface{})
	if list == nil {
		list = []interface{}{}
	}
	return list
}

// SearchActivities 搜索活动库。
func (s *CcylService) SearchActivities(pageNum, pageSize int, name, level, scoreType, org, order, status, quality string) ([]interface{}, error) {
	json, err := s.post("list-activity-library", "/app/activity/list-activity-library", map[string]interface{}{
		"pn": pageNum, "time": millis(), "ps": pageSize,
		"name": name, "level": level, "scoreType": scoreType,
		"org": org, "order": order, "status": status, "quality": quality,
	})
	if err != nil {
		return nil, err
	}
	if err := requireSuccess(json, "获取活动列表失败"); err != nil {
		return nil, err
	}
	return listField(json, "list"), nil
}

// GetMyActivities 获取我参与的活动。
func (s *CcylService) GetMyActivities(pageNum, pageSize int) ([]interface{}, error) {
	json, err := s.post("list-mine", "/app/activity/list-mine", map[string]interface{}{
		"pn": pageNum, "time": millis(), "ps": pageSize,
	})
	if err != nil {
		return nil, err
	}
	if err := requireSuccess(json, "获取我参与的活动失败"); err != nil {
		return nil, err
	}
	return listField(json, "content"), nil
}

// GetAllOrgs 获取全部组织。
func (s *CcylService) GetAllOrgs() ([]interface{}, error) {
	json, err := s.post("list-all", "/app/org/list-all", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	if err := requireSuccess(json, "获取组织列表失败"); err != nil {
		return nil, err
	}
	return listField(json, "list"), nil
}

// GetActivityLibDetail 获取活动系列详情（activityLib + activities + subscribed）。
func (s *CcylService) GetActivityLibDetail(activityLibraryID string) (map[string]interface{}, error) {
	json, err := s.get("get-lib-detail", "/app/activity/get-lib-detail/"+activityLibraryID)
	if err != nil {
		return nil, err
	}
	if err := requireSuccess(json, "获取活动系列详情失败"); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"activity_lib": json["activityLib"],
		"activities":   listField(json, "activities"),
		"subscribed":   json["subscribed"] == true,
	}, nil
}

// GetActivityScoreTypes 获取活动系列的能力类型。
func (s *CcylService) GetActivityScoreTypes(activityLibraryID string) ([]interface{}, error) {
	json, err := s.post("list-activity-score", "/app/activity/list-activity-score/"+activityLibraryID, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	if err := requireSuccess(json, "获取能力类型失败"); err != nil {
		return nil, err
	}
	return listField(json, "list"), nil
}

// SignUpActivity 报名活动。
func (s *CcylService) SignUpActivity(activityID, scoreType string) error {
	json, err := s.post("sign-up-act", "/app/activity/sign-up-act", map[string]interface{}{
		"activityId": activityID, "scoreType": scoreType,
	})
	if err != nil {
		return err
	}
	return requireSuccess(json, "报名失败")
}

// CancelSignUp 取消报名（userID 自动取自当前会话）。
func (s *CcylService) CancelSignUp(activityID string) error {
	userID := s.auth.UserID()
	if userID == "" {
		return &auth.UnauthenticatedError{Msg: "第二课堂未登录"}
	}
	json, err := s.post("cancel", "/app/activity/cancel", map[string]interface{}{
		"activityId": activityID, "userId": userID,
	})
	if err != nil {
		return err
	}
	return requireSuccess(json, "取消报名失败")
}

// SubscribeActivity 预约活动系列。
func (s *CcylService) SubscribeActivity(activityLibraryID string) error {
	json, err := s.post("subscribe-act", "/app/activity/subscribe-act/"+activityLibraryID, map[string]interface{}{})
	if err != nil {
		return err
	}
	return requireSuccess(json, "预约活动失败")
}

// CancelSubscribe 取消预约活动系列。
func (s *CcylService) CancelSubscribe(activityLibraryID string) error {
	json, err := s.post("cancel-subscribe", "/app/activity/cancel-subscribe/"+activityLibraryID, map[string]interface{}{})
	if err != nil {
		return err
	}
	return requireSuccess(json, "取消预约失败")
}

// GetActivityDetail 获取活动详情。
func (s *CcylService) GetActivityDetail(activityID string) (map[string]interface{}, error) {
	json, err := s.post("get-detail", "/app/activity/get-detail", map[string]interface{}{
		"activityId": activityID,
	})
	if err != nil {
		return nil, err
	}
	if err := requireSuccess(json, "获取活动详情失败"); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"activity":     json["activity"],
		"activity_lib": json["activityLib"],
		"is_xtw_role":  json["isXtwRole"] == true,
		"sign_up":      json["signUp"] == true,
	}, nil
}

// GetCreditList 获取成绩单（学分）。
func (s *CcylService) GetCreditList(pageNum, pageSize int) ([]interface{}, error) {
	json, err := s.post("list-credit", "/app/credit/list", map[string]interface{}{
		"pn": pageNum, "ps": pageSize,
	})
	if err != nil {
		return nil, err
	}
	if err := requireSuccess(json, "获取成绩单失败"); err != nil {
		return nil, err
	}
	return listField(json, "list"), nil
}

// ExportCreditsToEmail 导出成绩单到邮箱。
func (s *CcylService) ExportCreditsToEmail(creditIDs []string, email string) (string, error) {
	json, err := s.post("export-pdf", "/app/credit/exportPdfV2", map[string]interface{}{
		"creditIds": strings.Join(creditIDs, ","), "qqEmail": email,
	})
	if err != nil {
		return "", err
	}
	if err := requireSuccess(json, "导出失败"); err != nil {
		return "", err
	}
	msg, _ := json["msg"].(string)
	if msg == "" {
		msg = "成绩单已发送至邮箱"
	}
	return msg, nil
}
