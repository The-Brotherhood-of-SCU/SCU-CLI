package api

// 办事大厅动态表单引擎（移植自 Bugaoshan service_plugin_models.dart /
// service_form_models.dart / service_form_controller.dart，结构经真实抓包校准）。
//
// 数据流：start-data（权限 auth + 预填 data + 当前表单 currform）
//        + get-formv（插件定义，双层 JSON 编码）
//        → FormSchema（字段类型/选项/标签/排序/数据源/ShowHide/Validate）
//        → FormState（值容器：预填播种 + 用户覆盖 + DataSource 取数）
//        → BuildFormData（类型感知序列化 + ShowHide 显隐 + 占位规则）。

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// ─── 字段类型 ────────────────────────────────────────────────────

type ServiceFieldType string

const (
	FieldInput       ServiceFieldType = "input"
	FieldMultiInput  ServiceFieldType = "multiInput"
	FieldRadio       ServiceFieldType = "radio"
	FieldSelect      ServiceFieldType = "select"
	FieldSelectV2    ServiceFieldType = "selectV2" // 新下拉：提交一律数组（单选也是单元素数组）
	FieldCheckbox    ServiceFieldType = "checkbox"
	FieldCalendar    ServiceFieldType = "calendar"
	FieldRegion      ServiceFieldType = "region"
	FieldFile        ServiceFieldType = "file" // dFile / dXImage：提交 [{name,url,id}]
	FieldDataSource  ServiceFieldType = "dataSource"
	FieldUser        ServiceFieldType = "user"
	FieldShowHide    ServiceFieldType = "showHide"
	FieldVariate     ServiceFieldType = "variate"
	FieldValidate    ServiceFieldType = "validate"
	FieldConversion  ServiceFieldType = "conversion"
	FieldRepeatTable ServiceFieldType = "repeatTable"
	FieldText        ServiceFieldType = "text"  // 静态说明文字（dOneInput / Text_*）
	FieldImage       ServiceFieldType = "image" // 静态图片（dImage）
	FieldTable       ServiceFieldType = "table" // 布局容器（dTable）
	FieldUnknown     ServiceFieldType = "unknown"
)

// key 前缀 → 类型（key 形如 Radio_30，前缀与组件名一一对应）。
var serviceFieldPrefixTypes = []struct {
	prefix string
	typ    ServiceFieldType
}{
	{"MultiInput_", FieldMultiInput},
	{"MultiText_", FieldMultiInput},
	{"SelectV2_", FieldSelectV2},
	{"RepeatTable_", FieldRepeatTable},
	{"DataSource_", FieldDataSource},
	{"Conversion_", FieldConversion},
	{"ShowHide_", FieldShowHide},
	{"Checkbox_", FieldCheckbox},
	{"Calendar_", FieldCalendar},
	{"Validate_", FieldValidate},
	{"Variate_", FieldVariate},
	{"Select_", FieldSelect},
	{"Region_", FieldRegion},
	{"Input_", FieldInput},
	{"Radio_", FieldRadio},
	{"File_", FieldFile},
	{"Ximage_", FieldFile},
	{"User_", FieldUser},
	{"Text_", FieldText},
	{"Image_", FieldImage},
	{"Table_", FieldTable},
}

// 组件名（去 d 前缀、小写）→ 类型。来自真实抓包的组件清单。
var serviceComponentTypes = map[string]ServiceFieldType{
	"input": FieldInput, "integerinput": FieldInput, "numericinput": FieldInput,
	"phonenumber": FieldInput, "multitext": FieldMultiInput, "multiinputs": FieldMultiInput,
	"radio": FieldRadio, "select": FieldSelect, "selectv2": FieldSelectV2,
	"checkbox": FieldCheckbox, "calendar": FieldCalendar, "region": FieldRegion,
	"file": FieldFile, "ximage": FieldFile, "datasource": FieldDataSource,
	"user": FieldUser, "showhide": FieldShowHide, "variate": FieldVariate,
	"validate": FieldValidate, "conversion": FieldConversion, "repeattable": FieldRepeatTable,
	"oneinput": FieldText, "text": FieldText, "show": FieldText,
	"image": FieldImage, "table": FieldTable,
}

// serviceFieldTypeFromKey 按 key 前缀推断类型。
func serviceFieldTypeFromKey(key string) ServiceFieldType {
	for _, p := range serviceFieldPrefixTypes {
		if strings.HasPrefix(key, p.prefix) {
			return p.typ
		}
	}
	return FieldUnknown
}

// resolveServiceFieldType 优先组件声明（如 dRadio，大小写不敏感），缺失按 key 前缀推断。
func resolveServiceFieldType(declaredType, key string) ServiceFieldType {
	if declaredType != "" {
		name := strings.ToLower(strings.TrimSpace(declaredType))
		if strings.HasPrefix(name, "d") && len(name) > 1 {
			name = name[1:]
		}
		if t, ok := serviceComponentTypes[name]; ok {
			return t
		}
	}
	return serviceFieldTypeFromKey(key)
}

