package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/output"
	"github.com/spf13/cobra"
)

// ─── repair 命令组（智慧后勤在线报修） ───────────────────────────

func newZhhqService() (*api.ZhhqService, error) {
	a, err := newScuAuth()
	if err != nil {
		return nil, err
	}
	return api.NewZhhqService(auth.NewZhhqAuth(a)), nil
}

// repairInputError 标记本地输入错误（输出 kind=input）。
type repairInputError struct{ msg string }

func (e repairInputError) Error() string { return e.msg }

func repairInputErr(format string, args ...interface{}) error {
	return repairInputError{fmt.Sprintf(format, args...)}
}

var repairAddressesCmd = &cobra.Command{
	Use:   "addresses",
	Short: "获取常用报修地址（id 供 submit --address-id，user_id 供 list）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			return s.FetchAddresses()
		})
	},
}

var repairAreasCmd = &cobra.Command{
	Use:   "areas",
	Short: "获取报修区域树（id 供 projects / save-address --area-id，full_name 供 --area-name）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			return s.FetchAreaTree()
		})
	},
}

var repairProjectsCmd = &cobra.Command{
	Use:   "projects <areaId>",
	Short: "按区域获取维修项目两级树（叶子 value 供 submit --project）",
	Long: `获取维修项目（两级树：顶层大类如「水/木/泥」，children 为具体项目）。
areaId 取自 repair areas 输出（或常用地址的 area_id）。
submit --project 必须取叶子节点的 value（如 101），提交时自动拼「大类/项目」全名。`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			return s.FetchProjects(args[0])
		})
	},
}

var repairBookDatesCmd = &cobra.Command{
	Use:   "book-dates",
	Short: "获取可预约上门日期（供 submit --book-date）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			return s.FetchBookDates()
		})
	},
}

var repairBookTimesCmd = &cobra.Command{
	Use:   "book-times <bookDate>",
	Short: "获取某日期的可预约时段（供 submit --book-time）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			return s.FetchBookTimes(args[0])
		})
	},
}

var repairListArgs struct{ userID string }

