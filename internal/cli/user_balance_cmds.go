package cli

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/config"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/output"
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
	Short: "获取校园网在线设备列表（输出 device_id / ip，供 offline 使用）",
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

var userOfflineArgs struct{ deviceID, ip string }

var userOfflineCmd = &cobra.Command{
	Use:   "offline",
	Short: "强制指定校园网设备下线（--device-id / --ip 取自 user devices 输出）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			if userOfflineArgs.deviceID == "" || userOfflineArgs.ip == "" {
				return nil, errors.New("必须提供 --device-id 与 --ip（取自 user devices 输出）")
			}
			s, err := newWfwService()
			if err != nil {
				return nil, err
			}
			if err := s.ForceNetworkDeviceOffline(userOfflineArgs.deviceID, userOfflineArgs.ip); err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"message":   "设备已下线",
				"device_id": userOfflineArgs.deviceID,
				"ip":        userOfflineArgs.ip,
			}, nil
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
				// 记住绑定房间（趋势快照按 room_key 归集）。
				_ = config.SaveBalanceBinding(a.type_, config.BalanceBinding{
					SchoolCode: a.schoolCode, RegCode: a.regCode, UnitCode: a.unitCode, RoomNo: a.roomNo,
				})
			}
			info, err := pay.QueryRoomInfo(cusNo, a.type_, cusName)
			if err != nil {
				return nil, err
			}
			recordBalanceSnapshot(a.type_, info)
			typeName := map[int]string{1: "照明电费", 2: "空调电费"}[a.type_]
			return map[string]interface{}{
				"type_name": typeName,
				"room_info": info,
			}, nil
		})
	},
}

