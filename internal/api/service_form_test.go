package api

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// ─── 类型推断 ────────────────────────────────────────────────────

func TestResolveServiceFieldType(t *testing.T) {
	cases := []struct {
		declared, key string
		want          ServiceFieldType
	}{
		{"dRadio", "Radio_30", FieldRadio},
		{"dSelectV2", "SelectV2_44", FieldSelectV2},
		{"dFile", "File_71", FieldFile},
		{"dXImage", "Ximage_72", FieldFile},
		{"dMultiText", "MultiText_31", FieldMultiInput},
		{"dIntegerInput", "Input_99", FieldInput}, // integerinput 归 input
		{"dOneInput", "Text_12", FieldText},       // 静态说明文字
		{"", "Calendar_40", FieldCalendar},        // 无声明按前缀
		{"", "Region_50", FieldRegion},
		{"dUnknownWidget", "DataSource_85", FieldDataSource}, // 未知组件回退前缀
		{"", "Mystery_1", FieldUnknown},
	}
	for _, c := range cases {
		if got := resolveServiceFieldType(c.declared, c.key); got != c.want {
			t.Errorf("resolveServiceFieldType(%q,%q)=%s, want %s", c.declared, c.key, got, c.want)
		}
	}
}

// ─── 日期解析 ────────────────────────────────────────────────────

func TestParseServiceDateTime(t *testing.T) {
	cases := []struct {
		raw  string
		want string // RFC3339-ish 前缀比对
	}{
		{"2026-08-10", "2026-08-10T00:00:00"},
		{"2026-08-10 17:10", "2026-08-10T17:10:00"},
		{"2026-08-10T17:10:21", "2026-08-10T17:10:21"},
		{"2026-08-10T17:10:21+", "2026-08-10T17:10:21"}, // 截断的时区后缀
	}
	for _, c := range cases {
		got := parseServiceDateTime(c.raw)
		if got == nil {
			t.Errorf("parseServiceDateTime(%q)=nil", c.raw)
			continue
		}
		if got.Format("2006-01-02T15:04:05") != c.want {
			t.Errorf("parseServiceDateTime(%q)=%s, want %s", c.raw, got.Format("2006-01-02T15:04:05"), c.want)
		}
	}
	for _, bad := range []string{"", "abc", "2026/08/10"} {
		if parseServiceDateTime(bad) != nil {
			t.Errorf("parseServiceDateTime(%q) 应返回 nil", bad)
		}
	}
}

// ─── Validate 规则 ───────────────────────────────────────────────

func TestTryParseDateOrderRule(t *testing.T) {
	r := tryParseDateOrderRule("{f_dateDayMinus}({p_Calendar_30},{p_Calendar_31})<0", "开始日期不能晚于结束日期")
	if r == nil {
		t.Fatal("应解析出规则")
	}
	if r.FirstKey != "Calendar_30" || r.SecondKey != "Calendar_31" || r.Message != "开始日期不能晚于结束日期" {
		t.Errorf("规则内容不符: %+v", r)
	}
	if tryParseDateOrderRule("{f_other}({p_A},{p_B})<0", "") != nil {
		t.Error("非 f_dateDayMinus 形态应返回 nil")
	}
	if tryParseDateOrderRule(123, "") != nil {
		t.Error("非字符串应返回 nil")
	}
}

// ─── ShowHide 表达式 ─────────────────────────────────────────────

