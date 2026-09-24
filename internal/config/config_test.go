package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigErrorClassification 配置层错误必须可被 IsConfigError 识别，
// 且错误信息保持原样（供 message 展示）。
func TestConfigErrorClassification(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SCU_CLI_CONFIG_DIR", dir)

	// 正常写入后再破坏文件，触发解析错误。
	if err := SaveCredentials(&Credentials{Token: "t"}); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte("{bad json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCredentials()
	if err == nil {
		t.Fatal("损坏的凭据文件应返回错误")
	}
	if !IsConfigError(err) {
		t.Fatalf("凭据解析错误应归类为 ConfigError, got %T", err)
	}
	if !strings.Contains(err.Error(), "解析凭据失败") {
		t.Fatalf("错误信息不应被包装改变: %v", err)
	}

	// 不存在时不是错误，更不是 ConfigError。
	t.Setenv("SCU_CLI_CONFIG_DIR", filepath.Join(dir, "absent"))
	c, err := LoadCredentials()
	if err != nil {
		t.Fatalf("文件不存在应返回空凭据: %v", err)
	}
	if c.Token != "" {
		t.Fatalf("文件不存在应返回空凭据, got %+v", c)
	}
}