// ─── 小工具 ──────────────────────────────────────────────────────

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			if v != "" {
				return v
			}
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
	}
	return ""
}

func toIntLoose(v interface{}, fallback int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i
		}
	}
	return fallback
}

// maybeJSONString 二次解码：服务端常把对象编码成 JSON 字符串。
func maybeJSONString(v interface{}) interface{} {
	if s, ok := v.(string); ok {
		var decoded interface{}
		if err := json.Unmarshal([]byte(s), &decoded); err == nil {
			return decoded
		}
		return nil
	}
	return v
}

func toStringMap(v interface{}) map[string]interface{} {
	if m, ok := maybeJSONString(v).(map[string]interface{}); ok {
		return m
	}
	return nil
}

// ─── 插件模型 ────────────────────────────────────────────────────

// ServiceFieldOption 是 Radio/Select/SelectV2/Checkbox 的选项 {value, name}。
type ServiceFieldOption struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

// ServiceDataSourceRef 是 DataSource 字段的取数配置（POST /site/data-source/detail）。
type ServiceDataSourceRef struct {
	ID            string            `json:"id"`             // attr.data.sourceid
	FormVersionID string            `json:"form_versionId"` // form 的 version_id
	Component     string            `json:"component"`      // 插件 key，如 DataSource_85
	FormID        string            `json:"form_id"`
	ResultKey     string            `json:"result_key"` // 配对字段；特殊值 setplugin 表示按 MapConfig 分发
	MapConfig     map[string]string `json:"map_config"` // setplugin 模式：目标字段 key → 结果列名
	Configure     map[string]string `json:"configure"`  // sourceConfig 附加参数
}

// IsSetPlugin 是否 setplugin 分发模式。
func (r *ServiceDataSourceRef) IsSetPlugin() bool { return r.ResultKey == "setplugin" }

// ServiceDateOrderRule 是 Validate 插件的日期顺序校验：
// rule 形态 {f_dateDayMinus}({p_A},{p_B})<0，语义 A 必须早于等于 B，alert 为提示文案。
type ServiceDateOrderRule struct {
	FirstKey  string `json:"first_key"`
	SecondKey string `json:"second_key"`
	Message   string `json:"message"`
}

var dateOrderRuleRe = regexp.MustCompile(
	`^\s*\{f_dateDayMinus\}\(\s*\{p_([A-Za-z0-9_]+)\}\s*,\s*\{p_([A-Za-z0-9_]+)\}\s*\)\s*<\s*0\s*$`)

func tryParseDateOrderRule(rule, alert interface{}) *ServiceDateOrderRule {
	rs, ok := rule.(string)
	if !ok || rs == "" {
		return nil
	}
	m := dateOrderRuleRe.FindStringSubmatch(rs)
	if m == nil {
		return nil
	}
	msg, _ := alert.(string)
	return &ServiceDateOrderRule{FirstKey: m[1], SecondKey: m[2], Message: msg}
}

// ShowHideCondition 是 ShowHide 插件的一个条件（attr.data.conditions[i]）。
type ServiceShowHideCondition struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

// ShowHideControl 是条件命中后的动作（attr.data.controls[i].setInfo）。
type ServiceShowHideControl struct {
	IsShow          *bool    `json:"is_show,omitempty"`     // 1→显示 0→隐藏 nil→不动
	IsRequired      *bool    `json:"is_required,omitempty"` // 1→必填 0/2→非必填
	ClearWhenHidden bool     `json:"clear_when_hidden"`     // isEmpty==1
	Targets         []string `json:"targets"`
}

// ServiceShowHideRule 是完整显隐规则：有序条件 + conkey → 动作。
// 语义：按条件顺序求值，所有命中条件的动作按序应用（后者覆盖前者同字段设置）。
type ServiceShowHideRule struct {
	Conditions []ServiceShowHideCondition        `json:"conditions"`
	Controls   map[string]ServiceShowHideControl `json:"controls"`
}

