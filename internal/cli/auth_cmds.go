package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// 以下 run 函数为占位实现，会在各模块提交中替换为真实逻辑。

func runLogin(cmd *cobra.Command) error {
	return errors.New("login 尚未实现")
}

func runLogout(cmd *cobra.Command) error {
	return errors.New("logout 尚未实现")
}

func runWhoami(cmd *cobra.Command) error {
	return errors.New("whoami 尚未实现")
}
