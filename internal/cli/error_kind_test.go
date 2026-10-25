package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/config"
)

// TestErrorKindConfig 凭据损坏是本地配置错误，errorKind 必须给 config
// 而非 service——AI 依赖该分类决定「提示用户修本地环境」还是「按服务端
// 故障处理」。
func TestErrorKindConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SCU_CLI_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte("{bad json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := config.LoadCredentials()
	if err == nil {
		t.Fatal("损坏的凭据文件应返回错误")
	}
	if got := errorKind(err); got != "config" {
		t.Fatalf("errorKind = %q, want config", got)
	}
}

func TestErrorKindCloudPrintUploadBoundary(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"preflight-auth", &auth.UnauthenticatedError{Msg: "未提交文件，登录失败"}, "unauthenticated"},
		{"post-401", &api.CloudPrintUploadUncertainError{Reason: "提交后 HTTP 401"}, "upload_uncertain"},
		{"post-403", &api.CloudPrintUploadUncertainError{Reason: "提交后 HTTP 403"}, "upload_uncertain"},
		{"callback-code-4", &api.CloudPrintUploadUncertainError{Reason: "回跳 code=4"}, "upload_uncertain"},
		{"rejected-413", &auth.ServiceError{Msg: "HTTP 413"}, "service"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 同时验证错误经过包装后仍保持 JSON kind，不会落入通用重试分支。
			if got := errorKind(fmt.Errorf("cloudprint: %w", tc.err)); got != tc.want {
				t.Fatalf("kind=%s, want %s", got, tc.want)
			}
		})
	}
}
