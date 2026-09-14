// Package cli 定义 scu 命令行工具的全部命令。
package cli

import (
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/output"
	"github.com/spf13/cobra"
)

// Version 由构建时 -ldflags -X 注入（发布为标签名如 v0.1.0）；源码开发构建为 dev。
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "scu",
	Short:   "四川大学校园服务命令行工具",
	Version: Version,
	Long: `scu 是四川大学校园服务的命令行接口，面向 AI 与脚本使用。

覆盖服务：
  统一认证  login / logout / whoami
  教务系统  课表 / 成绩 / 考表 / 空闲教室 / 培养方案 / 校历 / 课程课表
  微服务    用户信息 / 校园网设备管理
  缴费平台  电费 / 空调余额
  体测系统  体测成绩 / 通知
  第二课堂  活动 / 报名 / 学分
  办事大厅  我的申请 / 事项表单提交
  智慧后勤  在线报修（工单提交 / 进度 / 撤回 / 评价）
  无感认证  校园网 Passpoint MAC 绑定管理

所有命令默认输出 JSON 到 stdout，诊断信息输出到 stderr，
退出码 0 表示成功，非 0 表示失败。`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute 运行根命令。RunE 之外的错误（参数数量校验、flag 解析、未知命令）
// 没有经过 output.Fail，这里补写失败包层，保证 stdout 恒为 JSON。
func Execute() error {
	err := rootCmd.Execute()
	if err != nil && !output.IsReported(err) {
		return output.Fail("input", err)
	}
	return err
}

func init() {
	rootCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd, captchaCmd)
	rootCmd.AddCommand(zhjwCmd)
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(balanceCmd)
	rootCmd.AddCommand(fitnessCmd)
	rootCmd.AddCommand(ccylCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(repairCmd)
	rootCmd.AddCommand(passpointCmd)
}
