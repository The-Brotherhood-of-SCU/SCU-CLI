package cli

import (
	"os"
	"path/filepath"
	"testing"

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
