// Package auth 实现 SCU 统一认证与各子系统认证。
package auth

import "errors"

// 认证异常分类，与 Bugaoshan 的 scu_exceptions 对应。
// 使用 errors.Is / errors.As 判定。

// UnauthenticatedError 表示根或子系统认证失效，可在恢复边界重试一次。
type UnauthenticatedError struct{ Msg string }

func (e *UnauthenticatedError) Error() string { return e.Msg }

// ServiceError 表示 HTTP、响应格式或服务端业务错误，不触发认证重试。
type ServiceError struct{ Msg string }

func (e *ServiceError) Error() string { return e.Msg }

// RateLimitedError 表示明确的服务端限流，不自动重试。
type RateLimitedError struct{ Msg string }

func (e *RateLimitedError) Error() string { return e.Msg }

// LoginError 表示验证码、账号密码或登录接口错误。
type LoginError struct {
	Msg    string
	// InvalidCaptcha 为 true 时调用方可换验证码重试。
	InvalidCaptcha bool
}

func (e *LoginError) Error() string { return e.Msg }

// CcylAuthExpiredError 表示第二课堂明确返回 token 过期，可在 CCYL 恢复边界重试一次。
type CcylAuthExpiredError struct{ Msg string }

func (e *CcylAuthExpiredError) Error() string { return e.Msg }

// IsUnauthenticated 判定错误是否为认证失效。
func IsUnauthenticated(err error) bool {
	var e *UnauthenticatedError
	return errors.As(err, &e)
}
