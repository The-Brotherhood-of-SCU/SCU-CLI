// Package cli 定义 scu 命令行工具的全部命令。
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scu",
	Short: "四川大学校园服务命令行工具",
	Long: `scu 是四川大学校园服务的命令行接口，面向 AI 与脚本使用。

覆盖服务：
  统一认证  login / logout / whoami
  教务系统  课表 / 成绩 / 考表 / 空闲教室 / 培养方案 / 校历
  微服务    用户信息
  缴费平台  电费 / 空调余额
  体测系统  体测成绩 / 通知
  第二课堂  活动 / 报名 / 学分

所有命令默认输出 JSON 到 stdout，诊断信息输出到 stderr，
退出码 0 表示成功，非 0 表示失败。`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute 运行根命令。
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd, captchaCmd)
	rootCmd.AddCommand(zhjwCmd)
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(balanceCmd)
	rootCmd.AddCommand(fitnessCmd)
	rootCmd.AddCommand(ccylCmd)
}