func TestEvalShowHideExpression(t *testing.T) {
	vals := map[string]interface{}{
		"Radio_30":    "1",
		"Input_31":    "  ",
		"SelectV2_32": "xyz",
		"Calendar_40": time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
	}
	valueOf := func(k string) interface{} { return vals[k] }
	cases := []struct {
		expr string
		want *bool
	}{
		{"true", boolPtr(true)},
		{"false", boolPtr(false)},
		{"{p_Radio_30}==''", boolPtr(false)},
		{"{p_Input_31}==''", boolPtr(true)}, // trim 后为空
		{"{p_Radio_30}!=''", boolPtr(true)},
		{`{p_Radio_30}.indexOf('1')!==-1`, boolPtr(true)},
		{`{p_Radio_30}.indexOf('2')==-1`, boolPtr(true)},
		{`{p_SelectV2_32}.includes('y')`, boolPtr(true)},
		{`{p_SelectV2_32}[0].value=='xyz'`, boolPtr(true)},
		{`{p_SelectV2_32}[0]=='abc'`, boolPtr(false)},
		{`{p_SelectV2_32}[0].value!='xyz'`, boolPtr(false)},
		{`new Date({p_Calendar_40}) <= new Date('2026-08-10')`, boolPtr(true)},
		{`new Date({p_Calendar_40}) < new Date('2026-08-10')`, boolPtr(false)},
		{`new Date({p_Calendar_40}) >= new Date('2026-08-11')`, boolPtr(false)},
		{"{p_Radio_30}==1", boolPtr(true)}, // 宽松比较
		{"{p_Radio_30}=='1'", boolPtr(true)},
		{"{p_Radio_30}!=2", boolPtr(true)},
		{"{p_Radio_30}==2 || {p_Input_31}==''", boolPtr(true)}, // 一真即真
		{"{p_Radio_30}==2 || {p_Input_31}=='x'", boolPtr(false)},
		{"{p_Missing}==''", boolPtr(true)},  // 缺失值归一为空串
		{"{p_Calendar_40}.foo('bar')", nil}, // 未知形态
	}
	for _, c := range cases {
		got := evalShowHideExpression(c.expr, valueOf)
		if (got == nil) != (c.want == nil) {
			t.Errorf("eval(%q)=%v, want %v", c.expr, got, c.want)
			continue
		}
		if got != nil && *got != *c.want {
			t.Errorf("eval(%q)=%v, want %v", c.expr, *got, *c.want)
		}
	}
}

// ─── 插件解析（双层编码） ─────────────────────────────────────────

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseServicePluginDoubleEncoded(t *testing.T) {
	// attr 与 attr.data 都是 JSON 字符串（服务端双层编码）。
	attrData := map[string]interface{}{
		"label":   "离开校区",
		"options": []interface{}{map[string]interface{}{"name": "是", "value": "1"}, map[string]interface{}{"label": "否", "value": "0"}},
		"sort":    float64(3),
	}
	attr := map[string]interface{}{"data": mustJSON(t, attrData)}
	plugin := map[string]interface{}{
		"type": "dRadio",
		"attr": mustJSON(t, attr),
	}
	p, err := parseServicePlugin(plugin, "1419", "2357", "Radio_30")
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "Radio_30" || p.Type != FieldRadio || p.Label != "离开校区" || p.Sort != 3 {
		t.Errorf("插件基本字段不符: %+v", p)
	}
	if len(p.Options) != 2 || p.Options[0].Value != "1" || p.Options[0].Name != "是" || p.Options[1].Name != "否" {
		t.Errorf("选项解析不符: %+v", p.Options)
	}
}