var repairListCmd = &cobra.Command{
	Use:   "list [--user-id]",
	Short: "我的报修工单列表（status 为中文状态：待完工/待评价/已关闭/已撤回）",
	Long: `获取「我的动态」报修工单列表，按时间倒序。
--user-id 省略时自动取常用地址（repair addresses）第一项的 user_id。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			userID := repairListArgs.userID
			if userID == "" {
				addresses, err := s.FetchAddresses()
				if err != nil {
					return nil, err
				}
				if len(addresses) == 0 {
					return nil, &auth.ServiceError{Msg: "无常用地址可获取 user_id，请先 repair save-address 或用 --user-id 指定"}
				}
				userID = addresses[0].UserID
			}
			return s.FetchDynamicTickets(userID)
		})
	},
}

var repairDetailCmd = &cobra.Command{
	Use:   "detail <id>",
	Short: "获取报修工单详情（id 取自 list 输出；logs 为进度时间线）",
	Long: `获取工单详情。id 取自 repair list 输出的 id 字段（activeId）。
待评价工单的评价对象 id 在 finished_info.repair_id（供 repair evaluate 使用）。`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			return s.FetchRepairDetail(args[0])
		})
	},
}

var repairWithdrawCmd = &cobra.Command{
	Use:   "withdraw <id>",
	Short: "撤回报修工单（id 取自 list 输出；仅待完工状态可撤回）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			allowed, err := s.IfAllowWithdrawRepair(args[0])
			if err != nil {
				return nil, err
			}
			if !allowed {
				return nil, &auth.ServiceError{Msg: "当前工单状态不允许撤回（仅待完工可撤回）"}
			}
			if err := s.WithdrawRepair(args[0]); err != nil {
				return nil, err
			}
			return map[string]interface{}{"id": args[0], "message": "工单已撤回"}, nil
		})
	},
}

var repairEvaluateProjectsCmd = &cobra.Command{
	Use:   "evaluate-projects",
	Short: "获取工单评价项（维修质量/态度/速度等，id 供 evaluate --stars）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			return s.FetchEvaluateProjects()
		})
	},
}

var repairEvaluateArgs struct {
	star    int
	stars   []string
	content string
	labels  string
}

var repairEvaluateCmd = &cobra.Command{
	Use:   "evaluate <repairId>",
	Short: "评价报修工单（repairId 取自 detail 输出的 finished_info.repair_id）",
	Long: `评价已完工工单。repairId 不是工单 id，而是 detail 输出中
finished_info.repair_id。--star 为统一评分（1-5，应用于全部评价项）；
--stars 可按评价项覆盖（格式 评价项id:分数，逗号分隔，id 见 evaluate-projects）；
--content 为文字评价，--labels 为标签（逗号分隔）。`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if repairEvaluateArgs.star < 1 || repairEvaluateArgs.star > 5 {
			return output.Fail("input", fmt.Errorf("--star 必须为 1-5"))
		}
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			projects, err := s.FetchEvaluateProjects()
			if err != nil {
				return nil, err
			}
			stars := map[string]int{}
			for _, spec := range repairEvaluateArgs.stars {
				i := strings.Index(spec, ":")
				if i <= 0 || i == len(spec)-1 {
					return nil, repairInputErr("--stars 格式为 评价项id:分数（逗号分隔），收到 %q", spec)
				}
				var star int
				if _, err := fmt.Sscanf(spec[i+1:], "%d", &star); err != nil || star < 1 || star > 5 {
					return nil, repairInputErr("--stars 分数必须为 1-5，收到 %q", spec)
				}
				stars[spec[:i]] = star
			}
			input := api.RepairEvaluateInput{
				RepairID: args[0],
				Projects: projects,
				Stars:    stars,
				Content:  repairEvaluateArgs.content,
			}
			if repairEvaluateArgs.labels != "" {
				input.Labels = strings.Split(repairEvaluateArgs.labels, ",")
			}
			if err := s.EvaluateRepair(input); err != nil {
				return nil, err
			}
			return map[string]interface{}{"repair_id": args[0], "message": "评价提交成功"}, nil
		})
	},
}

var repairSaveAddressArgs struct {
	areaID, areaName, detail, phone, name string
	isCommon                              bool
}

var repairSaveAddressCmd = &cobra.Command{
	Use:   "save-address --area-id <id> --area-name <name> --detail <text> --phone <mobile>",
	Short: "保存常用报修地址（area-id/area-name 取自 repair areas）",
	Long: `新增常用报修地址。area-id 与 area-name（full_name，如 望江学生区/东苑五栋）
取自 repair areas 输出；--detail 为楼栋房间号等详细地址；--phone 为联系电话。
--default 设为默认地址。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		a := repairSaveAddressArgs
		if a.areaID == "" || a.areaName == "" || a.detail == "" || a.phone == "" {
			return output.Fail("input", fmt.Errorf("--area-id/--area-name/--detail/--phone 均为必填"))
		}
		return runJSON(func() (interface{}, error) {
			s, err := newZhhqService()
			if err != nil {
				return nil, err
			}
			if err := s.SaveAddress(a.areaID, a.areaName, a.detail, a.phone, a.name, a.isCommon); err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "地址保存成功"}, nil
		})
	},
}

// repairSubmitArgs 提交工单参数。
var repairSubmitArgs struct {
	addressID   string
	project     string
	content     string
	bookDate    string
	bookTime    string
	images      []string
	allowAbsent bool
	dryRun      bool
}

// repairImageExts 图片格式白名单（与 zhhq 上传接口支持的格式一致）。
var repairImageExts = map[string]bool{"jpg": true, "jpeg": true, "png": true, "heic": true, "heif": true}

const repairMaxImageBytes = 10 * 1024 * 1024

