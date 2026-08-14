package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// ServiceHallService 网上办事大厅业务 API（service.scu.edu.cn）。
// 认证为 SSO 中继（CAS 回跳下发 PHPSESSID/vjuid/vjvd/vt 会话 cookie），
// 业务请求无 Authorization header，统一信封 {"e":0,"m":"...","d":...}。
type ServiceHallService struct {
	auth *auth.SsoRelayAuth
}

func NewServiceHallService(a *auth.SsoRelayAuth) *ServiceHallService {
	return &ServiceHallService{auth: a}
}

const serviceBase = "https://service.scu.edu.cn"

// serviceJSONHeaders 办事大厅 AJAX JSON 请求头（与 Bugaoshan 一致）。
var serviceJSONHeaders = map[string]string{
	"Accept":           "application/json, text/plain, */*",
	"Content-Type":     "application/json;charset=UTF-8",
	"Origin":           serviceBase,
	"Referer":          serviceBase,
	"User-Agent":       auth.DefaultUserAgent,
	"X-Requested-With": "XMLHttpRequest",
}

func (s *ServiceHallService) request(fn func(*auth.CookieClient) (interface{}, error)) (interface{}, error) {
	return auth.RetryOnUnauthenticated(s.auth.GetClient, fn, s.auth.Invalidate)
}

// decodeServiceResponse 办事大厅统一信封解析：
// 302/401/403/空 body 或 e==10042（数字或字符串）为会话失效；e==0 为成功。
func decodeServiceResponse(body string, statusCode int) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(body)
	if statusCode == 302 || statusCode == 401 || statusCode == 403 || trimmed == "" {
		return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
	}
	if strings.HasPrefix(trimmed, "<") && strings.Contains(body, "login") {
		return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, &auth.ServiceError{Msg: "[service] 响应解析失败"}
	}
	switch e := out["e"].(type) {
	case string:
		if e == "10042" {
			return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
		}
	case float64:
		if int(e) == 10042 {
			return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
		}
	}
	return out, nil
}

// serviceSuccess 判定 e==0（数字或字符串 "0"）。
func serviceSuccess(json map[string]interface{}) bool {
	switch e := json["e"].(type) {
	case float64:
		return int(e) == 0
	case string:
		return e == "0"
	}
	return false
}

func serviceBusinessError(json map[string]interface{}, fallback string) error {
	if m, ok := json["m"].(string); ok && m != "" {
		return &auth.ServiceError{Msg: m}
	}
	return &auth.ServiceError{Msg: fallback}
}

// serviceData 取 d 字段并校验业务成功。
func serviceData(json map[string]interface{}, fallback string) (interface{}, error) {
	if !serviceSuccess(json) {
		return nil, serviceBusinessError(json, fallback)
	}
	d := json["d"]
	if d == nil {
		return nil, serviceBusinessError(json, fallback)
	}
	return d, nil
}

// ─── 我的申请 ────────────────────────────────────────────────────

// FetchMyApplications 查询我的申请列表。
// status 为筛选档位而非实例状态值：0=全部，1=进行中+草稿，3=已完成（与服务端一致）。
// 展示状态用列表项的 inst_status 中文字段；page 从 1 开始，每页固定 20 条。
// 查询参数 key 必须全部占位（keyword/time_lower/time_upper/y/task_name 传空串）。
func (s *ServiceHallService) FetchMyApplications(status, page int) ([]interface{}, error) {
	q := url.Values{
		"p":          {fmt.Sprint(page)},
		"page_size":  {"20"},
		"status":     {fmt.Sprint(status)},
		"keyword":    {""},
		"time_lower": {""},
		"time_upper": {""},
		"y":          {""},
		"task_name":  {""},
	}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(serviceBase+"/site/process/inst-list?"+q.Encode(), serviceJSONHeaders)
		if err != nil {
			return nil, err
		}
		json, err := decodeServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := serviceData(json, "获取我的申请失败")
		if err != nil {
			return nil, err
		}
		// 兼容 d.list 与 d 本身为列表两种形态。
		if dm, ok := d.(map[string]interface{}); ok {
			if list, ok := dm["list"].([]interface{}); ok {
				return list, nil
			}
		}
		if list, ok := d.([]interface{}); ok {
			return list, nil
		}
		return []interface{}{}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}

// ─── 事项表单端点（移植自 Bugaoshan service_api_service.dart） ────

// defaultStarterDepartID 是 select-department 无结果时的兜底部门 id。
const defaultStarterDepartID = "395876"

// serviceFormHeaders 表单流程请求头（Bugaoshan 统一用 form-urlencoded Content-Type）。
var serviceFormHeaders = map[string]string{
	"Accept":           "application/json, text/plain, */*",
	"Content-Type":     "application/x-www-form-urlencoded",
	"Origin":           serviceBase,
	"Referer":          serviceBase,
	"User-Agent":       auth.DefaultUserAgent,
	"X-Requested-With": "XMLHttpRequest",
}