func TestParseServicePluginDataSource(t *testing.T) {
	attrData := map[string]interface{}{
		"sourceid":     "8",
		"resultKey":    "setplugin",
		"mapConfig":    map[string]interface{}{"User_156": map[string]interface{}{"key": "grade"}},
		"sourceConfig": map[string]interface{}{"dept": map[string]interface{}{"value": "301"}},
	}
	plugin := map[string]interface{}{
		"key":  "DataSource_85",
		"type": "dDataSource",
		"attr": map[string]interface{}{"data": attrData},
	}
	p, err := parseServicePlugin(plugin, "1419", "2357", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.DataSource == nil {
		t.Fatal("应解析出 DataSource")
	}
	ref := p.DataSource
	if ref.ID != "8" || !ref.IsSetPlugin() || ref.MapConfig["User_156"] != "grade" || ref.Configure["dept"] != "301" {
		t.Errorf("DataSource 配置不符: %+v", ref)
	}
	if ref.FormVersionID != "2357" || ref.FormID != "1419" || ref.Component != "DataSource_85" {
		t.Errorf("DataSource 上下文字段不符: %+v", ref)
	}
}

func TestParseShowHideRuleMapConditions(t *testing.T) {
	// conditions 为 {"0": {...}} Map 形态，controls 为 List。
	attrData := map[string]interface{}{
		"conditions": map[string]interface{}{
			"0": map[string]interface{}{"name": "默认", "expression": "true"},
			"1": map[string]interface{}{"name": "选是", "expression": "{p_Radio_30}==1"},
		},
		"controls": []interface{}{
			map[string]interface{}{"conkey": "0", "setInfo": map[string]interface{}{"isShow": float64(0), "isRequired": float64(0), "plugins": []interface{}{"Input_31"}}},
			map[string]interface{}{"conkey": "1", "setInfo": map[string]interface{}{"isShow": float64(1), "isRequired": float64(1), "plugins": []interface{}{"Input_31"}}},
		},
	}
	rule := tryParseShowHideRule(attrData)
	if rule == nil {
		t.Fatal("应解析出 ShowHide 规则")
	}
	if len(rule.Conditions) != 2 || rule.Conditions[1].Expression != "{p_Radio_30}==1" {
		t.Errorf("条件顺序不符: %+v", rule.Conditions)
	}
	c0 := rule.Controls["0"]
	if c0.IsShow == nil || *c0.IsShow || c0.IsRequired == nil || *c0.IsRequired {
		t.Errorf("control 0 不符: %+v", c0)
	}
	c1 := rule.Controls["1"]
	if c1.IsShow == nil || !*c1.IsShow || c1.IsRequired == nil || !*c1.IsRequired || len(c1.Targets) != 1 {
		t.Errorf("control 1 不符: %+v", c1)
	}
}

// ─── start-data / schema 构建 ────────────────────────────────────

func TestParseFormDefinition(t *testing.T) {
	d := map[string]interface{}{
		"currform": []interface{}{float64(1419)}, // int 元素
		"auth":     map[string]interface{}{"1419": map[string]interface{}{"Input_31": "require", "User_156": "writable"}},
		"data":     map[string]interface{}{"1419": map[string]interface{}{"User_156": "张三"}},
	}
	def := parseFormDefinition(d)
	if def.FormID != "1419" {
		t.Errorf("FormID=%q, want 1419", def.FormID)
	}
	if def.Auth["Input_31"] != "require" || def.Data["User_156"] != "张三" {
		t.Errorf("auth/data 解析不符: %+v", def)
	}
}

// buildTestSchema 构造一个最小 schema：单选 + 条件显示输入框 + 日期对 + 占位字段。
func buildTestSchema(t *testing.T) *FormSchema {
	t.Helper()
	showHideData := map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{"name": "默认", "expression": "true"},
			map[string]interface{}{"name": "选是", "expression": "{p_Radio_30}==1"},
		},
		"controls": []interface{}{
			map[string]interface{}{"conkey": "0", "setInfo": map[string]interface{}{"isShow": float64(0), "isRequired": float64(0), "plugins": []interface{}{"Input_31"}}},
			map[string]interface{}{"conkey": "1", "setInfo": map[string]interface{}{"isShow": float64(1), "isRequired": float64(1), "plugins": []interface{}{"Input_31"}}},
		},
	}
	validateData := map[string]interface{}{
		"rule":  "{f_dateDayMinus}({p_Calendar_40},{p_Calendar_41})<0",
		"alert": "开始日期不能晚于结束日期",
	}
	pluginsInner := map[string]interface{}{
		"Radio_30": map[string]interface{}{
			"type": "dRadio", "sort": float64(1),
			"attr": map[string]interface{}{"data": map[string]interface{}{
				"label":   "是否离校",
				"options": []interface{}{map[string]interface{}{"name": "是", "value": "1"}, map[string]interface{}{"name": "否", "value": "0"}},
			}},
		},
		"Input_31":    map[string]interface{}{"type": "dInput", "sort": float64(2), "attr": map[string]interface{}{"data": map[string]interface{}{"label": "去向"}}},
		"Calendar_40": map[string]interface{}{"type": "dCalendar", "sort": float64(3), "attr": map[string]interface{}{"data": map[string]interface{}{"label": "开始日期"}}},
		"Calendar_41": map[string]interface{}{"type": "dCalendar", "sort": float64(4), "attr": map[string]interface{}{"data": map[string]interface{}{"label": "结束日期"}}},
		"ShowHide_90": map[string]interface{}{"type": "dShowHide", "sort": float64(9), "attr": map[string]interface{}{"data": showHideData}},
		"Validate_91": map[string]interface{}{"type": "dValidate", "sort": float64(10), "attr": map[string]interface{}{"data": validateData}},
		"Text_92":     map[string]interface{}{"type": "dOneInput", "sort": float64(11)},
	}
	formvD := map[string]interface{}{
		"id":              "1419",
		"form_version_id": "2357",
		"plugins":         mustJSON(t, map[string]interface{}{"plugins": pluginsInner}), // 双层编码
	}
	def := FormDefinition{
		FormID: "1419",
		Auth: map[string]string{
			"Radio_30": "writable", "Input_31": "writable",
			"Calendar_40": "require", "Calendar_41": "require",
			"ShowHide_90": "writable", "Validate_91": "writable", "Text_92": "writable",
			"Ghost_99": "writable", // 孤儿 auth key（插件未下发）
		},
		Data: map[string]interface{}{"Calendar_40": "2026-08-10", "Radio_30": ""},
	}
	schema, err := BuildFormSchema("350", formvD, def)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestBuildFormSchema(t *testing.T) {
	schema := buildTestSchema(t)
	if schema.FormID != "1419" || schema.FormVersionID != "2357" {
		t.Errorf("schema 头部不符: %+v", schema)
	}
	if len(schema.Plugins) != 7 {
		t.Fatalf("插件数=%d, want 7", len(schema.Plugins))
	}
	// sort 升序
	for i := 1; i < len(schema.Plugins); i++ {
		if schema.Plugins[i-1].Sort > schema.Plugins[i].Sort {
			t.Error("插件未按 sort 升序")
		}
	}
	if len(schema.DateOrderRules()) != 1 || schema.DateOrderRules()[0].FirstKey != "Calendar_40" {
		t.Error("日期规则缺失")
	}
	if len(schema.ActiveShowHidePlugins()) != 1 {
		t.Error("ShowHide 规则缺失")
	}
}

