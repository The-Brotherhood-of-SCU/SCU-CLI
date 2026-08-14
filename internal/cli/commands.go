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
	Long: `教务系统。各编号类参数的发现链：

  planCode      ← zhjw semesters 输出的 value（如 2025-2026-2-1）
  校区/教学楼号  ← zhjw classroom index（campuses[].campusNumber、
                  buildings[].teachingBuildingNumber）
  学院/年级代码  ← zhjw program colleges / grades 输出的 value
  fajhh         ← zhjw program search 输出记录
  urlPath       ← zhjw program detail 输出 treeList 节点
  院系/专业/班级 ← zhjw class options → subjects → list`,
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
	Long: `缴费平台。房间参数的逐级发现链（各级输出均为 [{name, code}]）：

  schoolCode ← balance campus 输出的 code
  regCode    ← balance buildings <schoolCode> 输出的 code
  unitCode   ← balance units <schoolCode> <regCode> 输出的 code`,
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
	Long: `第二课堂。各 ID 的发现链：

  activityLibraryId ← ccyl activities 输出记录的 id
  activityId        ← ccyl lib-detail <activityLibraryId> 输出中各场次活动的 id
  --score-type      ← ccyl score-types <activityLibraryId> 输出的能力类型 id
  creditId          ← ccyl credits 输出记录`,
}

// 办事大厅命令组

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "网上办事大厅（我的申请 / 事项办理）",
}

func init() {
	loginCmd.Flags().StringP("username", "u", "", "学号（省略则交互输入）")
	loginCmd.Flags().StringP("password", "p", "", "密码（省略则交互输入，不推荐在命令行传递）")
}