// FetchStarterDepartID 查询事项的发起部门（select-department）。
// 取 select==1 项的 college（候选字段 college/depart_id/department_id/value/id，
// 跳过空串与 "0"），否则第一项；请求失败上抛（供会话重试），解析无果兜底 395876。
func (s *ServiceHallService) FetchStarterDepartID(appID string) (string, error) {
	q := url.Values{"app_id": {appID}}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(serviceBase+"/site/process/select-department?"+q.Encode(), serviceFormHeaders)
		if err != nil {
			return nil, err
		}
		json, err := decodeServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := serviceData(json, "获取发起部门失败")
		if err != nil {
			return nil, err
		}
		dm, ok := d.(map[string]interface{})
		if !ok {
			return defaultStarterDepartID, nil
		}
		departs, ok := dm["depart"].([]interface{})
		if !ok || len(departs) == 0 {
			return defaultStarterDepartID, nil
		}
		pick := func(item interface{}) string {
			m, ok := item.(map[string]interface{})
			if !ok {
				return ""
			}
			for _, k := range []string{"college", "depart_id", "department_id", "value", "id"} {
				v := firstString(m, k)
				if v != "" && v != "0" {
					return v
				}
			}
			return ""
		}
		for _, item := range departs {
			if m, ok := item.(map[string]interface{}); ok && toIntLoose(m["select"], 0) == 1 {
				if id := pick(item); id != "" {
					return id, nil
				}
			}
		}
		if id := pick(departs[0]); id != "" {
			return id, nil
		}
		return defaultStarterDepartID, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// FetchStartData 获取事项发起数据（表单 id、字段权限、预填值）。
// 查询参数 node_id/agent_uid 必须传空串占位、userview 传 1。
func (s *ServiceHallService) FetchStartData(appID, starterDepartID string) (*FormDefinition, error) {
	q := url.Values{
		"app_id":            {appID},
		"node_id":           {""},
		"userview":          {"1"},
		"agent_uid":         {""},
		"starter_depart_id": {starterDepartID},
	}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(serviceBase+"/site/process/start-data?"+q.Encode(), serviceFormHeaders)
		if err != nil {
			return nil, err
		}
		json, err := decodeServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := serviceData(json, "获取发起数据失败")
		if err != nil {
			return nil, err
		}
		dm, ok := d.(map[string]interface{})
		if !ok {
			return nil, &auth.ServiceError{Msg: "start-data 响应结构异常"}
		}
		def := parseFormDefinition(dm)
		if def.FormID == "" {
			return nil, &auth.ServiceError{Msg: "start-data 缺少 currform"}
		}
		return &def, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*FormDefinition), nil
}

// FetchStartInfo 获取事项的 bpmn_id（get-formv 的入参）。
func (s *ServiceHallService) FetchStartInfo(appID string) (string, error) {
	q := url.Values{"app_id": {appID}}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(serviceBase+"/site/process/start-info?"+q.Encode(), serviceFormHeaders)
		if err != nil {
			return nil, err
		}
		json, err := decodeServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := serviceData(json, "获取事项信息失败")
		if err != nil {
			return nil, err
		}
		dm, ok := d.(map[string]interface{})
		if !ok {
			return nil, &auth.ServiceError{Msg: "start-info 响应结构异常"}
		}
		bpmnID := firstString(dm, "bpmn_id", "bpmnId")
		if bpmnID == "" {
			return nil, &auth.ServiceError{Msg: "start-info 缺少 bpmn_id"}
		}
		return bpmnID, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// FetchFormPlugins 获取表单插件定义（get-formv），返回原始 d（plugins 为双层编码 JSON）。
func (s *ServiceHallService) FetchFormPlugins(formID, bpmnID, starterDepartID string) (map[string]interface{}, error) {
	q := url.Values{
		"id":                {formID},
		"bpmn_id":           {bpmnID},
		"sess_id":           {"0"},
		"report_id":         {"0"},
		"agent_uid":         {""},
		"starter_depart_id": {starterDepartID},
	}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(serviceBase+"/site/form/get-formv?"+q.Encode(), serviceFormHeaders)
		if err != nil {
			return nil, err
		}
		json, err := decodeServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := serviceData(json, "获取表单定义失败")
		if err != nil {
			return nil, err
		}
		dm, ok := d.(map[string]interface{})
		if !ok {
			return nil, &auth.ServiceError{Msg: "get-formv 响应结构异常"}
		}
		return dm, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// FetchDataSourceValue DataSource 字段取数（POST /site/data-source/detail）。
// 返回 d.list：单列为字符串，多列为对象（由 MapConfig 分发）。
func (s *ServiceHallService) FetchDataSourceValue(appID string, ref *ServiceDataSourceRef, starterDepartID string) (interface{}, error) {
	form := url.Values{
		"id":                {ref.ID},
		"inst_id":           {"0"},
		"app_id":            {appID},
		"form_version_id":   {ref.FormVersionID},
		"component":         {ref.Component},
		"params[formId]":    {ref.FormID},
		"params[pluginKey]": {ref.Component},
		"agent_uid":         {""},
		"starter_depart_id": {starterDepartID},
	}
	for k, val := range ref.Configure {
		form["configure["+k+"]"] = []string{val}
	}
	return s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.PostForm(serviceBase+"/site/data-source/detail", serviceFormHeaders, form)
		if err != nil {
			return nil, err
		}
		json, err := decodeServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := serviceData(json, "数据源取数失败")
		if err != nil {
			return nil, err
		}
		if dm, ok := d.(map[string]interface{}); ok {
			if list, ok := dm["list"]; ok {
				return list, nil
			}
		}
		return d, nil
	})
}

