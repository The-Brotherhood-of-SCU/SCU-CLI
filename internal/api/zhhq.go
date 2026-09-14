package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// ZhhqService 智慧后勤在线报修业务 API（zhhq.scu.edu.cn/api），
// 移植自 Bugaoshan zhhq_api_service.dart。
//
// 认证：每次请求生成全新 Token 头（AES 加密 {tokenKey, clientId, timestamp,
// GUID}）+ TokenKey 头；响应体为 AES-CBC 加密 JSON，解密后解析。
// 成功 status == "success"，业务数据在 data；错误在 errorCode/message
// （4010-4017 为 token 类错误，交给恢复边界重建会话）。
type ZhhqService struct {
	auth *auth.ZhhqAuth
}

func NewZhhqService(a *auth.ZhhqAuth) *ZhhqService { return &ZhhqService{auth: a} }

const zhhqBase = "https://zhhq.scu.edu.cn/api"

// request 是统一恢复边界：tokenKey 失效（4010-4017）时 invalidate 重建重试一次。
func (s *ZhhqService) request(fn func(client *auth.CookieClient, tokenKey string) (interface{}, error)) (interface{}, error) {
	client, tokenKey, err := s.auth.Session()
	var result interface{}
	if err == nil {
		result, err = fn(client, tokenKey)
		if err == nil {
			return result, nil
		}
	}
	if !auth.IsUnauthenticated(err) {
		return nil, err
	}
	s.auth.Invalidate()
	client, tokenKey, err = s.auth.Session()
	if err != nil {
		return nil, err
	}
	return fn(client, tokenKey)
}

// zhhqHeaders 构造业务请求头（Token 每次新生成）。
func zhhqHeaders(tokenKey string, jsonBody bool) (map[string]string, error) {
	token, err := auth.ZhhqBuildToken(tokenKey)
	if err != nil {
		return nil, err
	}
	contentType := "application/x-www-form-urlencoded; charset=UTF-8"
	if jsonBody {
		contentType = "application/json;charset=utf-8"
	}
	return map[string]string{
		"Accept":           "application/json, text/plain, */*",
		"Content-Type":     contentType,
		"Origin":           "https://zhhq.scu.edu.cn",
		"Referer":          "https://zhhq.scu.edu.cn/ihome/newrepair",
		"User-Agent":       auth.DefaultUserAgent,
		"X-Requested-With": "XMLHttpRequest",
		"Token":            token,
		"TokenKey":         tokenKey,
	}, nil
}

// decodeZhhqResponse 解密并解析 zhhq 加密响应。
func decodeZhhqResponse(body string, statusCode int) (map[string]interface{}, error) {
	if statusCode == 302 || statusCode == 401 || statusCode == 403 || strings.TrimSpace(body) == "" {
		return nil, &auth.UnauthenticatedError{Msg: "zhhq 会话已失效"}
	}
	out, err := auth.ZhhqDecodeResponse(body)
	if err != nil {
		return nil, &auth.ServiceError{Msg: "[zhhq] 响应解析失败"}
	}
	// 4010-4017 均为 token 类错误（无效/超时/签名错误），触发重新认证。
	if code, ok := toIntLooseOK(out["errorCode"]); ok && code >= 4010 && code <= 4017 {
		return nil, &auth.UnauthenticatedError{Msg: "zhhq 会话已失效"}
	}
	// 业务错误统一判定：status 明确非 success，或 errorCode 明确非 0。
	if msg := zhhqBusinessErrorMessage(out); msg != "" {
		return nil, &auth.ServiceError{Msg: msg}
	}
	return out, nil
}

// toIntLooseOK 宽松解析数字字段。
func toIntLooseOK(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case string:
		var i int
		if _, err := fmt.Sscanf(strings.TrimSpace(n), "%d", &i); err == nil {
			return i, true
		}
	}
	return 0, false
}

// zhhqFieldString 取字段的字符串形态（nil 为空串）。
func zhhqFieldString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// zhhqBusinessErrorMessage 业务错误判定，返回可展示的错误文案；无错误返回空串。
func zhhqBusinessErrorMessage(out map[string]interface{}) string {
	code := zhhqFieldString(out["errorCode"])
	status := zhhqFieldString(out["status"])
	if (status != "" && status != "success") || (code != "" && code != "0") {
		if msg := zhhqFieldString(out["message"]); msg != "" {
			return msg
		}
		return "操作失败"
	}
	return ""
}