func TestFormStateSeedAndShowHide(t *testing.T) {
	schema := buildTestSchema(t)
	st := NewFormState(schema)
	// 播种：calendar 字符串解析为 time.Time；空串不播种
	if _, ok := st.Values["Calendar_40"].(time.Time); !ok {
		t.Error("Calendar_40 应播种为 time.Time")
	}
	if _, ok := st.Values["Radio_30"]; ok {
		t.Error("空串不应播种")
	}
	// 默认条件 true → Input_31 隐藏非必填
	if st.IsFieldVisible(*schema.PluginByKey("Input_31")) {
		t.Error("默认条件命中后 Input_31 应隐藏")
	}
	// 选"是"→ 显示且必填（后命中条件覆盖前者）
	st.Values["Radio_30"] = "1"
	if !st.IsFieldVisible(*schema.PluginByKey("Input_31")) {
		t.Error("Radio_30=1 后 Input_31 应显示")
	}
	if !st.IsFieldRequired("Input_31") {
		t.Error("Radio_30=1 后 Input_31 应动态必填")
	}
	// 校验：动态必填为空 → 报错
	if err := st.Validate(); err == nil {
		t.Error("动态必填为空应校验失败")
	}
}

func TestFormStateValidateDateOrder(t *testing.T) {
	schema := buildTestSchema(t)
	st := NewFormState(schema)
	st.Values["Radio_30"] = "0"
	st.Values["Calendar_41"] = time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) // 早于 Calendar_40(08-10)
	err := st.Validate()
	if err == nil || err.Error() != "开始日期不能晚于结束日期" {
		t.Errorf("日期顺序校验应报 alert 文案, got %v", err)
	}
}

