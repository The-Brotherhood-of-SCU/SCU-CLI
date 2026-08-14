package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/output"
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

	serviceSubmitCmd.Flags().StringVar(&serviceSubmitArgs.fields, "fields", "", "字段值 JSON 对象，键为 form 输出的 key，如 '{\"Radio_30\":\"1\",\"Calendar_32\":\"2026-08-14\"}'")
	serviceSubmitCmd.Flags().StringArrayVar(&serviceSubmitArgs.attach, "attach", nil, "附件，格式 字段Key=本地文件路径（可重复）")
	serviceSubmitCmd.Flags().BoolVar(&serviceSubmitArgs.dryRun, "dry-run", false, "只组装并输出提交体，不实际提交")

	serviceCmd.AddCommand(serviceApplicationsCmd, serviceFormCmd, serviceSubmitCmd)
}

// ─── 动态表单（事项办理） ────────────────────────────────────────

// serviceInputError 标记本地输入错误（输出 kind=input）。
type serviceInputError struct{ msg string }

func (e serviceInputError) Error() string { return e.msg }

func serviceInputErr(format string, args ...interface{}) error {
	return serviceInputError{fmt.Sprintf(format, args...)}
}

// loadServiceForm 完成 select-department → start-data → start-info → get-formv → schema 全链。
func loadServiceForm(s *api.ServiceHallService, appID string) (*api.FormSchema, string, error) {
	starterDepartID, err := s.FetchStarterDepartID(appID)
	if err != nil {
		return nil, "", err
	}
	def, err := s.FetchStartData(appID, starterDepartID)
	if err != nil {
		return nil, "", err
	}
	bpmnID, err := s.FetchStartInfo(appID)
	if err != nil {
		return nil, "", err
	}
	formvD, err := s.FetchFormPlugins(def.FormID, bpmnID, starterDepartID)
	if err != nil {
		return nil, "", err
	}
	schema, err := api.BuildFormSchema(appID, formvD, *def)
	if err != nil {
		return nil, "", err
	}
	return schema, starterDepartID, nil
}

var serviceFormCmd = &cobra.Command{
	Use:   "form <app_id>",
	Short: "查看事项表单结构（字段 / 类型 / 选项 / 必填 / 预填值）",
	Long: `拉取办事大厅事项的表单 schema。app_id 来自 service applications 或事项页面 URL。
输出的 fields 按填写顺序排列：key（提交字段名）、label（中文标签）、type、required、
options（选项 value/name，radio/select/selectV2/checkbox 的合法取值）、prefilled（服务端预填值）。
type 取值：input/multiInput 文本、radio/select/selectV2 单选、checkbox 多选、calendar 日期、
region 省市区、file 附件、dataSource 联动取数、user 只读人员。date_rules 为日期顺序校验
（first_key 必须早于等于 second_key）。布局/逻辑类字段（说明文字等）不在列表中。`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newServiceHallService()
			if err != nil {
				return nil, err
			}
			schema, starterDepartID, err := loadServiceForm(s, args[0])
			if err != nil {
				return nil, err
			}
			fields := make([]map[string]interface{}, 0, len(schema.Plugins))
			for _, p := range schema.Plugins {
				if placeholderDumpSkip[p.Type] {
					continue
				}
				f := map[string]interface{}{
					"key":        p.Key,
					"label":      p.Label,
					"type":       string(p.Type),
					"permission": schema.Auth[p.Key],
					"required":   schema.IsRequired(p.Key),
					"readonly":   schema.IsReadOnly(p.Key),
					"suppressed": schema.IsSuppressed(p.Key),
				}
				if len(p.Options) > 0 {
					f["options"] = p.Options
				}
				if v, ok := schema.Data[p.Key]; ok && v != nil && fmt.Sprint(v) != "" {
					f["prefilled"] = v
				}
				if p.Hint != "" {
					f["hint"] = p.Hint
				}
				if p.Type == api.FieldFile {
					f["max_count"] = p.MaxCount
				}
				if p.DataSource != nil {
					f["data_source"] = map[string]interface{}{
						"id":         p.DataSource.ID,
						"result_key": p.DataSource.ResultKey,
					}
				}
				fields = append(fields, f)
			}
			return map[string]interface{}{
				"app_id":            schema.AppID,
				"form_id":           schema.FormID,
				"form_version_id":   schema.FormVersionID,
				"starter_depart_id": starterDepartID,
				"fields":            fields,
				"date_rules":        schema.DateOrderRules(),
			}, nil
		})
	},
}

// placeholderDumpSkip 布局/逻辑类字段不在 form 输出中展示。
var placeholderDumpSkip = map[api.ServiceFieldType]bool{
	api.FieldShowHide: true, api.FieldVariate: true, api.FieldValidate: true,
	api.FieldConversion: true, api.FieldRepeatTable: true, api.FieldText: true,
	api.FieldImage: true, api.FieldTable: true, api.FieldUnknown: true,
}