// zhhqPostForm 发起表单 POST 并解密响应。
func zhhqPostForm(c *auth.CookieClient, path string, tokenKey string, form url.Values) (map[string]interface{}, error) {
	headers, err := zhhqHeaders(tokenKey, false)
	if err != nil {
		return nil, err
	}
	resp, err := c.PostForm(zhhqBase+path, headers, form)
	if err != nil {
		return nil, err
	}
	return decodeZhhqResponse(string(resp.Body), resp.StatusCode)
}

// zhhqPostJSON 发起 JSON POST 并解密响应。
func zhhqPostJSON(c *auth.CookieClient, path string, tokenKey string, payload interface{}) (map[string]interface{}, error) {
	headers, err := zhhqHeaders(tokenKey, true)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do("POST", zhhqBase+path, headers, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return decodeZhhqResponse(string(resp.Body), resp.StatusCode)
}

// ─── 模型（移植自 Bugaoshan repair.dart） ─────────────────────────

// RepairAddress 常用地址（getCommonAddress 返回）。
type RepairAddress struct {
	ID            string `json:"id"`
	AreaName      string `json:"area_name"`
	AddressDetail string `json:"address_detail"`
	Phone         string `json:"phone"`
	AreaID        string `json:"area_id"`
	// IsCommon 是否默认地址（"1"=默认）。
	IsCommon bool `json:"is_common"`
	// UserID 用户 id（createUser，用于「我的动态」列表筛选）。
	UserID string `json:"user_id"`
}

func parseRepairAddress(m map[string]interface{}) RepairAddress {
	return RepairAddress{
		ID:            firstString(m, "id"),
		AreaName:      firstString(m, "areaName"),
		AddressDetail: firstString(m, "addressDetail"),
		Phone:         firstString(m, "phone"),
		AreaID:        firstString(m, "areaId"),
		IsCommon:      firstString(m, "ifCommon") == "1",
		UserID:        firstString(m, "userId", "createUser"),
	}
}

// RepairProject 维修项目（getProjectByAreaId 返回的两级树节点）。
// 顶层是大类（如「水」），children 是具体项目（如「水龙头类」）。
// 提交工单时 projectID 取叶子项目的 value，projectName 用「大类/项目」。
type RepairProject struct {
	Label    string          `json:"label"`
	Value    string          `json:"value"`
	Children []RepairProject `json:"children,omitempty"`
}

func parseRepairProject(m map[string]interface{}) RepairProject {
	node := RepairProject{
		Label: firstString(m, "label"),
		Value: firstString(m, "value", "id"),
	}
	if children, ok := m["children"].([]interface{}); ok {
		for _, c := range children {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			node.Children = append(node.Children, parseRepairProject(cm))
		}
	}
	return node
}

// FindRepairProjectLeaf 在两级树中按叶子 value 查找，返回叶子与「大类/项目」全名。
func FindRepairProjectLeaf(nodes []RepairProject, value string) (*RepairProject, string) {
	for i := range nodes {
		for j := range nodes[i].Children {
			if nodes[i].Children[j].Value == value {
				return &nodes[i].Children[j], nodes[i].Label + "/" + nodes[i].Children[j].Label
			}
		}
		if nodes[i].Value == value && len(nodes[i].Children) == 0 {
			return &nodes[i], nodes[i].Label
		}
	}
	return nil, ""
}

// RepairAcceptDept 报修负责部门（getAcceptUserByAreaIdAndProjectId 返回）。
// deptId/deptName/payName 直接作为 publish 请求体的 acceptDept* / payName。
type RepairAcceptDept struct {
	DeptID   string `json:"dept_id"`
	DeptName string `json:"dept_name"`
	PayName  string `json:"pay_name"`
}

func parseRepairAcceptDept(m map[string]interface{}) RepairAcceptDept {
	return RepairAcceptDept{
		DeptID:   firstString(m, "deptId"),
		DeptName: firstString(m, "deptName"),
		PayName:  firstString(m, "payName"),
	}
}

// RepairAreaNode 报修区域树节点（getAreaTree 返回）。
type RepairAreaNode struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	FullName string           `json:"full_name"`
	Children []RepairAreaNode `json:"children,omitempty"`
}

