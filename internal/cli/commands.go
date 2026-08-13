package cli

import (
	"github.com/spf13/cobra"
)

// 认证命令

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "统一认证登录（学号 + 密码 + 验证码）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogin(cmd)
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "退出登录，清除本地凭据",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogout(cmd)
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "显示当前登录账号与凭据状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWhoami(cmd)
	},
}

// 教务系统命令组

var zhjwCmd = &cobra.Command{
	Use:   "zhjw",
	Short: "教务系统（课表 / 成绩 / 考表 / 教室 / 培养方案 / 校历）",
}

// 用户信息命令组

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "微服务用户信息",
}

// 缴费平台命令组

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "缴费平台余额（电费 / 空调）",
}

// 体测命令组

var fitnessCmd = &cobra.Command{
	Use:   "fitness",
	Short: "体测系统（成绩 / 通知）",
}

// 第二课堂命令组

var ccylCmd = &cobra.Command{
	Use:   "ccyl",
	Short: "第二课堂（活动 / 报名 / 学分）",
}

func init() {
	loginCmd.Flags().StringP("username", "u", "", "学号（省略则交互输入）")
	loginCmd.Flags().StringP("password", "p", "", "密码（省略则交互输入，不推荐在命令行传递）")
}