func tryParseShowHideRule(attrData map[string]interface{}) *ServiceShowHideRule {
	rawConds, okC := attrData["conditions"]
	rawControls, okK := attrData["controls"]
	if !okC || !okK || rawConds == nil || rawControls == nil {
		return nil
	}
	var conditions []ServiceShowHideCondition
	addCond := func(v interface{}) {
		m, ok := v.(map[string]interface{})
		if !ok {
			return
		}
		name, _ := m["name"].(string)
		expr, _ := m["expression"].(string)
		conditions = append(conditions, ServiceShowHideCondition{Name: name, Expression: expr})
	}
	switch c := rawConds.(type) {
	case []interface{}:
		for _, v := range c {
			addCond(v)
		}
	case map[string]interface{}:
		// Map 形态（{"0": {...}}）按下标数字排序保证顺序
		keys := make([]string, 0, len(c))
		for k := range c {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return toIntLoose(keys[i], 0) < toIntLoose(keys[j], 0) })
		for _, k := range keys {
			addCond(c[k])
		}
	}
	if len(conditions) == 0 {
		return nil
	}
	controls := map[string]ServiceShowHideControl{}
	if list, ok := rawControls.([]interface{}); ok {
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			conkey := firstString(m, "conkey")
			setInfo, ok := m["setInfo"].(map[string]interface{})
			if conkey == "" || !ok {
				continue
			}
			var isShow, isRequired *bool
			if v := toIntLoose(setInfo["isShow"], -1); v != -1 {
				b := v == 1
				isShow = &b
			}
			if v := toIntLoose(setInfo["isRequired"], -1); v != -1 {
				b := v == 1
				isRequired = &b
			}
			var targets []string
			if pl, ok := setInfo["plugins"].([]interface{}); ok {
				for _, t := range pl {
					targets = append(targets, fmt.Sprint(t))
				}
			}
			controls[conkey] = ServiceShowHideControl{
				IsShow: isShow, IsRequired: isRequired,
				ClearWhenHidden: toIntLoose(setInfo["isEmpty"], 0) == 1,
				Targets:         targets,
			}
		}
	}
	return &ServiceShowHideRule{Conditions: conditions, Controls: controls}
}

// ─── ShowHide 表达式求值 ─────────────────────────────────────────