func parseRepairAreaTree(m map[string]interface{}, parentName string) RepairAreaNode {
	node := RepairAreaNode{
		ID:       firstString(m, "id"),
		Name:     firstString(m, "name"),
		FullName: firstString(m, "name"),
	}
	if parentName != "" {
		node.FullName = parentName + "/" + node.Name
	}
	if children, ok := m["children"].([]interface{}); ok {
		for _, c := range children {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			node.Children = append(node.Children, parseRepairAreaTree(cm, node.Name))
		}
	}
	return node
}

// RepairTicket 报修工单（activeTemplateData/list「我的动态」的行）。
// status 后端直接返回中文（已关闭/待完工/待评价/已撤回）；
// content 是 JSON 字符串（维修项目/故障地点/服务单位/故障描述），
// 可能多层转义或含裸控制字符，由 decodeRepairContent 统一兼容解析。
type RepairTicket struct {
	// ID 详情接口（repairInfo/get）使用的工单 id（= 列表行的 activeId）。
	ID          string `json:"id"`
	AreaName    string `json:"area_name"`
	ProjectName string `json:"project_name"`
	ServiceUnit string `json:"service_unit,omitempty"`
	Content     string `json:"content"`
	Status      string `json:"status"`
	CreateTime  int64  `json:"create_time"`
	// ActiveTime 展示时间（YYYY-MM-DD HH:mm:ss），用于排序。
	ActiveTime string `json:"active_time,omitempty"`
}

// escapeRawControlChars 把 JSON 字符串值内部的裸控制字符转义
// （后端拼接 content 时不转义用户输入，多行描述里的裸换行会让整段 JSON 非法）。
// 只改写字符串值内部，追踪引号与反斜杠转义状态，不影响结构字符。
func escapeRawControlChars(input string) (string, bool) {
	var buf strings.Builder
	inString := false
	changed := false
	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if inString {
			switch {
			case ch == '\\':
				// 反斜杠连同其转义的下一个字符原样保留。
				buf.WriteRune(ch)
				if i+1 < len(runes) {
					i++
					buf.WriteRune(runes[i])
				}
			case ch == '"':
				inString = false
				buf.WriteRune(ch)
			case ch < 0x20:
				changed = true
				switch ch {
				case '\n':
					buf.WriteString(`\n`)
				case '\r':
					buf.WriteString(`\r`)
				case '\t':
					buf.WriteString(`\t`)
				default:
					fmt.Fprintf(&buf, `\u%04x`, ch)
				}
			default:
				buf.WriteRune(ch)
			}
		} else {
			if ch == '"' {
				inString = true
			}
			buf.WriteRune(ch)
		}
	}
	return buf.String(), changed
}

// tryJSONDecode 解码 JSON；失败时转义裸控制字符后重试。
func tryJSONDecode(input string) (interface{}, bool) {
	var decoded interface{}
	if err := json.Unmarshal([]byte(input), &decoded); err == nil {
		return decoded, true
	}
	repaired, changed := escapeRawControlChars(input)
	if !changed {
		return nil, false
	}
	if err := json.Unmarshal([]byte(repaired), &decoded); err == nil {
		return decoded, true
	}
	return nil, false
}