var serviceSubmitArgs struct {
	fields string
	attach []string
	dryRun bool
}

var serviceSubmitCmd = &cobra.Command{
	Use:   "submit <app_id>",
	Short: "填写并提交办事大厅事项（--fields JSON / --attach 附件 / --dry-run 预览）",
	Long: `填写并提交办事大厅事项。流程：service form <app_id> 查看字段 → 组装 --fields JSON → 提交。

--fields 是 JSON 对象，键为 form 输出的 key，值按 type：
  input/multiInput/dataSource   字符串
  radio/select/selectV2         选项的 value 字符串
  checkbox                      value 字符串数组 ["1","2"]
  calendar                      "2026-08-14" 或 "2026-08-14 10:00"
  region                        {"province":"四川省","city":"成都市","area":"武侯区","details":"详细地址"}
附件字段用 --attach File_71=./照片.png（可重复，上限见 form 输出 max_count）。
只读/隐藏/user 字段由服务端透传，不要提供。DataSource 字段提供名称后会自动取数联动。
服务端必填校验、ShowHide 动态显隐/必填、日期顺序校验均在提交前本地执行。
--dry-run 输出完整提交体而不发送，建议先用它确认。`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := runServiceSubmit(args[0])
		if err != nil {
			if ie, ok := err.(serviceInputError); ok {
				return output.Fail("input", ie)
			}
			return output.Fail(errorKind(err), err)
		}
		return output.JSON(data)
	},
}

func runServiceSubmit(appID string) (interface{}, error) {
	s, err := newServiceHallService()
	if err != nil {
		return nil, err
	}
	schema, starterDepartID, err := loadServiceForm(s, appID)
	if err != nil {
		return nil, err
	}
	st := api.NewFormState(schema)

	// 1) --fields 用户值（含类型校验与转换）
	userKeys, err := applyServiceFields(s, schema, st)
	if err != nil {
		return nil, err
	}

	// 2) --attach 附件上传
	if err := uploadServiceAttachments(s, schema, st, appID); err != nil {
		return nil, err
	}

	// 3) DataSource 联动取数（失败降级为仅提交名称并告警）
	var warnings []string
	for _, p := range schema.DataSourcePlugins() {
		name, _ := st.Values[p.Key].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		listValue, err := s.FetchDataSourceValue(appID, p.DataSource, starterDepartID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("数据源 %s 取数失败（仅提交名称）: %v", p.Key, err))
			continue
		}
		st.ApplyDataSourceValue(p, listValue)
	}

	// 4) ShowHide：被动态隐藏的字段若用户提供了值，按占位提交并告警
	for _, key := range userKeys {
		if p := schema.PluginByKey(key); p != nil && !st.IsFieldVisible(*p) {
			warnings = append(warnings, fmt.Sprintf("字段 %s 当前为隐藏状态，已按占位值提交", key))
		}
	}

	// 5) 必填 + 日期顺序校验
	if err := st.Validate(); err != nil {
		return nil, err
	}

	formData := st.BuildFormData()
	out := map[string]interface{}{
		"app_id":  appID,
		"form_id": schema.FormID,
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	if serviceSubmitArgs.dryRun {
		out["dry_run"] = true
		out["form_data"] = formData
		return out, nil
	}
	if err := s.SubmitMatter(appID, starterDepartID, formData); err != nil {
		return nil, err
	}
	out["submitted"] = true
	out["message"] = "事项提交成功"
	return out, nil
}

// applyServiceFields 解析 --fields JSON 并写入 FormState，返回用户提供过值的字段 key。
func applyServiceFields(s *api.ServiceHallService, schema *api.FormSchema, st *api.FormState) ([]string, error) {
	raw := strings.TrimSpace(serviceSubmitArgs.fields)
	if raw == "" {
		return nil, nil
	}
	var values map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, serviceInputErr("--fields 不是合法 JSON: %v", err)
	}
	var keys []string
	for key, v := range values {
		p := schema.PluginByKey(key)
		if p == nil {
			return nil, serviceInputErr("未知字段 %q（先用 service form %s 查看字段清单）", key, schema.AppID)
		}
		if placeholderDumpSkip[p.Type] {
			return nil, serviceInputErr("字段 %s（类型 %s）为布局/逻辑字段，不可填写", key, p.Type)
		}
		if p.Type == api.FieldUser {
			return nil, serviceInputErr("字段 %s 为人员字段，由服务端透传，不可填写", key)
		}
		if schema.IsReadOnly(key) || schema.IsSuppressed(key) {
			return nil, serviceInputErr("字段 %s 为只读/隐藏（%s），不可填写", key, schema.Auth[key])
		}
		coerced, err := coerceServiceFieldValue(s, p, v)
		if err != nil {
			return nil, err
		}
		st.Values[key] = coerced
		keys = append(keys, key)
	}
	return keys, nil
}

