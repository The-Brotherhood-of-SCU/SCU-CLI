package cli

import (
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/spf13/cobra"
)

// ─── fitness 命令组 ─────────────────────────────────────────────

func newFitnessService() (*api.FitnessService, error) {
	a, err := newScuAuth()
	if err != nil {
		return nil, err
	}
	return api.NewFitnessService(auth.NewFitnessAuth(a)), nil
}

var fitnessNoticesCmd = &cobra.Command{
	Use:   "notices",
	Short: "获取体测通知列表",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newFitnessService()
			if err != nil {
				return nil, err
			}
			return s.FetchNotices()
		})
	},
}

var fitnessScoreYear int

var fitnessScoreCmd = &cobra.Command{
	Use:   "score",
	Short: "获取体测成绩（--year 指定年度，默认今年）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newFitnessService()
			if err != nil {
				return nil, err
			}
			data, err := s.FetchScore(fitnessScoreYear)
			if err != nil {
				return nil, err
			}
			if data == nil {
				return map[string]interface{}{"year": fitnessScoreYear, "score": nil, "message": "该年度无体测成绩"}, nil
			}
			return map[string]interface{}{"year": fitnessScoreYear, "score": data}, nil
		})
	},
}

// ─── ccyl 命令组 ─────────────────────────────────────────────────

func newCcylService() (*api.CcylService, error) {
	a, err := newScuAuth()
	if err != nil {
		return nil, err
	}
	return api.NewCcylService(auth.NewCcylAuth(a)), nil
}

var ccylSearchArgs struct {
	name      string
	level     string
	scoreType string
	org       string
	order     string
	status    string
	quality   string
	page      int
	size      int
}

var ccylActivitiesCmd = &cobra.Command{
	Use:   "activities",
	Short: "搜索第二课堂活动库",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			a := ccylSearchArgs
			return s.SearchActivities(a.page, a.size, a.name, a.level, a.scoreType, a.org, a.order, a.status, a.quality)
		})
	},
}

var ccylMineCmd = &cobra.Command{
	Use:   "mine",
	Short: "获取我参与的活动",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			return s.GetMyActivities(ccylSearchArgs.page, ccylSearchArgs.size)
		})
	},
}

var ccylOrgsCmd = &cobra.Command{
	Use:   "orgs",
	Short: "获取全部组织",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			return s.GetAllOrgs()
		})
	},
}

var ccylDetailCmd = &cobra.Command{
	Use:   "detail <activityId>",
	Short: "获取活动详情",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			return s.GetActivityDetail(args[0])
		})
	},
}

var ccylLibDetailCmd = &cobra.Command{
	Use:   "lib-detail <activityLibraryId>",
	Short: "获取活动系列详情",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			return s.GetActivityLibDetail(args[0])
		})
	},
}

var ccylScoreTypesCmd = &cobra.Command{
	Use:   "score-types <activityLibraryId>",
	Short: "获取活动系列的能力类型（报名时需要 scoreType）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			return s.GetActivityScoreTypes(args[0])
		})
	},
}

var ccylSignUpScoreType string

var ccylSignUpCmd = &cobra.Command{
	Use:   "signup <activityId>",
	Short: "报名活动（--score-type 能力类型，由 score-types 获取）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			if err := s.SignUpActivity(args[0], ccylSignUpScoreType); err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "报名成功", "activity_id": args[0]}, nil
		})
	},
}

var ccylCancelCmd = &cobra.Command{
	Use:   "cancel <activityId>",
	Short: "取消报名",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			if err := s.CancelSignUp(args[0]); err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "已取消报名", "activity_id": args[0]}, nil
		})
	},
}

var ccylSubscribeCmd = &cobra.Command{
	Use:   "subscribe <activityLibraryId>",
	Short: "预约活动系列",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			if err := s.SubscribeActivity(args[0]); err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "预约成功", "activity_library_id": args[0]}, nil
		})
	},
}

var ccylUnsubscribeCmd = &cobra.Command{
	Use:   "unsubscribe <activityLibraryId>",
	Short: "取消预约活动系列",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			if err := s.CancelSubscribe(args[0]); err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "已取消预约", "activity_library_id": args[0]}, nil
		})
	},
}

var ccylCreditsCmd = &cobra.Command{
	Use:   "credits",
	Short: "获取第二课堂成绩单（学分）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			return s.GetCreditList(ccylSearchArgs.page, ccylSearchArgs.size)
		})
	},
}

var ccylExportCmd = &cobra.Command{
	Use:   "export <email> <creditId1> [creditId2...]",
	Short: "导出成绩单到邮箱",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newCcylService()
			if err != nil {
				return nil, err
			}
			msg, err := s.ExportCreditsToEmail(args[1:], args[0])
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": msg}, nil
		})
	},
}

func init() {
	fitnessScoreCmd.Flags().IntVar(&fitnessScoreYear, "year", time.Now().Year(), "体测年度")
	fitnessCmd.AddCommand(fitnessNoticesCmd, fitnessScoreCmd)

	a := ccylActivitiesCmd.Flags()
	a.StringVar(&ccylSearchArgs.name, "name", "", "活动名称关键词")
	a.StringVar(&ccylSearchArgs.level, "level", "", "活动级别")
	a.StringVar(&ccylSearchArgs.scoreType, "score-type", "", "能力类型")
	a.StringVar(&ccylSearchArgs.org, "org", "", "组织编号")
	a.StringVar(&ccylSearchArgs.order, "order", "", "排序")
	a.StringVar(&ccylSearchArgs.status, "status", "", "状态")
	a.StringVar(&ccylSearchArgs.quality, "quality", "", "素质类型")
	a.IntVar(&ccylSearchArgs.page, "page", 1, "页码")
	a.IntVar(&ccylSearchArgs.size, "size", 10, "每页数量")

	// mine / credits 复用 page/size 标志
	ccylMineCmd.Flags().IntVar(&ccylSearchArgs.page, "page", 1, "页码")
	ccylMineCmd.Flags().IntVar(&ccylSearchArgs.size, "size", 10, "每页数量")
	ccylCreditsCmd.Flags().IntVar(&ccylSearchArgs.page, "page", 1, "页码")
	ccylCreditsCmd.Flags().IntVar(&ccylSearchArgs.size, "size", 10, "每页数量")

	ccylSignUpCmd.Flags().StringVar(&ccylSignUpScoreType, "score-type", "", "能力类型 ID（由 score-types 获取）")

	ccylCmd.AddCommand(ccylActivitiesCmd, ccylMineCmd, ccylOrgsCmd, ccylDetailCmd,
		ccylLibDetailCmd, ccylScoreTypesCmd, ccylSignUpCmd, ccylCancelCmd,
		ccylSubscribeCmd, ccylUnsubscribeCmd, ccylCreditsCmd, ccylExportCmd)
}