// decodeRepairContent 把 content 统一解析为 Map。
// 兼容：Map / JSON 字符串 / 多层转义 JSON 字符串 / 值内裸控制字符的 JSON
// （最多解 5 层防异常嵌套）。
func decodeRepairContent(content interface{}) map[string]interface{} {
	current := content
	for i := 0; i < 5; i++ {
		switch v := current.(type) {
		case map[string]interface{}:
			return v
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				return nil
			}
			decoded, ok := tryJSONDecode(trimmed)
			if !ok {
				// 非 JSON（纯文本描述）或 JSON 已损坏：不再继续解。
				return nil
			}
			if m, ok := decoded.(map[string]interface{}); ok {
				return m
			}
			// 解出的是字符串：值是更深一层的 JSON，继续循环。
			if s, ok := decoded.(string); ok {
				current = s
				continue
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

// repairDisplayContent 展示内容：优先「故障描述」；缺失时回退字段拼接；
// 绝不回退为原始 JSON 文本。
func repairDisplayContent(parsed map[string]interface{}, rawContent interface{}) string {
	if desc, ok := parsed["故障描述"].(string); ok && strings.TrimSpace(desc) != "" {
		return desc
	}
	parts := make([]string, 0, 3)
	for _, key := range []string{"维修项目", "故障地点", "服务单位"} {
		if v, ok := parsed[key].(string); ok && v != "" {
			parts = append(parts, v)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " · ")
	}
	// 完全无字段且 content 是纯文本时原样展示；
	// 看似 JSON 转义文本（含 \"）的损坏数据回退为空。
	if s, ok := rawContent.(string); ok {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" && !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed, `\"`) {
			return trimmed
		}
	}
	return ""
}

func parseRepairTicket(m map[string]interface{}) RepairTicket {
	activeID := firstString(m, "activeId", "id")
	parsed := decodeRepairContent(m["content"])
	return RepairTicket{
		ID:          activeID,
		AreaName:    firstString(parsed, "故障地点"),
		ProjectName: firstString(parsed, "维修项目"),
		ServiceUnit: firstString(parsed, "服务单位"),
		Content:     repairDisplayContent(parsed, m["content"]),
		Status:      firstString(m, "status"),
		CreateTime:  int64(toIntLoose(m["createTime"], 0)),
		ActiveTime:  firstString(m, "activeTime"),
	}
}

// RepairLogItem 工单进度时间线单条（logVOS 元素）。
type RepairLogItem struct {
	StatusName string `json:"status_name"`
	Content    string `json:"content"`
	CreateTime string `json:"create_time"`
}

// RepairFinishedInfo 工单完成信息（评价用 finishedInfo.repairId）。
type RepairFinishedInfo struct {
	RepairID     string `json:"repair_id"`
	CompleteTime string `json:"complete_time,omitempty"`
	TotalAmount  string `json:"total_amount,omitempty"`
}

// RepairTicketDetail 报修工单详情（repairInfo/get 返回）。
// 与列表行不同：status 是数字（如 "3"）、projectName/content 为纯文本。
type RepairTicketDetail struct {
	ID             string `json:"id"`
	SerialNumber   string `json:"serial_number"`
	ProjectName    string `json:"project_name"`
	Content        string `json:"content"`
	AreaName       string `json:"area_name"`
	Address        string `json:"address"`
	AcceptDeptName string `json:"accept_dept_name"`
	PayName        string `json:"pay_name,omitempty"`
	BookTimeString string `json:"book_time_string,omitempty"`
	// IfOnduty 是否允许无人时维修。
	IfOnduty     bool                `json:"if_onduty"`
	Status       string              `json:"status"`
	IfCommont    string              `json:"if_commont"`
	IfComplete   string              `json:"if_complete"`
	Logs         []RepairLogItem     `json:"logs"`
	FinishedInfo *RepairFinishedInfo `json:"finished_info,omitempty"`
}

func parseRepairTicketDetail(m map[string]interface{}) RepairTicketDetail {
	detail := RepairTicketDetail{
		ID:             firstString(m, "id"),
		SerialNumber:   firstString(m, "serialNumber"),
		ProjectName:    firstString(m, "projectName"),
		Content:        firstString(m, "content"),
		AreaName:       firstString(m, "areaName"),
		Address:        firstString(m, "address"),
		AcceptDeptName: firstString(m, "acceptDeptName"),
		PayName:        firstString(m, "payName"),
		BookTimeString: firstString(m, "bookTimeString"),
		IfOnduty:       m["ifOnduty"] == true || firstString(m, "ifOnduty") == "1" || firstString(m, "ifOnduty") == "true",
		Status:         firstString(m, "status"),
		IfCommont:      firstString(m, "ifCommont"),
		IfComplete:     firstString(m, "ifComplete"),
	}
	if logs, ok := m["logVOS"].([]interface{}); ok {
		for _, l := range logs {
			lm, ok := l.(map[string]interface{})
			if !ok {
				continue
			}
			detail.Logs = append(detail.Logs, RepairLogItem{
				StatusName: firstString(lm, "statusName"),
				Content:    firstString(lm, "content"),
				CreateTime: firstString(lm, "createTime"),
			})
		}
	}
	if fm, ok := m["finishedInfo"].(map[string]interface{}); ok {
		detail.FinishedInfo = &RepairFinishedInfo{
			RepairID:     firstString(fm, "repairId"),
			CompleteTime: firstString(fm, "completeTime"),
			TotalAmount:  firstString(fm, "totalAmount"),
		}
	}
	return detail
}

// RepairEvaluateProject 工单评价项（commontProject/getProject 返回）。
type RepairEvaluateProject struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Weight string `json:"weight"`
}

// ─── 端点 ─────────────────────────────────────────────────────────

// zhhqStringList 取 data 为字符串列表。
func zhhqStringList(out map[string]interface{}) []string {
	data, _ := out["data"].([]interface{})
	list := make([]string, 0, len(data))
	for _, e := range data {
		list = append(list, fmt.Sprint(e))
	}
	return list
}

// FetchAddresses 获取常用地址列表。
func (s *ZhhqService) FetchAddresses() ([]RepairAddress, error) {
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/repair/oneNetPublish/getCommonAddress", tokenKey, nil)
		if err != nil {
			return nil, err
		}
		data, _ := out["data"].([]interface{})
		list := make([]RepairAddress, 0, len(data))
		for _, e := range data {
			if m, ok := e.(map[string]interface{}); ok {
				list = append(list, parseRepairAddress(m))
			}
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]RepairAddress), nil
}

