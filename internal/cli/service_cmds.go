package cli

import (
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/spf13/cobra"
)

// ─── service 命令组（网上办事大厅） ──────────────────────────────

func newServiceHallService() (*api.ServiceHallService, error) {
	a, err := newScuAuth()
	if err != nil {
		return nil, err
	}
	return api.NewServiceHallService(auth.NewServiceHallAuth(a)), nil
}

var serviceAppsArgs struct{ status, page int }

var serviceApplicationsCmd = &cobra.Command{
	Use:   "applications",
	Short: "我的申请列表（--status 0 全部 / 1 进行中+草稿 / 3 已完成；每页 20 条）",
	Long: `查询办事大厅"我的申请"。--status 是筛选档位而非实例状态值；
列表项的 inst_status 字段是中文状态文案（如"已完成"），app_name 为事项名，created 为提交时间。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newServiceHallService()
			if err != nil {
				return nil, err
			}
			return s.FetchMyApplications(serviceAppsArgs.status, serviceAppsArgs.page)
		})
	},
}

func init() {
	serviceApplicationsCmd.Flags().IntVar(&serviceAppsArgs.status, "status", 0, "筛选档位：0 全部，1 进行中+草稿，3 已完成")
	serviceApplicationsCmd.Flags().IntVar(&serviceAppsArgs.page, "page", 1, "页码（每页固定 20 条）")

	serviceCmd.AddCommand(serviceApplicationsCmd)
}