// ─── 省市区字典 ──────────────────────────────────────────────────

// regionNode 是省市区字典树的节点（编码统一补齐 6 位）。
type regionNode struct {
	Label    string
	Value    string
	Children []regionNode
}

// normalizeRegionCode 区域编码右侧补 '0' 至 6 位（"51"→"510000"）。
func normalizeRegionCode(v string) string {
	if v == "" || len(v) >= 6 {
		return v
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return v
		}
	}
	return v + strings.Repeat("0", 6-len(v))
}

func parseRegionNodes(raw interface{}) []regionNode {
	var items []interface{}
	switch v := raw.(type) {
	case []interface{}:
		items = v
	case map[string]interface{}:
		for _, item := range v {
			items = append(items, item)
		}
	}
	var out []regionNode
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		label := firstString(m, "name", "label", "title")
		if label == "" {
			continue
		}
		node := regionNode{Label: label, Value: normalizeRegionCode(firstString(m, "value", "code", "id"))}
		node.Children = parseRegionNodes(m["children"])
		out = append(out, node)
	}
	return out
}

// FetchProvinces 获取省市区三级字典（/api/dictionary/province）。
// starter_depart_id 固定 395876（Bugaoshan 硬编码行为）。
func (s *ServiceHallService) FetchProvinces() ([]regionNode, error) {
	q := url.Values{"agent_uid": {""}, "starter_depart_id": {defaultStarterDepartID}}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(serviceBase+"/api/dictionary/province?"+q.Encode(), serviceFormHeaders)
		if err != nil {
			return nil, err
		}
		json, err := decodeServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		d, err := serviceData(json, "获取省市区字典失败")
		if err != nil {
			return nil, err
		}
		// 兼容 d 直接为列表 / d.list / d.children 三种形态。
		if dm, ok := d.(map[string]interface{}); ok {
			if nodes := parseRegionNodes(dm["list"]); len(nodes) > 0 {
				return nodes, nil
			}
			if nodes := parseRegionNodes(dm["children"]); len(nodes) > 0 {
				return nodes, nil
			}
		}
		if nodes := parseRegionNodes(d); len(nodes) > 0 {
			return nodes, nil
		}
		return nil, &auth.ServiceError{Msg: "省市区字典为空"}
	})
	if err != nil {
		return nil, err
	}
	return v.([]regionNode), nil
}

func findRegionNode(nodes []regionNode, nameOrCode string) *regionNode {
	if nameOrCode == "" {
		return nil
	}
	for i := range nodes {
		if nodes[i].Label == nameOrCode || nodes[i].Value == nameOrCode {
			return &nodes[i]
		}
	}
	return nil
}

func regionNodeLabels(nodes []regionNode) string {
	labels := make([]string, 0, len(nodes))
	for _, n := range nodes {
		labels = append(labels, n.Label)
	}
	return strings.Join(labels, ", ")
}

var municipalityLabels = map[string]bool{"北京市": true, "天津市": true, "上海市": true, "重庆市": true}