// FetchAreaTree 获取区域树（用于地址选择）。
func (s *ZhhqService) FetchAreaTree() ([]RepairAreaNode, error) {
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/repair/publish/getAreaTree", tokenKey, nil)
		if err != nil {
			return nil, err
		}
		data, _ := out["data"].([]interface{})
		list := make([]RepairAreaNode, 0, len(data))
		for _, e := range data {
			if m, ok := e.(map[string]interface{}); ok {
				list = append(list, parseRepairAreaTree(m, ""))
			}
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]RepairAreaNode), nil
}

// FetchProjects 按区域获取维修项目（两级树）。
func (s *ZhhqService) FetchProjects(areaID string) ([]RepairProject, error) {
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/repair/publish/getProjectByAreaId", tokenKey, url.Values{"areaId": {areaID}})
		if err != nil {
			return nil, err
		}
		data, _ := out["data"].([]interface{})
		list := make([]RepairProject, 0, len(data))
		for _, e := range data {
			if m, ok := e.(map[string]interface{}); ok {
				list = append(list, parseRepairProject(m))
			}
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]RepairProject), nil
}

// FetchBookDates 获取可预约日期（未来数天）。
func (s *ZhhqService) FetchBookDates() ([]string, error) {
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/repair/publish/getBookDate", tokenKey, nil)
		if err != nil {
			return nil, err
		}
		return zhhqStringList(out), nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]string), nil
}

// FetchBookTimes 获取某日期的可预约时段。
func (s *ZhhqService) FetchBookTimes(bookDate string) ([]string, error) {
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/repair/publish/getBookTime", tokenKey, url.Values{"bookDate": {bookDate}})
		if err != nil {
			return nil, err
		}
		return zhhqStringList(out), nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]string), nil
}