var repairSubmitCmd = &cobra.Command{
	Use:   "submit --address-id <id> --project <projectId> --content <text>",
	Short: "提交报修工单（--image 可重复附图，--dry-run 预览提交体）",
	Long: `提交宿舍报修工单。参数发现链：
  --address-id  repair addresses 输出的 id（若为空先 repair save-address）
  --project     repair projects <area_id> 输出的叶子项目 value
  --book-date   repair book-dates 输出（可省略）
  --book-time   repair book-times <date> 输出（可省略；提供时 --book-date 必填）
  --image       本地图片路径（jpg/jpeg/png/heic/heif，单个 ≤10MB，最多 3 张，可重复）
  --allow-absent 允许无人时上门维修

提交流程与 App 一致：上传图片 → 按区域+项目预取负责部门（收费/受理单位）
→ 组装并提交。--dry-run 只输出提交体不发送。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := runRepairSubmit()
		if err != nil {
			if ie, ok := err.(repairInputError); ok {
				return output.Fail("input", ie)
			}
			return output.Fail(errorKind(err), err)
		}
		return output.JSON(data)
	},
}

func runRepairSubmit() (interface{}, error) {
	a := repairSubmitArgs
	if a.addressID == "" {
		return nil, repairInputErr("--address-id 必填（取自 repair addresses 输出）")
	}
	if a.project == "" {
		return nil, repairInputErr("--project 必填（取自 repair projects <area_id> 输出的叶子 value）")
	}
	if strings.TrimSpace(a.content) == "" {
		return nil, repairInputErr("--content（故障描述）必填")
	}
	if a.bookTime != "" && a.bookDate == "" {
		return nil, repairInputErr("提供 --book-time 时必须同时提供 --book-date")
	}
	if len(a.images) > 3 {
		return nil, repairInputErr("图片最多 3 张")
	}

	s, err := newZhhqService()
	if err != nil {
		return nil, err
	}

	// 1) 地址：id → areaId/areaName/addressDetail/phone。
	addresses, err := s.FetchAddresses()
	if err != nil {
		return nil, err
	}
	var address *api.RepairAddress
	for i := range addresses {
		if addresses[i].ID == a.addressID {
			address = &addresses[i]
			break
		}
	}
	if address == nil {
		return nil, repairInputErr("地址 %q 不存在（用 repair addresses 查看现有地址）", a.addressID)
	}

	// 2) 项目：叶子 value → 全名（大类/项目）。
	projects, err := s.FetchProjects(address.AreaID)
	if err != nil {
		return nil, err
	}
	leaf, projectLabel := api.FindRepairProjectLeaf(projects, a.project)
	if leaf == nil {
		return nil, repairInputErr("项目 %q 不在区域 %s 的项目树中（用 repair projects %s 查看，取叶子 value）", a.project, address.AreaID, address.AreaID)
	}

	// 3) 上传图片。
	resources := make([]map[string]interface{}, 0, len(a.images))
	for _, path := range a.images {
		url, err := uploadRepairImage(s, path)
		if err != nil {
			return nil, err
		}
		resources = append(resources, map[string]interface{}{"fileUrl": url, "fileType": "1", "statusType": "1"})
		output.Info("已上传图片 %s", filepath.Base(path))
	}

	// 4) 预取维修负责部门（acceptDept*/payName 来源）。
	var warnings []string
	dept, err := s.FetchAcceptDept(address.AreaID, a.project)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("预取负责部门失败（不影响提交）: %v", err))
	}

	// 5) 报修人姓名取微服务用户信息（与 App 的 userRealname 一致）。
	scu, err := newScuAuth()
	if err != nil {
		return nil, err
	}
	profile, err := api.NewWfwService(auth.NewWfwAuth(scu)).FetchUserProfile()
	if err != nil {
		return nil, err
	}
	realname, _ := profile["realname"].(string)

	ifOnduty := "0"
	if a.allowAbsent {
		ifOnduty = "1"
	}
	payload := map[string]interface{}{
		// 与前端提交链完全一致（缺字段服务端会报"缺少参数"）。
		"type":             "0",
		"ifShielding":      "1",
		"areaName":         address.AreaName,
		"address":          address.AddressDetail,
		"projectName":      projectLabel,
		"bookDate":         a.bookDate,
		"bookTime":         a.bookTime,
		"repairUserName":   realname,
		"repairUserMobile": address.Phone,
		"repairDeptName":   "",
		"ifOnduty":         ifOnduty,
		"repairNum":        1,
		"content":          strings.TrimSpace(a.content),
		"ifPublish":        "1",
		"ifUrgent":         "0",
		"id":               "",
		"resourcesVOS":     resources,
		"source":           "1",
		"projectId":        a.project,
		"areaId":           address.AreaID,
		"ifRecord":         0,
	}
	if dept != nil {
		payload["payName"] = dept.PayName
		payload["acceptDeptId"] = dept.DeptID
		payload["acceptDeptName"] = dept.DeptName
	} else {
		warnings = append(warnings, "未取到负责部门信息，提交体缺 payName/acceptDept*（服务端可能拒单）")
	}

	out := map[string]interface{}{"address": address.AreaName + " " + address.AddressDetail, "project": projectLabel}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	if a.dryRun {
		out["dry_run"] = true
		out["payload"] = payload
		return out, nil
	}
	if err := s.SubmitTicket(payload); err != nil {
		return nil, err
	}
	out["submitted"] = true
	out["message"] = "报修工单提交成功"
	return out, nil
}

// uploadRepairImage 校验并上传单张图片（格式白名单 + 10MB 上限）。
func uploadRepairImage(s *api.ZhhqService, path string) (string, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if !repairImageExts[ext] {
		return "", repairInputErr("图片 %s 格式不支持（仅 jpg/jpeg/png/heic/heif）", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", repairInputErr("图片 %s 无法读取: %v", path, err)
	}
	if info.Size() > repairMaxImageBytes {
		return "", repairInputErr("图片 %s 超过 10MB 上限", path)
	}
	return s.UploadRepairImage(path)
}

// ─── 命令组注册 ──────────────────────────────────────────────────

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "智慧后勤在线报修（工单提交 / 进度 / 撤回 / 评价）",
	Long: `智慧后勤（zhhq.scu.edu.cn）在线报修。各 ID 的发现链：

  address-id  ← repair addresses 输出的 id（为空先 repair save-address）
  project     ← repair projects <area_id> 输出的叶子项目 value
  book-date   ← repair book-dates 输出
  book-time   ← repair book-times <book-date> 输出
  工单 id      ← repair list 输出的 id
  repairId    ← repair detail <工单id> 输出的 finished_info.repair_id`,
}

func init() {
	repairListCmd.Flags().StringVar(&repairListArgs.userID, "user-id", "", "用户 id（省略取常用地址的 user_id）")

	e := repairEvaluateCmd.Flags()
	e.IntVar(&repairEvaluateArgs.star, "star", 5, "统一评分（1-5，应用于全部评价项）")
	e.StringArrayVar(&repairEvaluateArgs.stars, "stars", nil, "按评价项覆盖评分，格式 评价项id:分数（可重复）")
	e.StringVar(&repairEvaluateArgs.content, "content", "", "文字评价（可空）")
	e.StringVar(&repairEvaluateArgs.labels, "labels", "", "评价标签，逗号分隔（可空）")

	sa := repairSaveAddressCmd.Flags()
	sa.StringVar(&repairSaveAddressArgs.areaID, "area-id", "", "区域 id（repair areas 输出）")
	sa.StringVar(&repairSaveAddressArgs.areaName, "area-name", "", "区域全名（repair areas 的 full_name）")
	sa.StringVar(&repairSaveAddressArgs.detail, "detail", "", "详细地址（楼栋/房间号）")
	sa.StringVar(&repairSaveAddressArgs.phone, "phone", "", "联系电话")
	sa.StringVar(&repairSaveAddressArgs.name, "name", "", "联系人姓名（可空）")
	sa.BoolVar(&repairSaveAddressArgs.isCommon, "default", false, "设为默认地址")

	s := repairSubmitCmd.Flags()
	s.StringVar(&repairSubmitArgs.addressID, "address-id", "", "常用地址 id（repair addresses 输出）")
	s.StringVar(&repairSubmitArgs.project, "project", "", "维修项目 id（repair projects 输出的叶子 value）")
	s.StringVar(&repairSubmitArgs.content, "content", "", "故障描述")
	s.StringVar(&repairSubmitArgs.bookDate, "book-date", "", "预约上门日期（repair book-dates 输出；可空）")
	s.StringVar(&repairSubmitArgs.bookTime, "book-time", "", "预约时段（repair book-times 输出；可空）")
	s.StringArrayVar(&repairSubmitArgs.images, "image", nil, "现场照片路径（可重复，最多 3 张，单张 ≤10MB）")
	s.BoolVar(&repairSubmitArgs.allowAbsent, "allow-absent", false, "允许无人时上门维修")
	s.BoolVar(&repairSubmitArgs.dryRun, "dry-run", false, "只组装并输出提交体，不实际提交")

	repairCmd.AddCommand(repairAddressesCmd, repairAreasCmd, repairProjectsCmd,
		repairBookDatesCmd, repairBookTimesCmd, repairListCmd, repairDetailCmd,
		repairWithdrawCmd, repairEvaluateProjectsCmd, repairEvaluateCmd,
		repairSaveAddressCmd, repairSubmitCmd)
}
