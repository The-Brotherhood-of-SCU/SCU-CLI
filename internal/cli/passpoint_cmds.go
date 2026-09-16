package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/output"
	"github.com/spf13/cobra"
)

// ─── passpoint 命令组（校园网无感认证） ──────────────────────────

func newNewService() (*api.NewServiceService, error) {
	a, err := newScuAuth()
	if err != nil {
		return nil, err
	}
	return api.NewNewServiceService(auth.NewNewServiceAuth(a)), nil
}

var passpointDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "获取无感认证设备列表（user_mac 供 cancel）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newNewService()
			if err != nil {
				return nil, err
			}
			return s.FetchPasspointDevices()
		})
	},
}

var passpointUserCmd = &cobra.Command{
	Use:   "user",
	Short: "获取校园网账户信息（account_state 1 为在线）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newNewService()
			if err != nil {
				return nil, err
			}
			return s.FetchPasspointUserInfo()
		})
	},
}

var passpointAddArgs struct {
	mac  string
	days int
	exit string
}

// passpointExitLabels 无感认证支持的出口（defaultServiceName 可选项）。
var passpointExitLabels = []string{"", "中国电信", "中国移动", "中国联通"}

// passpointMacRE newservice 后端要求的 MAC 格式：12 位十六进制、无分隔符、大写。
var passpointMacRE = regexp.MustCompile(`^[0-9A-F]{12}$`)

// normalizePasspointMac 归一化无感认证 MAC：去空格/冒号/连字符、转大写。
// 后端只接受 12 位无分隔符十六进制（如 B8782EBDCE85），与 Bugaoshan 输入约束一致。
func normalizePasspointMac(mac string) (string, error) {
	m := strings.ToUpper(strings.NewReplacer(" ", "", ":", "", "-", "").Replace(mac))
	if !passpointMacRE.MatchString(m) {
		return "", fmt.Errorf("--mac 格式应为 12 位十六进制（无分隔符），如 B8782EBDCE85")
	}
	return m, nil
}

var passpointAddCmd = &cobra.Command{
	Use:   "add --mac <MAC>",
	Short: "绑定设备无感认证（绑定 MAC 后连校园网自动完成认证）",
	Long: `添加无感设备。绑定设备 MAC 地址后，连接校园网自动完成认证，无需手动登录。
--mac 为设备 MAC 地址（12 位十六进制、无分隔符，如 B8782EBDCE85；冒号/连字符写法会自动归一化）；
--days 为有效期（0-365 天，0 表示最长有效期 6 年，默认 30）；
--exit 为出口运营商（校园网省略 / 中国电信 / 中国移动 / 中国联通，默认校园网）。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if passpointAddArgs.mac == "" {
			return output.Fail("input", fmt.Errorf("--mac 必填"))
		}
		mac, err := normalizePasspointMac(passpointAddArgs.mac)
		if err != nil {
			return output.Fail("input", err)
		}
		if passpointAddArgs.days < 0 || passpointAddArgs.days > 365 {
			return output.Fail("input", fmt.Errorf("--days 取值 0-365（0 为最长有效期 6 年）"))
		}
		validExit := false
		for _, e := range passpointExitLabels {
			if e == passpointAddArgs.exit {
				validExit = true
				break
			}
		}
		if !validExit {
			return output.Fail("input", fmt.Errorf("--exit 只支持 空（校园网）/ 中国电信 / 中国移动 / 中国联通"))
		}
		return runJSON(func() (interface{}, error) {
			s, err := newNewService()
			if err != nil {
				return nil, err
			}
			if err := s.AddPasspointDevice(mac, passpointAddArgs.days, passpointAddArgs.exit); err != nil {
				return nil, err
			}
			out := map[string]interface{}{
				"message":  "无感认证绑定成功，设备连接校园网后将自动认证",
				"user_mac": mac,
				"days":     passpointAddArgs.days,
			}
			if passpointAddArgs.exit != "" {
				out["exit"] = passpointAddArgs.exit
			}
			return out, nil
		})
	},
}

var passpointCancelArgs struct{ mac string }

var passpointCancelCmd = &cobra.Command{
	Use:   "cancel --mac <MAC>",
	Short: "取消指定设备的无感认证（mac 取自 devices 输出的 user_mac）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if passpointCancelArgs.mac == "" {
			return output.Fail("input", fmt.Errorf("--mac 必填（取自 passpoint devices 输出的 user_mac）"))
		}
		mac, err := normalizePasspointMac(passpointCancelArgs.mac)
		if err != nil {
			return output.Fail("input", err)
		}
		return runJSON(func() (interface{}, error) {
			s, err := newNewService()
			if err != nil {
				return nil, err
			}
			if err := s.CancelPasspointDevice(mac); err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"message":  "已取消该设备的无感认证",
				"user_mac": mac,
			}, nil
		})
	},
}

var passpointCmd = &cobra.Command{
	Use:   "passpoint",
	Short: "校园网无感认证（Passpoint MAC 绑定管理）",
}

func init() {
	a := passpointAddCmd.Flags()
	a.StringVar(&passpointAddArgs.mac, "mac", "", "设备 MAC 地址（12 位十六进制、无分隔符，如 B8782EBDCE85）")
	a.IntVar(&passpointAddArgs.days, "days", 30, "有效期天数（0-365，0 为最长有效期 6 年）")
	a.StringVar(&passpointAddArgs.exit, "exit", "", "出口运营商：省略为校园网 / 中国电信 / 中国移动 / 中国联通")
	passpointCancelCmd.Flags().StringVar(&passpointCancelArgs.mac, "mac", "", "设备 MAC 地址（passpoint devices 输出的 user_mac）")

	passpointCmd.AddCommand(passpointDevicesCmd, passpointUserCmd, passpointAddCmd, passpointCancelCmd)
}