// FetchDynamicTickets 获取「我的动态」报修工单列表。
// 使用 manager/activeTemplateData/list（首页「我的动态」同源接口，比
// oneNetPublish/myList 快一个量级）；status 直接是中文文案。
// userID 为当前用户的 createUser，可从常用地址的 user_id 字段获取。
func (s *ZhhqService) FetchDynamicTickets(userID string) ([]RepairTicket, error) {
	// 与前端请求参数完全一致：search 数组每条用 searchValue（非 value）、
	// systemCode 放进 search 数组、带 order 排序参数、无分页。
	search, err := json.Marshal([]map[string]string{
		{"andOr": "and", "searchField": "createUser", "operator": "=", "searchValue": userID},
		{"andOr": "and", "searchField": "systemCode", "operator": "=", "searchValue": "newRepair"},
	})
	if err != nil {
		return nil, err
	}
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/manager/activeTemplateData/list", tokenKey, url.Values{
			"search": {string(search)},
			"order":  {"createTime desc"},
		})
		if err != nil {
			return nil, err
		}
		data, _ := out["data"].([]interface{})
		tickets := make([]RepairTicket, 0, len(data))
		for _, e := range data {
			if m, ok := e.(map[string]interface{}); ok {
				tickets = append(tickets, parseRepairTicket(m))
			}
		}
		// 接口返回顺序无序，按 activeTime 倒序（最新在前），
		// 缺失时回退 createTime 时间戳。
		sort.SliceStable(tickets, func(i, j int) bool {
			if tickets[i].ActiveTime != "" && tickets[j].ActiveTime != "" && tickets[i].ActiveTime != tickets[j].ActiveTime {
				return tickets[i].ActiveTime > tickets[j].ActiveTime
			}
			return tickets[i].CreateTime > tickets[j].CreateTime
		})
		return tickets, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]RepairTicket), nil
}

// FetchRepairDetail 获取报修工单详情。id 传入列表行的 id（activeId）。
func (s *ZhhqService) FetchRepairDetail(id string) (*RepairTicketDetail, error) {
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/repair/repairInfo/get", tokenKey, url.Values{"id": {id}})
		if err != nil {
			return nil, err
		}
		data, ok := out["data"].(map[string]interface{})
		if !ok {
			return nil, &auth.ServiceError{Msg: "报修详情数据异常"}
		}
		detail := parseRepairTicketDetail(data)
		return &detail, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*RepairTicketDetail), nil
}

// IfAllowWithdrawRepair 查询工单是否允许撤回。
func (s *ZhhqService) IfAllowWithdrawRepair(id string) (bool, error) {
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/repair/myRepair/ifAllowWithdrawMyRepair", tokenKey, url.Values{"id": {id}})
		if err != nil {
			return nil, err
		}
		data := out["data"]
		if b, ok := data.(bool); ok {
			return b, nil
		}
		s := strings.TrimSpace(fmt.Sprint(data))
		return s == "true" || s == "1", nil
	})
	if err != nil {
		return false, err
	}
	return v.(bool), nil
}

// WithdrawRepair 撤回报修工单。
func (s *ZhhqService) WithdrawRepair(id string) error {
	_, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		_, err := zhhqPostForm(c, "/repair/myRepair/withdrawMyRepair", tokenKey, url.Values{"id": {id}})
		return nil, err
	})
	return err
}

// FetchEvaluateProjects 获取工单评价项（维修质量/维修态度/维修速度等）。
func (s *ZhhqService) FetchEvaluateProjects() ([]RepairEvaluateProject, error) {
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/repair/commontProject/getProject", tokenKey, nil)
		if err != nil {
			return nil, err
		}
		data, _ := out["data"].([]interface{})
		list := make([]RepairEvaluateProject, 0, len(data))
		for _, e := range data {
			if m, ok := e.(map[string]interface{}); ok {
				list = append(list, RepairEvaluateProject{
					ID:     firstString(m, "id"),
					Name:   firstString(m, "name"),
					Weight: firstString(m, "weight"),
				})
			}
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]RepairEvaluateProject), nil
}

// RepairEvaluateInput 评价报修工单的输入。
// Projects 为 GetEvaluateProjects 返回的评价项（完整对象），每项打 star（1-5）。
type RepairEvaluateInput struct {
	RepairID string
	Projects []RepairEvaluateProject
	Stars    map[string]int
	Content  string
	Labels   []string
}

// EvaluateRepair 评价报修工单（visitEvaluateUser/save）。
// repairId 为详情 finishedInfo.repair_id。
func (s *ZhhqService) EvaluateRepair(input RepairEvaluateInput) error {
	common := make([]map[string]interface{}, 0, len(input.Projects))
	for _, p := range input.Projects {
		star, ok := input.Stars[p.ID]
		if !ok {
			star = input.Stars["*"]
		}
		common = append(common, map[string]interface{}{
			"id":     p.ID,
			"name":   p.Name,
			"weight": p.Weight,
			"star":   star,
		})
	}
	payload := map[string]interface{}{
		"common":   common,
		"content":  input.Content,
		"repairId": input.RepairID,
		"source":   "0",
		"label":    strings.Join(input.Labels, ","),
	}
	_, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		_, err := zhhqPostJSON(c, "/repair/visitEvaluateUser/save", tokenKey, payload)
		return nil, err
	})
	return err
}

