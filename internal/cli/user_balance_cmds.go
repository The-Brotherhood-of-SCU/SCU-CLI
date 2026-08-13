package cli

import (
	"errors"
	"fmt"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/spf13/cobra"
)

func newWfwService() (*api.WfwService, error) {
	a, err := newScuAuth()
	if err != nil {
		return nil, err
	}
	return api.NewWfwService(auth.NewWfwAuth(a)), nil
}

// ─── user 命令组 ─────────────────────────────────────────────────

var userInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "获取用户基本信息（姓名、学号、角色等）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newWfwService()
			if err != nil {
				return nil, err
			}
			return s.FetchUserProfile()
		})
	},
}

var userLabelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "获取用户信息标签",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newWfwService()
			if err != nil {
				return nil, err
			}
			return s.FetchProfileLabels()
		})
	},
}

var userDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "获取校园网在线设备列表",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newWfwService()
			if err != nil {
				return nil, err
			}
			return s.FetchNetworkDevices()
		})
	},
}

// ─── balance 命令组 ─────────────────────────────────────────────

// newPayAppService 构造缴费平台服务（PayApp 依赖 WFW 认证）。
func newPayAppService(scu *auth.ScuAuth) *api.PayAppService {
	wfwAuth := auth.NewWfwAuth(scu)
	return api.NewPayAppService(auth.NewPayAppAuth(scu, wfwAuth))
}

var balanceCampusCmd = &cobra.Command{
	Use:   "campus",
	Short: "获取校区列表（[{name, code}]，code 即 schoolCode）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			scu, err := newScuAuth()
			if err != nil {
				return nil, err
			}
			return newPayAppService(scu).GetCampus()
		})
	},
}

var balanceBuildingsCmd = &cobra.Command{
	Use:   "buildings <schoolCode>",
	Short: "获取楼栋列表（schoolCode 取自 campus 的 code；输出 code 即 regCode）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			scu, err := newScuAuth()
			if err != nil {
				return nil, err
			}
			return newPayAppService(scu).GetArchitecture(args[0])
		})
	},
}

var balanceUnitsCmd = &cobra.Command{
	Use:   "units <schoolCode> <regCode>",
	Short: "获取单元列表（regCode 取自 buildings 的 code；输出 code 即 unitCode）",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			scu, err := newScuAuth()
			if err != nil {
				return nil, err
			}
			return newPayAppService(scu).GetUnit(args[0], args[1])
		})
	},
}

var balanceQueryArgs struct {
	type_      int
	schoolCode string
	regCode    string
	unitCode   string
	roomNo     string
}

var balanceQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "查询余额（--type 1 照明电费 / 2 空调电费）",
	Long: `查询房间余额。cusNo/cusName 自动取自微服务用户信息。

首次查询某房间需提供房间信息完成绑定：
  scu balance query --type 1 --school-code 1 --reg-code 101 --unit-code 1 --room 101

绑定后可直接查询（使用已绑定的房间）：
  scu balance query --type 1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			scu, err := newScuAuth()
			if err != nil {
				return nil, err
			}
			// cusNo / cusName 来自微服务用户信息。
			wfwSvc := api.NewWfwService(auth.NewWfwAuth(scu))
			profile, err := wfwSvc.FetchUserProfile()
			if err != nil {
				return nil, err
			}
			cusName, _ := profile["realname"].(string)
			cusNo := ""
			if role, ok := profile["role"].(map[string]interface{}); ok {
				cusNo, _ = role["number"].(string)
			}
			if cusNo == "" {
				return nil, errors.New("无法从微服务用户信息获取学号 (role.number)")
			}

			a := balanceQueryArgs
			pay := newPayAppService(scu)
			// 提供房间信息时先绑定（与 App 的 verificationRoom 一致）。
			if a.roomNo != "" {
				ok, err := pay.VerificationRoom(cusNo, a.type_, cusName, a.schoolCode, a.regCode, a.unitCode, a.roomNo)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, fmt.Errorf("房间绑定失败（%s %s %s %s），请检查房间信息", a.schoolCode, a.regCode, a.unitCode, a.roomNo)
				}
			}
			info, err := pay.QueryRoomInfo(cusNo, a.type_, cusName)
			if err != nil {
				return nil, err
			}
			typeName := map[int]string{1: "照明电费", 2: "空调电费"}[a.type_]
			return map[string]interface{}{
				"type_name": typeName,
				"room_info": info,
			}, nil
		})
	},
}

func init() {
	userCmd.AddCommand(userInfoCmd, userLabelsCmd, userDevicesCmd)

	q := balanceQueryCmd.Flags()
	q.IntVar(&balanceQueryArgs.type_, "type", 1, "查询类型：1 照明电费，2 空调电费")
	q.StringVar(&balanceQueryArgs.schoolCode, "school-code", "", "校区代码（balance campus 的 code；绑定时必填）")
	q.StringVar(&balanceQueryArgs.regCode, "reg-code", "", "楼栋代码（balance buildings 的 code；绑定时必填）")
	q.StringVar(&balanceQueryArgs.unitCode, "unit-code", "", "单元代码（balance units 的 code；无单元可空）")
	q.StringVar(&balanceQueryArgs.roomNo, "room", "", "房间号（提供时先绑定房间）")

	balanceCmd.AddCommand(balanceCampusCmd, balanceBuildingsCmd, balanceUnitsCmd, balanceQueryCmd)
}