// coerceServiceFieldValue 按字段类型校验并转换用户输入。
func coerceServiceFieldValue(s *api.ServiceHallService, p *api.ServicePlugin, raw interface{}) (interface{}, error) {
	optionValues := func() ([]string, string) {
		var vals, labels []string
		for _, o := range p.Options {
			vals = append(vals, o.Value)
			labels = append(labels, fmt.Sprintf("%s(%s)", o.Value, o.Name))
		}
		return vals, strings.Join(labels, ", ")
	}
	checkOption := func(v string) error {
		if len(p.Options) == 0 {
			return nil
		}
		vals, labels := optionValues()
		for _, ok := range vals {
			if ok == v {
				return nil
			}
		}
		return serviceInputErr("字段 %s 的值 %q 不在选项中（可选：%s）", p.Key, v, labels)
	}

	switch p.Type {
	case api.FieldInput, api.FieldMultiInput, api.FieldDataSource:
		v, ok := raw.(string)
		if !ok {
			return nil, serviceInputErr("字段 %s（%s）需要字符串值", p.Key, p.Type)
		}
		return v, nil
	case api.FieldRadio, api.FieldSelect, api.FieldSelectV2:
		v, ok := raw.(string)
		if !ok {
			return nil, serviceInputErr("字段 %s（%s）需要选项 value 字符串", p.Key, p.Type)
		}
		if err := checkOption(v); err != nil {
			return nil, err
		}
		return v, nil
	case api.FieldCheckbox:
		arr, ok := raw.([]interface{})
		if !ok {
			return nil, serviceInputErr("字段 %s（checkbox）需要 value 字符串数组", p.Key)
		}
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			v, ok := item.(string)
			if !ok {
				return nil, serviceInputErr("字段 %s（checkbox）数组元素必须是字符串", p.Key)
			}
			if err := checkOption(v); err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case api.FieldCalendar:
		v, ok := raw.(string)
		if !ok {
			return nil, serviceInputErr("字段 %s（calendar）需要日期字符串", p.Key)
		}
		t := api.ParseServiceDateTimeValue(v)
		if t == nil {
			return nil, serviceInputErr("字段 %s 日期格式无法解析: %q（应为 2006-01-02 或 2006-01-02 15:04）", p.Key, v)
		}
		return *t, nil
	case api.FieldRegion:
		m, ok := raw.(map[string]interface{})
		if !ok {
			return nil, serviceInputErr("字段 %s（region）需要对象 {province,city,area,details}", p.Key)
		}
		sel := map[string]string{}
		for _, k := range []string{"province", "city", "area", "details"} {
			if v, ok := m[k].(string); ok {
				sel[k] = v
			}
		}
		if sel["province"] == "" {
			return nil, serviceInputErr("字段 %s（region）缺少 province", p.Key)
		}
		return s.ResolveRegion(sel)
	case api.FieldFile:
		return nil, serviceInputErr("字段 %s 是附件字段，请用 --attach %s=<本地文件路径>", p.Key, p.Key)
	default:
		return nil, serviceInputErr("字段 %s（类型 %s）不支持填写", p.Key, p.Type)
	}
}

// uploadServiceAttachments 处理 --attach Key=path 列表：上传并写入 FormState。
func uploadServiceAttachments(s *api.ServiceHallService, schema *api.FormSchema, st *api.FormState, appID string) error {
	for _, spec := range serviceSubmitArgs.attach {
		i := strings.Index(spec, "=")
		if i <= 0 || i == len(spec)-1 {
			return serviceInputErr("--attach 格式为 字段Key=本地文件路径，收到 %q", spec)
		}
		key, path := spec[:i], spec[i+1:]
		p := schema.PluginByKey(key)
		if p == nil {
			return serviceInputErr("未知附件字段 %q（先用 service form %s 查看字段清单）", key, appID)
		}
		if p.Type != api.FieldFile {
			return serviceInputErr("字段 %s（类型 %s）不是附件字段", key, p.Type)
		}
		att, err := s.UploadAttachment(appID, path)
		if err != nil {
			return err
		}
		existing, _ := st.Values[key].([]api.ServiceAttachment)
		existing = append(existing, *att)
		if len(existing) > p.MaxCount {
			return serviceInputErr("字段 %s 附件超过上限 %d 个", key, p.MaxCount)
		}
		st.Values[key] = existing
		output.Info("已上传附件 %s → %s", path, att.Name)
	}
	return nil
}