// SaveAddress 保存（新增）常用报修地址。
// areaName 为级联选择得出的区域名称（如 望江学生区/东苑五栋）。
func (s *ZhhqService) SaveAddress(areaID, areaName, addressDetail, phone, userName string, isCommon bool) error {
	ifCommon := "0"
	if isCommon {
		ifCommon = "1"
	}
	payload := map[string]interface{}{
		"areaId":        areaID,
		"areaName":      areaName,
		"addressDetail": addressDetail,
		"phone":         phone,
		"userName":      userName,
		"ifCommon":      ifCommon,
		"id":            "",
	}
	_, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		_, err := zhhqPostJSON(c, "/repair/userCommonAddress/save", tokenKey, payload)
		return nil, err
	})
	return err
}

// SubmitTicket 提交报修工单（publish）。
func (s *ZhhqService) SubmitTicket(payload map[string]interface{}) error {
	_, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		_, err := zhhqPostJSON(c, "/repair/publish/publish", tokenKey, payload)
		return nil, err
	})
	return err
}

// FetchAcceptDept 提交前预取：按区域+维修项目获取负责部门与收费信息。
// 失败时返回 nil（由调用方决定是否阻断提交）。
func (s *ZhhqService) FetchAcceptDept(areaID, projectID string) (*RepairAcceptDept, error) {
	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		out, err := zhhqPostForm(c, "/repair/publish/getAcceptUserByAreaIdAndProjectId", tokenKey,
			url.Values{"areaId": {areaID}, "projectId": {projectID}})
		if err != nil {
			return nil, err
		}
		data, ok := out["data"].(map[string]interface{})
		if !ok {
			return (*RepairAcceptDept)(nil), nil
		}
		if dept, ok := data["dept"].(map[string]interface{}); ok {
			dept := parseRepairAcceptDept(dept)
			return &dept, nil
		}
		if users, ok := data["users"].([]interface{}); ok && len(users) > 0 {
			if u, ok := users[0].(map[string]interface{}); ok {
				dept := parseRepairAcceptDept(u)
				return &dept, nil
			}
		}
		return (*RepairAcceptDept)(nil), nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*RepairAcceptDept), nil
}

// UploadRepairImage 上传报修图片，返回服务端 path（提交工单的 resourcesVOS.fileUrl）。
// POST /api/file/upload（multipart：file + system=manager），
// 该接口响应是明文 JSON（不走 AES 解密）。
func (s *ZhhqService) UploadRepairImage(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("system", "manager"); err != nil {
		return "", err
	}
	part, err := mw.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	body := buf.Bytes()

	v, err := s.request(func(c *auth.CookieClient, tokenKey string) (interface{}, error) {
		headers, err := zhhqHeaders(tokenKey, false)
		if err != nil {
			return nil, err
		}
		headers["Content-Type"] = mw.FormDataContentType()
		resp, err := c.Do("POST", zhhqBase+"/file/upload", headers, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(string(resp.Body))
		if resp.StatusCode == 302 || resp.StatusCode == 401 || resp.StatusCode == 403 || trimmed == "" {
			return nil, &auth.UnauthenticatedError{Msg: "zhhq 会话已失效"}
		}
		// 明文 JSON（非 AES 加密）。
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
			return nil, &auth.ServiceError{Msg: "图片上传失败：响应解析异常"}
		}
		if msg := zhhqBusinessErrorMessage(out); msg != "" {
			return nil, &auth.ServiceError{Msg: msg}
		}
		data, ok := out["data"].(map[string]interface{})
		if !ok {
			return nil, &auth.ServiceError{Msg: "图片上传失败：响应异常"}
		}
		path := firstString(data, "path")
		if path == "" {
			return nil, &auth.ServiceError{Msg: "图片上传失败：未返回路径"}
		}
		return path, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}
