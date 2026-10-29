package auth

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
)

const CloudPrintBase = "http://202.115.54.240"
const CloudPrintPage = "/client/new/cprintPc/cprint.html"
const CloudPrintJobsPage = "/client/new/cprintPc/printDoc.html"

// CloudPrintEnvelope 是打印服务的 JSON 包层，与 CLI 输出包层不同。
// code 缺失时不能将其误认为成功的 code=0。
type CloudPrintEnvelope struct {
	Code      *int            `json:"code"`
	Message   string          `json:"message"`
	Result    json.RawMessage `json:"result"`
	SessionID string          `json:"SessionId"`
}

func DecodeCloudPrintResponse(resp *Response) (*CloudPrintEnvelope, error) {
	if resp.StatusCode == 401 || resp.StatusCode == 403 || isRedirect(resp.StatusCode) {
		return nil, &UnauthenticatedError{Msg: "云打印会话已失效"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ServiceError{Msg: fmt.Sprintf("云打印请求失败（HTTP %d）", resp.StatusCode)}
	}
	var envelope CloudPrintEnvelope
	if json.Unmarshal(resp.Body, &envelope) != nil || envelope.Code == nil {
		return nil, &ServiceError{Msg: "云打印响应缺少有效的 JSON code 字段"}
	}
	if *envelope.Code == 4 {
		return nil, &UnauthenticatedError{Msg: "云打印会话已失效"}
	}
	if *envelope.Code != 0 {
		msg := envelope.Message
		if msg == "" {
			msg = "云打印服务返回错误"
		}
		return nil, &ServiceError{Msg: fmt.Sprintf("%s（code=%d）", msg, *envelope.Code)}
	}
	return &envelope, nil
}

func CheckCloudPrint(client *CookieClient, base string) error {
	resp, err := client.Do("POST", base+"/api/client/Auth/Check", map[string]string{
		"Accept": "application/json", "Referer": base + CloudPrintPage,
	}, nil)
	if err != nil {
		return &ServiceError{Msg: "云打印 Auth/Check 请求失败（网络或超时）"}
	}
	result, err := DecodeCloudPrintResponse(resp)
	if err != nil {
		return err
	}
	if result.SessionID == "" {
		return &UnauthenticatedError{Msg: "云打印尚未建立登录会话（SessionId 为空）"}
	}
	return nil
}

type cloudPrintRoot interface{ GetClient() (*CookieClient, error) }

// CloudPrintAuth 解析 SSoPage 的 JSON 登录链接，用根 cookie jar 完成 libic/CAS 回跳。
// SCU token 与密码都不能发送到打印 IP。
type CloudPrintAuth struct {
	scu        cloudPrintRoot
	base       string
	allowed    func(*url.URL) bool
	mu         sync.Mutex
	lastClient *CookieClient
	ready      bool
}

// cloudPrintOriginAllowed 限定打印站、图书馆中继和统一认证的跳转来源。
func cloudPrintOriginAllowed(u *url.URL) bool {
	if u.User != nil || u.Fragment != "" {
		return false
	}
	return (u.Scheme == "http" && u.Host == "202.115.54.240") ||
		(u.Scheme == "https" && (u.Host == "libic.scu.edu.cn" || u.Host == "id.scu.edu.cn"))
}

func NewCloudPrintAuth(scu *ScuAuth) *CloudPrintAuth {
	return &CloudPrintAuth{scu: scu, base: CloudPrintBase, allowed: cloudPrintOriginAllowed}
}

func (a *CloudPrintAuth) ModuleID() string { return "cloudprint" }

func (a *CloudPrintAuth) GetClient() (*CookieClient, error) {
	client, err := a.scu.GetClient()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if client != a.lastClient {
		a.lastClient, a.ready = client, false
	}
	if a.ready {
		return client, nil
	}
	// Auth/Check 也会创建后续 SSoPage 使用的匿名会话。
	if err = CheckCloudPrint(client, a.base); err == nil {
		a.ready = true
		return client, nil
	} else if !IsUnauthenticated(err) {
		return nil, err
	}
	resp, err := client.Get(a.base+"/api/client/Auth/SSoPage?backurl="+url.QueryEscape(a.base+CloudPrintPage), map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, &ServiceError{Msg: "云打印 SSoPage 请求失败（网络或超时）"}
	}
	envelope, err := DecodeCloudPrintResponse(resp)
	if err != nil {
		return nil, err
	}
	var loginURL string
	if json.Unmarshal(envelope.Result, &loginURL) != nil || loginURL == "" {
		return nil, &ServiceError{Msg: "云打印 SSoPage 未返回登录链接"}
	}
	resp, err = client.FollowRedirectsAllowed(loginURL, nil, a.allowed)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ServiceError{Msg: fmt.Sprintf("图书馆认证回跳失败（HTTP %d）", resp.StatusCode)}
	}
	if err = CheckCloudPrint(client, a.base); err != nil {
		return nil, err
	}
	a.ready = true
	return client, nil
}

func (a *CloudPrintAuth) Invalidate() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.lastClient = false, nil
}