// ResolveRegion 把 {province, city, area, details} 标签解析为带编码的 Region 字段值
// （province/city/area 均可传标签或 6 位编码；直辖市的 city 可省略，自动取省级节点）。
func (s *ServiceHallService) ResolveRegion(input map[string]string) (map[string]interface{}, error) {
	nodes, err := s.FetchProvinces()
	if err != nil {
		return nil, err
	}
	prov := findRegionNode(nodes, input["province"])
	if prov == nil {
		return nil, &auth.ServiceError{Msg: fmt.Sprintf("省份不存在: %q（可选：%s）", input["province"], regionNodeLabels(nodes))}
	}
	city := findRegionNode(prov.Children, input["city"])
	if city == nil {
		if input["city"] == "" && municipalityLabels[prov.Label] {
			city = prov
		} else if input["city"] == prov.Label && municipalityLabels[prov.Label] {
			city = prov
		} else if input["city"] != "" {
			return nil, &auth.ServiceError{Msg: fmt.Sprintf("城市不存在: %q（%s 下可选：%s）", input["city"], prov.Label, regionNodeLabels(prov.Children))}
		}
	}
	var area *regionNode
	if city != nil {
		area = findRegionNode(city.Children, input["area"])
		if area == nil && input["area"] != "" {
			return nil, &auth.ServiceError{Msg: fmt.Sprintf("区县不存在: %q（%s 下可选：%s）", input["area"], city.Label, regionNodeLabels(city.Children))}
		}
	}
	part := func(n *regionNode) map[string]interface{} {
		if n == nil {
			return map[string]interface{}{"label": "", "value": ""}
		}
		return map[string]interface{}{"label": n.Label, "value": n.Value}
	}
	return map[string]interface{}{
		"province": part(prov),
		"city":     part(city),
		"area":     part(area),
		"details":  input["details"],
	}, nil
}

// ─── 附件上传 ────────────────────────────────────────────────────

// ServiceAttachment 是已上传附件（File_/Ximage_ 字段提交 [{name,url,id}]）。
type ServiceAttachment struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	ID   string `json:"id"`
}

// ToFormData 序列化为提交结构。
func (a ServiceAttachment) ToFormData() map[string]interface{} {
	return map[string]interface{}{"name": a.Name, "url": a.URL, "id": a.ID}
}

// UploadAttachment 上传单个附件（POST /site/attach/auth-upload，multipart 字段 upfile）。
// 该端点响应无 e 字段：{url,size,title,original,state,type,id}（可能二次编码），
// 成功判定为 url 非空或 state==SUCCESS，且必须拿到 id 以构造下载 URL。
func (s *ServiceHallService) UploadAttachment(appID, filePath string) (*ServiceAttachment, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("upfile", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	body := buf.Bytes()
	contentType := mw.FormDataContentType()
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		headers := map[string]string{
			"Accept":           "application/json, text/plain, */*",
			"Content-Type":     contentType,
			"Origin":           serviceBase,
			"Referer":          serviceBase + "/v2/matter/start?id=" + appID,
			"User-Agent":       auth.DefaultUserAgent,
			"X-Requested-With": "XMLHttpRequest",
		}
		resp, err := c.Do("POST", serviceBase+"/site/attach/auth-upload", headers, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(string(resp.Body))
		if resp.StatusCode == 302 || resp.StatusCode == 401 || resp.StatusCode == 403 || trimmed == "" {
			return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
		}
		if strings.HasPrefix(trimmed, "<") && strings.Contains(trimmed, "login") {
			return nil, &auth.UnauthenticatedError{Msg: "办事大厅登录已失效"}
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return nil, &auth.ServiceError{Msg: "[service] 附件上传响应解析失败"}
		}
		m, ok := maybeJSONString(parsed).(map[string]interface{})
		if !ok {
			return nil, &auth.ServiceError{Msg: "[service] 附件上传响应结构异常"}
		}
		urlv, _ := m["url"].(string)
		state, _ := m["state"].(string)
		id := firstString(m, "id")
		if (urlv == "" && state != "SUCCESS") || id == "" {
			if msg := firstString(m, "message", "msg", "m"); msg != "" {
				return nil, &auth.ServiceError{Msg: msg}
			}
			return nil, &auth.ServiceError{Msg: "附件上传失败"}
		}
		name := firstString(m, "original")
		if name == "" {
			name = filepath.Base(filePath)
		}
		return &ServiceAttachment{
			Name: name,
			URL:  serviceBase + "/site/attach/auth-download?file_id=" + id,
			ID:   id,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*ServiceAttachment), nil
}

// ─── 提交 ────────────────────────────────────────────────────────

// SubmitMatter 提交事项（POST /site/process/launch）。
// data 为 JSON 字符串 {app_id, node_id:”, form_data, userview:1}（userview 为数字）。
func (s *ServiceHallService) SubmitMatter(appID, starterDepartID string, formData map[string]interface{}) error {
	payload, err := json.Marshal(map[string]interface{}{
		"app_id":    appID,
		"node_id":   "",
		"form_data": formData,
		"userview":  1,
	})
	if err != nil {
		return err
	}
	form := url.Values{
		"data":              {string(payload)},
		"step":              {"0"},
		"agent_uid":         {""},
		"starter_depart_id": {starterDepartID},
	}
	_, err = s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.PostForm(serviceBase+"/site/process/launch", serviceFormHeaders, form)
		if err != nil {
			return nil, err
		}
		json, err := decodeServiceResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		if !serviceSuccess(json) {
			return nil, serviceBusinessError(json, "事项提交失败")
		}
		return true, nil
	})
	return err
}
