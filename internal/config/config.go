// Package config 负责 SCU-CLI 的配置目录与凭据持久化。
//
// 凭据（token、账号密码等敏感信息）存放在用户配置目录下的 credentials.json，
// 文件权限为 0600。不得将凭据写入普通配置或日志。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Credentials 是持久化的敏感凭据，与 Bugaoshan 的安全存储对应。
type Credentials struct {
	// Username / Password 仅用于会话过期后的自动重新登录。
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	// Token 是 SCU 统一认证的 access token。
	Token string `json:"token,omitempty"`
	// Principal 是当前 token 绑定的账号（通常为学号），用于校验 token 归属。
	Principal string `json:"principal,omitempty"`
	// TokenFingerprint 用于校验持久化的 token 与 principal 的绑定关系。
	TokenFingerprint string `json:"token_fingerprint,omitempty"`
	// LoginTime 是最近一次登录/刷新成功的时间，用于本地 TTL 判断。
	LoginTime time.Time `json:"login_time,omitempty"`

	// CcylToken / CcylUserID / CcylPrincipal 是第二课堂（dekt）独立 token 及绑定。
	CcylToken     string `json:"ccyl_token,omitempty"`
	CcylUserID    string `json:"ccyl_user_id,omitempty"`
	CcylPrincipal string `json:"ccyl_principal,omitempty"`
}

// ConfigError 表示配置层错误（配置目录、凭据读写/解析）。
// CLI 输出层据此归类 error.kind = "config"，与服务端/网络错误（service）
// 区分开——AI 依赖该分类决定重试还是提示用户修本地环境。
type ConfigError struct{ err error }

func (e *ConfigError) Error() string { return e.err.Error() }
func (e *ConfigError) Unwrap() error { return e.err }

// IsConfigError 报告 err（或其包装链）是否为配置层错误。
func IsConfigError(err error) bool {
	var ce *ConfigError
	return errors.As(err, &ce)
}

// Dir 返回配置目录，依次尝试 $SCU_CLI_CONFIG_DIR、os.UserConfigDir()/scu-cli。
func Dir() (string, error) {
	if d := os.Getenv("SCU_CLI_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", &ConfigError{fmt.Errorf("无法确定用户配置目录: %w", err)}
	}
	return filepath.Join(base, "scu-cli"), nil
}

func credentialsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// LoadCredentials 读取持久化凭据；文件不存在时返回空凭据而非错误。
func LoadCredentials() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Credentials{}, nil
	}
	if err != nil {
		return nil, &ConfigError{fmt.Errorf("读取凭据失败: %w", err)}
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, &ConfigError{fmt.Errorf("解析凭据失败: %w", err)}
	}
	return &c, nil
}

// SaveCredentials 以 0600 权限原子写入凭据。
func SaveCredentials(c *Credentials) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return &ConfigError{fmt.Errorf("创建配置目录失败: %w", err)}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return &ConfigError{err}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return &ConfigError{fmt.Errorf("写入凭据失败: %w", err)}
	}
	if err := os.Rename(tmp, path); err != nil {
		return &ConfigError{fmt.Errorf("提交凭据失败: %w", err)}
	}
	return nil
}

// ClearCredentials 删除持久化凭据（退出登录）。
func ClearCredentials() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return &ConfigError{fmt.Errorf("删除凭据失败: %w", err)}
	}
	return nil
}
