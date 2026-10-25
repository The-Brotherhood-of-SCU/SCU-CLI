package cli

import (
	"errors"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/config"
)

// errorKind 为全部命令提供机器可读的错误分类。
func errorKind(err error) string {
	// 上传一旦提交，后续认证错误也必须提示先查任务，不能引导直接重传。
	var uncertain *api.CloudPrintUploadUncertainError
	if errors.As(err, &uncertain) {
		return "upload_uncertain"
	}
	switch {
	case auth.IsUnauthenticated(err):
		return "unauthenticated"
	case config.IsConfigError(err):
		return "config"
	default:
		return "service"
	}
}