func TestBuildFormData(t *testing.T) {
	schema := buildTestSchema(t)
	st := NewFormState(schema)
	st.Values["Radio_30"] = "1"
	st.Values["Input_31"] = "成都市"
	st.Values["Calendar_41"] = time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	formData := st.BuildFormData()
	fields, ok := formData["1419"].(map[string]interface{})
	if !ok {
		t.Fatal("外层应以 formId 包装")
	}
	// Radio → {value, name}
	radio, _ := fields["Radio_30"].(map[string]interface{})
	if radio["value"] != "1" || radio["name"] != "是" {
		t.Errorf("Radio 序列化不符: %+v", fields["Radio_30"])
	}
	// 显示的 Input → 字符串
	if fields["Input_31"] != "成都市" {
		t.Errorf("Input_31=%v", fields["Input_31"])
	}
	// Calendar → UTC ISO（Calendar_41 输入即 UTC 12:00）
	if fields["Calendar_40"] != "2026-08-10T00:00:00.000Z" || fields["Calendar_41"] != "2026-08-20T12:00:00.000Z" {
		t.Errorf("Calendar 序列化不符: %v / %v", fields["Calendar_40"], fields["Calendar_41"])
	}
	// 占位字段：标量 ''、数组 []
	if fields["ShowHide_90"] != "" || fields["Validate_91"] != "" || fields["Text_92"] != "" {
		t.Errorf("标量占位应为 '': %+v", fields)
	}
	if fields["Ghost_99"] != "" {
		t.Errorf("Ghost_99（孤儿 auth key，未知前缀）应为标量占位 '': got %+v", fields["Ghost_99"])
	}
}

func TestBuildFormFieldsHiddenPlaceholder(t *testing.T) {
	schema := buildTestSchema(t)
	st := NewFormState(schema)
	st.Values["Radio_30"] = "0" // Input_31 保持隐藏
	st.Values["Input_31"] = "不应出现"
	st.Values["Calendar_41"] = time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	fields := st.BuildFormFields()
	if fields["Input_31"] != "" {
		t.Errorf("隐藏字段应提交占位空值, got %v", fields["Input_31"])
	}
}

func TestPlaceholderValueFor(t *testing.T) {
	if placeholderValueFor(FieldInput) != "" {
		t.Error("input 占位应为 ''")
	}
	for _, typ := range []ServiceFieldType{FieldConversion, FieldRepeatTable, FieldSelectV2, FieldCheckbox, FieldFile} {
		if arr, ok := placeholderValueFor(typ).([]interface{}); !ok || len(arr) != 0 {
			t.Errorf("%s 占位应为 []", typ)
		}
	}
}

// ─── DataSource / 序列化 ─────────────────────────────────────────

func TestApplyDataSourceValueSetPlugin(t *testing.T) {
	schema := buildTestSchema(t)
	schema.Plugins = append(schema.Plugins, ServicePlugin{
		Key: "DataSource_85", Type: FieldDataSource,
		DataSource: &ServiceDataSourceRef{
			ID: "8", ResultKey: "setplugin",
			MapConfig: map[string]string{"Input_31": "grade", "Calendar_40": "back_date"},
		},
	})
	schema.Auth["DataSource_85"] = "writable"
	st := NewFormState(schema)
	st.ApplyDataSourceValue(*schema.PluginByKey("DataSource_85"), map[string]interface{}{
		"grade": "2023", "back_date": "2026-09-01",
	})
	if st.Values["Input_31"] != "2023" {
		t.Errorf("MapConfig 分发不符: %v", st.Values["Input_31"])
	}
	if tm, ok := st.Values["Calendar_40"].(time.Time); !ok || tm.Format("2006-01-02") != "2026-09-01" {
		t.Errorf("calendar 目标应解析为日期: %v", st.Values["Calendar_40"])
	}
}

func TestApplyDataSourceValueResultKeyPair(t *testing.T) {
	schema := buildTestSchema(t)
	schema.Plugins = append(schema.Plugins, ServicePlugin{
		Key: "DataSource_85", Type: FieldDataSource,
		DataSource: &ServiceDataSourceRef{ID: "8", ResultKey: "Input_31"},
	})
	schema.Auth["DataSource_85"] = "writable"
	st := NewFormState(schema)
	p := *schema.PluginByKey("DataSource_85")
	st.ApplyDataSourceValue(p, "张老师")
	if st.Values["DataSource_85"] != "张老师" || st.Values["Input_31"] != "张老师" {
		t.Errorf("ResultKey 配对写入不符: %v / %v", st.Values["DataSource_85"], st.Values["Input_31"])
	}
	// dataSource 提交形态 {"list": name} + 配对字段
	st.Values["Radio_30"] = "0"
	st.Values["Calendar_41"] = time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	fields := st.BuildFormFields()
	ds, _ := fields["DataSource_85"].(map[string]interface{})
	if ds["list"] != "张老师" || fields["Input_31"] != "张老师" {
		t.Errorf("dataSource 提交形态不符: %+v / %v", fields["DataSource_85"], fields["Input_31"])
	}
}