// parseFloatLoose 宽松解析数值字段（服务端返回字符串或数字）。
func parseFloatLoose(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

// recordBalanceSnapshot 查询成功后记录一条余额快照（失败不影响主流程）。
// 余额解析失败跳过记录，避免把 0 写进历史污染趋势（与 Bugaoshan 一致）。
// room_key 取自本次绑定参数或上次 CLI 绑定；都不可得时跳过（服务端绑定
// 的响应里只有名称没有 code）。
func recordBalanceSnapshot(balanceType int, info map[string]interface{}) {
	balance, ok := parseFloatLoose(info["balance"])
	if !ok {
		return
	}
	price, ok := parseFloatLoose(info["price"])
	if !ok {
		price = 0
	}
	var b config.BalanceBinding
	if balanceQueryArgs.roomNo != "" {
		b = config.BalanceBinding{
			SchoolCode: balanceQueryArgs.schoolCode, RegCode: balanceQueryArgs.regCode,
			UnitCode: balanceQueryArgs.unitCode, RoomNo: balanceQueryArgs.roomNo,
		}
	} else {
		loaded, ok, err := config.LoadBalanceBinding(balanceType)
		if err != nil || !ok {
			return
		}
		b = loaded
	}
	_ = config.AppendBalanceSnapshot(config.BalanceSnapshot{
		RoomKey: b.RoomKey(), BalanceType: balanceType,
		Timestamp: time.Now().UTC().UnixMilli(), Balance: balance, Price: price,
	})
}

// ─── balance trend ─────────────────────────────────────────────

var balanceTrendArgs struct{ type_, days int }

var balanceTrendCmd = &cobra.Command{
	Use:   "trend",
	Short: "余额趋势分析（基于本地快照；每次 balance query 成功自动记录）",
	Long: `分析余额消耗趋势。数据来自本地快照：每次 balance query 成功都会记录一条
（按房间 + 类型归集，保留 365 天）。

算法与 Bugaoshan 一致：快照按北京日历日聚合（每日取最后一条），相邻日代表点
按段加权平均；充值段（余额上升）自动跳过并计数。

输出的 daily_points 可直接用于绘图；daily_avg_cost / daily_avg_kwh 为日均
消耗（元/天、度/天），total_* 为统计期内累计。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		typeName := map[int]string{1: "照明电费", 2: "空调电费"}[balanceTrendArgs.type_]
		if typeName == "" {
			return output.Fail("input", fmt.Errorf("--type 只支持 1（照明电费）/ 2（空调电费）"))
		}
		b, ok, err := config.LoadBalanceBinding(balanceTrendArgs.type_)
		if err != nil {
			return output.Fail("config", err)
		}
		if !ok {
			return output.Fail("input", fmt.Errorf("暂无%s的绑定房间，先 balance query --type %d --school-code … --room … 绑定并查询一次", typeName, balanceTrendArgs.type_))
		}
		return runJSON(func() (interface{}, error) {
			snaps, err := config.LoadBalanceSnapshots(b.RoomKey(), balanceTrendArgs.type_)
			if err != nil {
				return nil, err
			}
			records := make([]api.BalanceRecord, 0, len(snaps))
			for _, s := range snaps {
				records = append(records, api.BalanceRecord{
					RoomKey: s.RoomKey, BalanceType: s.BalanceType,
					Timestamp: s.Timestamp, Balance: s.Balance, Price: s.Price,
				})
			}
			if balanceTrendArgs.days > 0 {
				cutoff := time.Now().UTC().AddDate(0, 0, -balanceTrendArgs.days).UnixMilli()
				kept := records[:0]
				for _, r := range records {
					if r.Timestamp >= cutoff {
						kept = append(kept, r)
					}
				}
				records = kept
			}
			res := api.CalculateBalanceTrend(records)
			out := map[string]interface{}{
				"room_key":                  b.RoomKey(),
				"room_no":                   b.RoomNo,
				"balance_type":              balanceTrendArgs.type_,
				"type_name":                 typeName,
				"record_count":              res.RecordCount,
				"current_price":             round4(res.CurrentPrice),
				"daily_avg_cost":            round4(res.DailyAvgCost),
				"daily_avg_kwh":             round4(res.DailyAvgKwh),
				"total_cost":                round4(res.TotalCost),
				"total_kwh":                 round4(res.TotalKwh),
				"total_days":                round4(res.TotalDays),
				"skipped_recharge_segments": res.SkippedRechargeSegments,
			}
			if balanceTrendArgs.days > 0 {
				out["days"] = balanceTrendArgs.days
			}
			if res.RecordCount == 0 {
				out["note"] = "暂无历史快照；每次 balance query 成功后自动记录，积累数日后可见趋势"
				out["daily_points"] = []interface{}{}
				return out, nil
			}
			out["first_record_time"] = api.FormatBeijing(res.FirstRecordTime, "2006-01-02 15:04")
			out["last_record_time"] = api.FormatBeijing(res.LastRecordTime, "2006-01-02 15:04")
			points := make([]map[string]interface{}, 0, len(res.DailyPoints))
			for _, p := range res.DailyPoints {
				points = append(points, map[string]interface{}{
					"date":    api.FormatBeijing(p.Time(), "2006-01-02"),
					"balance": p.Balance,
					"price":   p.Price,
				})
			}
			out["daily_points"] = points
			return out, nil
		})
	},
}

func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

func init() {
	userOfflineCmd.Flags().StringVar(&userOfflineArgs.deviceID, "device-id", "", "设备 ID（user devices 输出的 device_id）")
	userOfflineCmd.Flags().StringVar(&userOfflineArgs.ip, "ip", "", "设备 IP（user devices 输出的 ip）")

	userCmd.AddCommand(userInfoCmd, userLabelsCmd, userDevicesCmd, userOfflineCmd)

	q := balanceQueryCmd.Flags()
	q.IntVar(&balanceQueryArgs.type_, "type", 1, "查询类型：1 照明电费，2 空调电费")
	q.StringVar(&balanceQueryArgs.schoolCode, "school-code", "", "校区代码（balance campus 的 code；绑定时必填）")
	q.StringVar(&balanceQueryArgs.regCode, "reg-code", "", "楼栋代码（balance buildings 的 code；绑定时必填）")
	q.StringVar(&balanceQueryArgs.unitCode, "unit-code", "", "单元代码（balance units 的 code；无单元可空）")
	q.StringVar(&balanceQueryArgs.roomNo, "room", "", "房间号（提供时先绑定房间）")

	t := balanceTrendCmd.Flags()
	t.IntVar(&balanceTrendArgs.type_, "type", 1, "查询类型：1 照明电费，2 空调电费")
	t.IntVar(&balanceTrendArgs.days, "days", 0, "只统计最近 N 天的快照（0 为全部，保留期 365 天）")

	balanceCmd.AddCommand(balanceCampusCmd, balanceBuildingsCmd, balanceUnitsCmd, balanceQueryCmd, balanceTrendCmd)
}