var (
	shEmptyCmpRe  = regexp.MustCompile(`^\{p_([A-Za-z0-9_]+)\}\s*(==|!=)\s*''$`)
	shIndexOfRe   = regexp.MustCompile(`^\{p_([A-Za-z0-9_]+)\}\.indexOf\(([^)]+)\)\s*(!==-1|==-1)$`)
	shIncludesRe  = regexp.MustCompile(`^\{p_([A-Za-z0-9_]+)\}\.includes\(([^)]+)\)$`)
	shArrayCmpRe  = regexp.MustCompile(`^\{p_([A-Za-z0-9_]+)\}\[0\](?:\.value)?\s*(==|!=)\s*(\S+)$`)
	shDateCmpRe   = regexp.MustCompile(`^new Date\(\{p_([A-Za-z0-9_]+)\}\)\s*(<=|>=|<|>)\s*new Date\('([^']+)'\)$`)
	shLooseCmpRe  = regexp.MustCompile(`^\{p_([A-Za-z0-9_]+)\}\s*(==|!=)\s*(\S+)$`)
	serviceDateRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})(?:[T ](\d{2}:\d{2}(?::\d{2})?))?`)
)

// normShowHideValue 归一化值用于表达式比较：nil→”，time.Time→ISO，其他 trim。
func normShowHideValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case time.Time:
		return t.Format("2006-01-02T15:04:05")
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func unquoteExpr(s string) string {
	t := strings.TrimSpace(s)
	if len(t) >= 2 && ((t[0] == '\'' && t[len(t)-1] == '\'') || (t[0] == '"' && t[len(t)-1] == '"')) {
		return t[1 : len(t)-1]
	}
	return t
}

// parseServiceDateTime 宽松解析服务端日期（'2026-08-10'、'2026-08-10T17:10:21+' 截断时区等）。
func parseServiceDateTime(raw interface{}) *time.Time {
	if raw == nil {
		return nil
	}
	if t, ok := raw.(time.Time); ok {
		return &t
	}
	s := strings.TrimSpace(fmt.Sprint(raw))
	if s == "" {
		return nil
	}
	m := serviceDateRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	layout := "2006-01-02"
	if m[2] != "" {
		s = m[1] + "T" + m[2]
		layout = "2006-01-02T15:04"
		if strings.Count(m[2], ":") == 2 {
			layout = "2006-01-02T15:04:05"
		}
	} else {
		s = m[1]
	}
	t, err := time.Parse(layout, s)
	if err != nil {
		return nil
	}
	return &t
}

// ParseServiceDateTimeValue 导出给 CLI 解析用户输入的日期（calendar 字段）。
func ParseServiceDateTimeValue(raw interface{}) *time.Time { return parseServiceDateTime(raw) }

// evalShowHideExpression 求值 ShowHide 条件表达式。
// 返回 nil 表示形态未支持（按不命中处理；"默认 true" 基准条件总是可求值）。
// 支持形态与 Bugaoshan evalServiceShowHideExpression 一一对应。
func evalShowHideExpression(expression string, valueOf func(string) interface{}) *bool {
	expr := strings.TrimSpace(expression)
	if expr == "" {
		return nil
	}
	if expr == "true" {
		return boolPtr(true)
	}
	if expr == "false" {
		return boolPtr(false)
	}
	// || 复合：任一 true 即 true；有未知段（nil）则不妄断
	if strings.Contains(expr, "||") {
		any := false
		for _, part := range strings.Split(expr, "||") {
			r := evalShowHideExpression(part, valueOf)
			if r != nil && *r {
				return boolPtr(true)
			}
			if r == nil {
				return nil
			}
		}
		return boolPtr(any)
	}

	if m := shEmptyCmpRe.FindStringSubmatch(expr); m != nil {
		empty := normShowHideValue(valueOf(m[1])) == ""
		return boolPtr(m[2] == "==" == empty)
	}
	if m := shIndexOfRe.FindStringSubmatch(expr); m != nil {
		v := normShowHideValue(valueOf(m[1]))
		needle := unquoteExpr(m[2])
		hit := needle != "" && strings.Contains(v, needle)
		if m[3] == "!==-1" {
			return boolPtr(hit)
		}
		return boolPtr(!hit)
	}
	if m := shIncludesRe.FindStringSubmatch(expr); m != nil {
		v := normShowHideValue(valueOf(m[1]))
		return boolPtr(strings.Contains(v, unquoteExpr(m[2])))
	}
	if m := shArrayCmpRe.FindStringSubmatch(expr); m != nil {
		v := normShowHideValue(valueOf(m[1]))
		hit := v == unquoteExpr(m[3])
		return boolPtr(m[2] == "==" == hit)
	}
	if m := shDateCmpRe.FindStringSubmatch(expr); m != nil {
		a := parseServiceDateTime(valueOf(m[1]))
		b := parseServiceDateTime(m[3])
		if a == nil || b == nil {
			return nil
		}
		switch m[2] {
		case "<=":
			return boolPtr(!a.After(*b))
		case ">=":
			return boolPtr(!a.Before(*b))
		case "<":
			return boolPtr(a.Before(*b))
		case ">":
			return boolPtr(a.After(*b))
		}
		return nil
	}
	if m := shLooseCmpRe.FindStringSubmatch(expr); m != nil {
		v := normShowHideValue(valueOf(m[1]))
		hit := v == unquoteExpr(m[3])
		return boolPtr(m[2] == "==" == hit)
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }

// ─── 插件解析 ────────────────────────────────────────────────────

// ServicePlugin 是单个表单字段（插件）。
type ServicePlugin struct {
	Key        string                `json:"key"`
	Type       ServiceFieldType      `json:"type"`
	Label      string                `json:"label"` // 服务端 description，如 "离开校区"
	Sort       int                   `json:"sort"`
	Options    []ServiceFieldOption  `json:"options,omitempty"`
	Hint       string                `json:"hint,omitempty"`
	MaxCount   int                   `json:"max_count"` // 附件上限，0/非法→3
	DataSource *ServiceDataSourceRef `json:"data_source,omitempty"`
	DateRule   *ServiceDateOrderRule `json:"date_rule,omitempty"`
	ShowHide   *ServiceShowHideRule  `json:"show_hide,omitempty"`
}

func parsePluginOptions(data map[string]interface{}) []ServiceFieldOption {
	for _, k := range []string{"options", "items", "list", "option"} {
		raw, ok := data[k].([]interface{})
		if !ok {
			continue
		}
		var out []ServiceFieldOption
		for _, item := range raw {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			label := firstString(m, "name", "label", "title")
			value := firstString(m, "value", "id")
			if label != "" && value != "" {
				out = append(out, ServiceFieldOption{Value: value, Name: label})
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// mapConfig 真实形态 {User_156: {key: "grade"}}；空 List 按空处理。
func parseMapConfig(raw interface{}) map[string]string {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	out := map[string]string{}
	for k, v := range m {
		if vm, ok := v.(map[string]interface{}); ok && vm["key"] != nil {
			out[k] = fmt.Sprint(vm["key"])
		}
	}
	return out
}

// sourceConfig 结构 {key: {value: ...}} 或 {key: 值}。
func parseSourceConfig(raw interface{}) map[string]string {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	out := map[string]string{}
	for k, v := range m {
		if vm, ok := v.(map[string]interface{}); ok && vm["value"] != nil {
			out[k] = fmt.Sprint(vm["value"])
		} else if v != nil {
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

// parseServicePlugin 从单个插件 JSON 解析（get-formv 的 plugins map entry）。
// fallbackKey 为 Map 外层 key 兜底。解析不出 key 时返回错误。
func parseServicePlugin(jsonMap map[string]interface{}, formID, formVersionID, fallbackKey string) (ServicePlugin, error) {
	key := firstString(jsonMap, "key", "dataid", "id")
	if key == "" {
		key = fallbackKey
	}
	if key == "" {
		return ServicePlugin{}, fmt.Errorf("插件缺少 key")
	}
	data := map[string]interface{}{}
	if attr := toStringMap(jsonMap["attr"]); attr != nil {
		if d := toStringMap(attr["data"]); d != nil {
			data = d
		}
	}
	typ := resolveServiceFieldType(firstString(jsonMap, "type", "component"), key)
	label := firstString(jsonMap, "description", "label", "title")
	if label == "" {
		label = firstString(data, "label", "title", "name")
	}
	p := ServicePlugin{
		Key:      key,
		Type:     typ,
		Label:    label,
		Sort:     toIntLoose(jsonMap["sort"], toIntLoose(data["sort"], 0)),
		Options:  parsePluginOptions(data),
		Hint:     firstString(data, "placeholder", "hint", "tip"),
		MaxCount: toIntLoose(firstNonNil(data["maxNum"], data["maxCount"]), 0),
	}
	if p.MaxCount <= 0 {
		p.MaxCount = 3
	}
	if typ == FieldDataSource {
		sourceID := firstString(data, "sourceid", "source_id", "data_source_id", "dataSourceId")
		if sourceID != "" {
			p.DataSource = &ServiceDataSourceRef{
				ID:            sourceID,
				FormVersionID: formVersionID,
				Component:     key,
				FormID:        formID,
				ResultKey:     firstString(data, "resultKey"),
				MapConfig:     parseMapConfig(data["mapConfig"]),
				Configure:     parseSourceConfig(data["sourceConfig"]),
			}
		}
	}
	if typ == FieldValidate {
		p.DateRule = tryParseDateOrderRule(data["rule"], data["alert"])
	}
	if typ == FieldShowHide {
		p.ShowHide = tryParseShowHideRule(data)
	}
	return p, nil
}

func firstNonNil(vals ...interface{}) interface{} {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

// ─── Schema（插件定义 + 权限/预填合并） ──────────────────────────

// FormDefinition 是 start-data 解析结果：当前表单 id、权限表、预填值。
type FormDefinition struct {
	FormID string                 // currform[0]（int 转字符串）
	Auth   map[string]string      // 字段 key → 权限
	Data   map[string]interface{} // 字段 key → 预填值
}

// parseFormDefinition 解析 start-data 的 d 字段。
// 注意 currform 元素是 int（1419），auth/data 的 key 是字符串 "1419"。
func parseFormDefinition(d map[string]interface{}) FormDefinition {
	formID := ""
	if curr, ok := d["currform"].([]interface{}); ok && len(curr) > 0 {
		formID = fmt.Sprint(curr[0])
	}
	def := FormDefinition{
		FormID: formID,
		Auth:   map[string]string{},
		Data:   map[string]interface{}{},
	}
	if auth, ok := d["auth"].(map[string]interface{}); ok && formID != "" {
		if inner, ok := auth[formID].(map[string]interface{}); ok {
			for k, v := range inner {
				def.Auth[k] = fmt.Sprint(v)
			}
		}
	}
	if data, ok := d["data"].(map[string]interface{}); ok && formID != "" {
		if inner, ok := data[formID].(map[string]interface{}); ok {
			for k, v := range inner {
				def.Data[k] = v
			}
		}
	}
	return def
}

// FormSchema 是完整可渲染 schema = get-formv 插件定义 + start-data 权限/预填。
type FormSchema struct {
	AppID         string
	FormID        string
	FormVersionID string
	Plugins       []ServicePlugin // sort 升序
	Auth          map[string]string
	Data          map[string]interface{}
}

// 占位类型：不渲染，提交时按规则给 ” 或 []。
var placeholderTypes = map[ServiceFieldType]bool{
	FieldShowHide: true, FieldVariate: true, FieldValidate: true,
	FieldConversion: true, FieldRepeatTable: true, FieldText: true,
	FieldImage: true, FieldTable: true, FieldUnknown: true,
}

// placeholderValueFor 占位字段提交空值（标量 ”、数组型 []）。
func placeholderValueFor(t ServiceFieldType) interface{} {
	switch t {
	case FieldConversion, FieldRepeatTable, FieldSelectV2, FieldCheckbox, FieldFile:
		return []interface{}{}
	default:
		return ""
	}
}

// IsRequired 字段是否必填（auth 静态必填；ShowHide 动态必填见 FormState）。
func (s *FormSchema) IsRequired(key string) bool { return s.Auth[key] == "require" }

// IsReadOnly 字段是否只读（readable/front_readonly/readonly）。
func (s *FormSchema) IsReadOnly(key string) bool {
	switch s.Auth[key] {
	case "readable", "front_readonly", "readonly":
		return true
	}
	return false
}

// IsSuppressed 字段是否被隐藏/禁用（hidden/forbidden）。
func (s *FormSchema) IsSuppressed(key string) bool {
	return s.Auth[key] == "hidden" || s.Auth[key] == "forbidden"
}

// PluginByKey 按 key 查插件。
func (s *FormSchema) PluginByKey(key string) *ServicePlugin {
	for i := range s.Plugins {
		if s.Plugins[i].Key == key {
			return &s.Plugins[i]
		}
	}
	return nil
}

// EditablePlugins 需要用户填写的字段（非占位、非 User、非 DataSource、非只读、非隐藏）。
func (s *FormSchema) EditablePlugins() []ServicePlugin {
	var out []ServicePlugin
	for _, p := range s.Plugins {
		if placeholderTypes[p.Type] || p.Type == FieldUser || p.Type == FieldDataSource ||
			s.IsReadOnly(p.Key) || s.IsSuppressed(p.Key) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// DataSourcePlugins 需取数的 DataSource 字段。
func (s *FormSchema) DataSourcePlugins() []ServicePlugin {
	var out []ServicePlugin
	for _, p := range s.Plugins {
		if p.Type == FieldDataSource && p.DataSource != nil {
			out = append(out, p)
		}
	}
	return out
}

// ActiveShowHidePlugins 可应用的 ShowHide 规则（writable 才作用于发起节点）。
func (s *FormSchema) ActiveShowHidePlugins() []ServicePlugin {
	var out []ServicePlugin
	for _, p := range s.Plugins {
		if p.Type == FieldShowHide && p.ShowHide != nil && !s.IsReadOnly(p.Key) && !s.IsSuppressed(p.Key) {
			out = append(out, p)
		}
	}
	return out
}

// DateOrderRules 全部 Validate 日期顺序校验。
func (s *FormSchema) DateOrderRules() []ServiceDateOrderRule {
	var out []ServiceDateOrderRule
	for _, p := range s.Plugins {
		if p.DateRule != nil {
			out = append(out, *p.DateRule)
		}
	}
	return out
}

// BuildFormSchema 从 get-formv 的 d 与 start-data 合并构建 schema。
func BuildFormSchema(appID string, formvD map[string]interface{}, def FormDefinition) (*FormSchema, error) {
	formID := firstString(formvD, "id", "form_id", "formId")
	if formID == "" {
		formID = def.FormID
	}
	formVersionID := firstString(formvD, "form_version_id", "version_id", "formVersionId")

	// d.plugins 为二次编码 JSON 字符串，解码后 {nowNum, plugins: {K: P}, rtplugins}
	pluginsRaw := maybeJSONString(formvD["plugins"])
	if m, ok := pluginsRaw.(map[string]interface{}); ok && m["plugins"] != nil {
		pluginsRaw = m["plugins"]
	}
	type entry struct {
		fallbackKey string
		m           map[string]interface{}
	}
	var entries []entry
	switch v := pluginsRaw.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				entries = append(entries, entry{"", m})
			}
		}
	case map[string]interface{}:
		for k, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				entries = append(entries, entry{k, m})
			}
		}
	}
	if len(entries) == 0 {
		return nil, &auth.ServiceError{Msg: "get-formv 无可解析插件"}
	}
	schema := &FormSchema{
		AppID: appID, FormID: formID, FormVersionID: formVersionID,
		Auth: def.Auth, Data: def.Data,
	}
	for _, e := range entries {
		p, err := parseServicePlugin(e.m, formID, formVersionID, e.fallbackKey)
		if err != nil {
			return nil, &auth.ServiceError{Msg: "插件解析失败: " + err.Error()}
		}
		schema.Plugins = append(schema.Plugins, p)
	}
	sort.SliceStable(schema.Plugins, func(i, j int) bool { return schema.Plugins[i].Sort < schema.Plugins[j].Sort })
	if len(schema.EditablePlugins()) == 0 {
		return nil, &auth.ServiceError{Msg: "表单无可填写字段（schema 不可渲染）"}
	}
	return schema, nil
}

// ─── FormState（值容器 + 校验 + 提交体组装） ─────────────────────

// FormState 值类型约定：
// input/multiInput/dataSource/radio/select/selectV2 → string（选项 value）
// checkbox → []string；calendar → time.Time；region → map（toRegionData 结构）
// file → []ServiceAttachment
type FormState struct {
	Schema *FormSchema
	Values map[string]interface{}
}

// NewFormState 构造并从服务端预填播种（calendar 字符串宽容解析；空串不播种；
// checkbox/file 的预填值从 JSON 解码的 []interface{} 归一化为具体类型，
// 否则下游 isEmptyValue/序列化的类型断言会静默失败）。
func NewFormState(schema *FormSchema) *FormState {
	st := &FormState{Schema: schema, Values: map[string]interface{}{}}
	for k, v := range schema.Data {
		if v == nil {
			continue
		}
		p := schema.PluginByKey(k)
		if p != nil {
			switch p.Type {
			case FieldCalendar:
				if parsed := parseServiceDateTime(v); parsed != nil {
					st.Values[k] = *parsed
				}
				continue
			case FieldCheckbox:
				if list, ok := v.([]interface{}); ok {
					ss := make([]string, 0, len(list))
					for _, item := range list {
						if s, ok := item.(string); ok {
							ss = append(ss, s)
						}
					}
					st.Values[k] = ss
					continue
				}
			case FieldFile:
				if list, ok := v.([]interface{}); ok {
					atts := make([]ServiceAttachment, 0, len(list))
					for _, item := range list {
						m, ok := item.(map[string]interface{})
						if !ok {
							continue
						}
						name, _ := m["name"].(string)
						u, _ := m["url"].(string)
						id, _ := m["id"].(string)
						atts = append(atts, ServiceAttachment{Name: name, URL: u, ID: id})
					}
					st.Values[k] = atts
					continue
				}
			}
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		st.Values[k] = v
	}
	return st
}

// evalShowHide 跑一遍 ShowHide 引擎，返回 (visibility, required) 两张表。
func (st *FormState) evalShowHide() (map[string]bool, map[string]bool) {
	vis := map[string]bool{}
	req := map[string]bool{}
	for _, p := range st.Schema.ActiveShowHidePlugins() {
		rule := p.ShowHide
		for i, cond := range rule.Conditions {
			hit := evalShowHideExpression(cond.Expression, func(k string) interface{} { return st.Values[k] })
			if hit == nil || !*hit {
				continue
			}
			control, ok := rule.Controls[strconv.Itoa(i)]
			if !ok {
				continue
			}
			for _, t := range control.Targets {
				if control.IsShow != nil {
					vis[t] = *control.IsShow
				}
				if control.IsRequired != nil {
					req[t] = *control.IsRequired
				}
			}
		}
	}
	return vis, req
}

// IsFieldVisible 字段当前是否显示（ShowHide 引擎 → 默认显示）。
func (st *FormState) IsFieldVisible(p ServicePlugin) bool {
	vis, _ := st.evalShowHide()
	if v, ok := vis[p.Key]; ok {
		return v
	}
	return true
}

// IsFieldRequired 字段当前是否必填（ShowHide 动态必填 → auth require）。
func (st *FormState) IsFieldRequired(key string) bool {
	_, req := st.evalShowHide()
	if v, ok := req[key]; ok {
		return v
	}
	return st.Schema.IsRequired(key)
}

// ApplyDataSourceValue DataSource 取数结果分发：
// setplugin 按 MapConfig 分发多列（calendar 目标宽容解析日期）；
// 否则 name 写入本字段与 ResultKey 配对字段。
func (st *FormState) ApplyDataSourceValue(p ServicePlugin, listValue interface{}) {
	ref := p.DataSource
	if ref == nil {
		return
	}
	if ref.IsSetPlugin() {
		if m, ok := listValue.(map[string]interface{}); ok {
			for targetKey, col := range ref.MapConfig {
				raw := m[col]
				if raw == nil || fmt.Sprint(raw) == "" {
					continue
				}
				if target := st.Schema.PluginByKey(targetKey); target != nil && target.Type == FieldCalendar {
					if parsed := parseServiceDateTime(raw); parsed != nil {
						st.Values[targetKey] = *parsed
					}
				} else {
					st.Values[targetKey] = fmt.Sprint(raw)
				}
			}
		}
		return
	}
	name := strings.TrimSpace(fmt.Sprint(listValue))
	if name == "" || name == "<nil>" {
		return
	}
	st.Values[p.Key] = name
	if ref.ResultKey != "" {
		st.Values[ref.ResultKey] = name
	}
}

// Validate 提交前校验：可见且当前必填的可编辑字段按类型判空；然后日期顺序规则。
func (st *FormState) Validate() error {
	for _, p := range st.Schema.EditablePlugins() {
		if !st.IsFieldRequired(p.Key) || !st.IsFieldVisible(p) {
			continue
		}
		if st.isEmptyValue(p, st.Values[p.Key]) {
			label := p.Label
			if label == "" {
				label = p.Key
			}
			return &auth.ServiceError{Msg: fmt.Sprintf("必填字段 %s（%s）为空", label, p.Key)}
		}
	}
	for _, rule := range st.Schema.DateOrderRules() {
		a, aOk := st.Values[rule.FirstKey].(time.Time)
		b, bOk := st.Values[rule.SecondKey].(time.Time)
		if aOk && bOk && a.After(b) {
			msg := rule.Message
			if msg == "" {
				msg = fmt.Sprintf("%s 必须早于等于 %s", rule.FirstKey, rule.SecondKey)
			}
			return &auth.ServiceError{Msg: msg}
		}
	}
	return nil
}

func (st *FormState) isEmptyValue(p ServicePlugin, v interface{}) bool {
	switch p.Type {
	case FieldRadio, FieldSelect, FieldSelectV2:
		return v == nil || fmt.Sprint(v) == ""
	case FieldCheckbox:
		list, _ := v.([]string)
		return len(list) == 0
	case FieldInput, FieldMultiInput, FieldDataSource:
		return v == nil || strings.TrimSpace(fmt.Sprint(v)) == ""
	case FieldRegion:
		m, _ := v.(map[string]interface{})
		return len(m) == 0
	case FieldFile:
		list, _ := v.([]ServiceAttachment)
		return len(list) == 0
	case FieldCalendar:
		_, ok := v.(time.Time)
		return !ok
	default:
		return v == nil
	}
}

func (st *FormState) hasValue(p ServicePlugin, v interface{}) bool { return !st.isEmptyValue(p, v) }

// BuildFormFields 组装单个表单的字段 Map（不含外层 formId 包装）。
// 规则复现 350 抓包已验证行为（占位/DataSource 配对省略/只读透传/隐藏给空值）。
func (st *FormState) BuildFormFields() map[string]interface{} {
	result := map[string]interface{}{}
	var omitted []string
	// DataSource 配对字段写入收集到循环结束后统一应用：遍历 Auth map 的
	// 顺序随机，直接写入可能被目标字段自身的分支（如隐藏占位）覆盖。
	pairWrites := map[string]string{}

	for key := range st.Schema.Auth {
		p := st.Schema.PluginByKey(key)
		typ := FieldUnknown
		if p != nil {
			typ = p.Type
		} else {
			typ = serviceFieldTypeFromKey(key)
		}
		v := st.Values[key]

		if placeholderTypes[typ] {
			result[key] = placeholderValueFor(typ)
			continue
		}

		if typ == FieldDataSource {
			name := strings.TrimSpace(fmt.Sprint(v))
			if v == nil || name == "" || name == "<nil>" {
				// 取数为空：整对省略（350 已验证）
				omitted = append(omitted, key)
				if p != nil && p.DataSource != nil && p.DataSource.ResultKey != "" {
					omitted = append(omitted, p.DataSource.ResultKey)
				}
				continue
			}
			result[key] = map[string]interface{}{"list": name}
			if p != nil && p.DataSource != nil && p.DataSource.ResultKey != "" && !p.DataSource.IsSetPlugin() {
				pairWrites[p.DataSource.ResultKey] = name
			}
			continue
		}

		if p == nil {
			// 孤儿 auth key（插件已删/未下发）按前缀规则占位兜底
			result[key] = placeholderValueFor(typ)
			continue
		}

		if st.Schema.IsReadOnly(key) || st.Schema.IsSuppressed(key) || typ == FieldUser {
			if st.hasValue(*p, v) {
				result[key] = serializeServiceFieldValue(*p, v)
			} else {
				result[key] = placeholderValueFor(typ)
			}
			continue
		}

		if !st.IsFieldVisible(*p) {
			result[key] = placeholderValueFor(typ)
			continue
		}
		result[key] = serializeServiceFieldValue(*p, v)
	}

	for k, v := range pairWrites {
		result[k] = v
	}
	for _, k := range omitted {
		delete(result, k)
	}
	return result
}

// BuildFormData 完整 form_data（外层以 formId 字符串包装）。
func (st *FormState) BuildFormData() map[string]interface{} {
	return map[string]interface{}{st.Schema.FormID: st.BuildFormFields()}
}

// serializeServiceFieldValue 类型感知序列化（单字段提交值）。
func serializeServiceFieldValue(p ServicePlugin, value interface{}) interface{} {
	optionName := func(v string) string {
		for _, o := range p.Options {
			if o.Value == v {
				return o.Name
			}
		}
		return ""
	}
	switch p.Type {
	case FieldRadio, FieldSelect:
		v := strings.TrimSpace(fmt.Sprint(value))
		if value == nil || v == "" || v == "<nil>" {
			return ""
		}
		return map[string]interface{}{"value": v, "name": optionName(v)}
	case FieldSelectV2:
		v := strings.TrimSpace(fmt.Sprint(value))
		if value == nil || v == "" || v == "<nil>" {
			return []interface{}{}
		}
		return []interface{}{map[string]interface{}{"value": v, "name": optionName(v)}}
	case FieldCheckbox:
		selected, _ := value.([]string)
		out := []interface{}{}
		for _, v := range selected {
			out = append(out, map[string]interface{}{"value": v, "name": optionName(v)})
		}
		return out
	case FieldCalendar:
		t, ok := value.(time.Time)
		if !ok {
			return ""
		}
		return t.UTC().Format("2006-01-02T15:04:05.000Z")
	case FieldRegion:
		m, ok := value.(map[string]interface{})
		if !ok || len(m) == 0 {
			return ""
		}
		return regionToFormData(m)
	case FieldFile:
		atts, _ := value.([]ServiceAttachment)
		out := []interface{}{}
		for _, a := range atts {
			out = append(out, a.ToFormData())
		}
		return out
	case FieldDataSource:
		name := strings.TrimSpace(fmt.Sprint(value))
		if value == nil || name == "" || name == "<nil>" {
			return ""
		}
		return map[string]interface{}{"list": name}
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
}

// regionToFormData Region 提交结构：{province/city/area:{label,value}, details, address}。
func regionToFormData(sel map[string]interface{}) map[string]interface{} {
	part := func(key string) map[string]interface{} {
		if m, ok := sel[key].(map[string]interface{}); ok {
			return map[string]interface{}{
				"label": fmt.Sprint(m["label"]),
				"value": fmt.Sprint(m["value"]),
			}
		}
		return map[string]interface{}{"label": "", "value": ""}
	}
	details := strings.TrimSpace(fmt.Sprint(sel["details"]))
	var segs []string
	for _, k := range []string{"province", "city", "area"} {
		if m, ok := sel[k].(map[string]interface{}); ok {
			if l := fmt.Sprint(m["label"]); l != "" && l != "<nil>" {
				segs = append(segs, l)
			}
		}
	}
	if details != "" {
		segs = append(segs, details)
	}
	return map[string]interface{}{
		"province": part("province"),
		"city":     part("city"),
		"area":     part("area"),
		"details":  details,
		"address":  strings.Join(segs, "/"),
	}
}