func TestBuildFormFieldsDataSourceEmptyOmitsPair(t *testing.T) {
	schema := buildTestSchema(t)
	schema.Plugins = append(schema.Plugins, ServicePlugin{
		Key: "DataSource_85", Type: FieldDataSource,
		DataSource: &ServiceDataSourceRef{ID: "8", ResultKey: "Input_31"},
	})
	schema.Auth["DataSource_85"] = "writable"
	st := NewFormState(schema)
	st.Values["Radio_30"] = "0"
	st.Values["Calendar_41"] = time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	fields := st.BuildFormFields()
	if _, ok := fields["DataSource_85"]; ok {
		t.Error("取数为空的 DataSource 应省略")
	}
	if _, ok := fields["Input_31"]; ok {
		t.Error("配对字段应一并省略")
	}
}

func TestSerializeFieldValueShapes(t *testing.T) {
	sel := ServicePlugin{Key: "Select_1", Type: FieldSelect, Options: []ServiceFieldOption{{Value: "a", Name: "甲"}}}
	if got := serializeServiceFieldValue(sel, ""); got != "" {
		t.Errorf("select 空值应为 '', got %v", got)
	}
	v2 := ServicePlugin{Key: "SelectV2_1", Type: FieldSelectV2, Options: []ServiceFieldOption{{Value: "a", Name: "甲"}}}
	gotV2 := serializeServiceFieldValue(v2, "a")
	if arr, ok := gotV2.([]interface{}); !ok || len(arr) != 1 {
		t.Errorf("selectV2 单选也应为单元素数组, got %v", gotV2)
	}
	if got := serializeServiceFieldValue(v2, ""); reflect.DeepEqual(got, "") {
		t.Error("selectV2 空值应为 []")
	}
	cb := ServicePlugin{Key: "Checkbox_1", Type: FieldCheckbox, Options: []ServiceFieldOption{{Value: "x", Name: "叉"}, {Value: "y", Name: "歪"}}}
	gotCb := serializeServiceFieldValue(cb, []string{"x", "y"})
	if arr, ok := gotCb.([]interface{}); !ok || len(arr) != 2 {
		t.Errorf("checkbox 序列化不符: %v", gotCb)
	}
	file := ServicePlugin{Key: "File_1", Type: FieldFile}
	gotFile := serializeServiceFieldValue(file, []ServiceAttachment{{Name: "a.png", URL: "u", ID: "7"}})
	arr, _ := gotFile.([]interface{})
	if len(arr) != 1 || arr[0].(map[string]interface{})["id"] != "7" {
		t.Errorf("file 序列化不符: %v", gotFile)
	}
	region := ServicePlugin{Key: "Region_1", Type: FieldRegion}
	gotRegion := serializeServiceFieldValue(region, map[string]interface{}{
		"province": map[string]interface{}{"label": "四川省", "value": "510000"},
		"city":     map[string]interface{}{"label": "成都市", "value": "510100"},
		"area":     map[string]interface{}{"label": "武侯区", "value": "510107"},
		"details":  "望江路29号",
	})
	rm, _ := gotRegion.(map[string]interface{})
	if rm["address"] != "四川省/成都市/武侯区/望江路29号" {
		t.Errorf("region address 拼接不符: %v", rm["address"])
	}
	if rm["province"].(map[string]interface{})["value"] != "510000" {
		t.Errorf("region province 不符: %v", rm["province"])
	}
	if got := serializeServiceFieldValue(region, map[string]interface{}{}); got != "" {
		t.Errorf("region 空选择应为 '', got %v", got)
	}
}

func TestNormalizeRegionCode(t *testing.T) {
	cases := map[string]string{"51": "510000", "5101": "510100", "510107": "510107", "": "", "abc": "abc"}
	for in, want := range cases {
		if got := normalizeRegionCode(in); got != want {
			t.Errorf("normalizeRegionCode(%q)=%q, want %q", in, got, want)
		}
	}
}
