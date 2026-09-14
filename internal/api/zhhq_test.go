package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// encryptZhhqBody 测试辅助：把 JSON 加密成 zhhq 响应体。
func encryptZhhqBody(t *testing.T, payload string) string {
	t.Helper()
	body, err := auth.ZhhqEncrypt(payload, auth.ZhhqResponseKey, auth.ZhhqResponseIV)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return body
}

func TestDecodeZhhqResponse(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		status  int
		wantErr string
	}{
		{
			name:   "成功响应",
			body:   encryptZhhqBody(t, `{"status":"success","errorCode":"0","data":[{"id":1}]}`),
			status: 200,
		},
		{
			name:    "token 失效 4010",
			body:    encryptZhhqBody(t, `{"status":"error","errorCode":"4010","message":"token invalid"}`),
			status:  200,
			wantErr: "unauthenticated",
		},
		{
			name:    "token 失效 4017",
			body:    encryptZhhqBody(t, `{"status":"error","errorCode":4017,"message":"timeout"}`),
			status:  200,
			wantErr: "unauthenticated",
		},
		{
			name:    "业务错误（status 非 success）",
			body:    encryptZhhqBody(t, `{"status":"error","errorCode":"500","message":"缺少参数"}`),
			status:  200,
			wantErr: "缺少参数",
		},
		{
			name:    "业务错误（errorCode 非 0）",
			body:    encryptZhhqBody(t, `{"errorCode":"1","message":"操作失败咯"}`),
			status:  200,
			wantErr: "操作失败咯",
		},
		{
			name:    "会话过期 302",
			body:    "",
			status:  302,
			wantErr: "unauthenticated",
		},
		{
			name:    "非加密明文响应",
			body:    "<html>error</html>",
			status:  200,
			wantErr: "响应解析失败",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeZhhqResponse(tc.body, tc.status)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("意外错误: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("期望错误但没有发生")
			}
			if tc.wantErr == "unauthenticated" {
				if !auth.IsUnauthenticated(err) {
					t.Fatalf("期望 unauthenticated，得到: %v", err)
				}
			} else if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("错误 %q 不包含 %q", got, tc.wantErr)
			}
		})
	}
}

func TestDecodeRepairContent(t *testing.T) {
	// 1) 双层转义 JSON 字符串。
	double := `"\"{\\\"维修项目\\\":\\\"水龙头类\\\"}\""`
	var raw interface{}
	if err := json.Unmarshal([]byte(double), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := decodeRepairContent(raw)
	if got["维修项目"] != "水龙头类" {
		t.Fatalf("双层转义解析 = %v", got)
	}

	// 2) 值内含裸控制字符的 JSON（issue #273 形态：多行故障描述未转义）。
	broken := "{\"故障描述\":\"第一行\n第二行\t结尾\",\"维修项目\":\"门锁窗扣类\"}"
	got = decodeRepairContent(broken)
	if got["维修项目"] != "门锁窗扣类" {
		t.Fatalf("裸控制字符解析 = %v", got)
	}
	if got["故障描述"] != "第一行\n第二行\t结尾" {
		t.Fatalf("故障描述 = %q", got["故障描述"])
	}

	// 3) 已是 Map。
	got = decodeRepairContent(map[string]interface{}{"故障地点": "东苑五栋 101"})
	if got["故障地点"] != "东苑五栋 101" {
		t.Fatalf("Map 解析 = %v", got)
	}

	// 4) 非 JSON 纯文本。
	if got := decodeRepairContent("纯文本描述"); got != nil {
		t.Fatalf("纯文本应返回 nil，得到 %v", got)
	}
}

func TestParseRepairTicketContent(t *testing.T) {
	row := map[string]interface{}{
		"activeId":   "A1",
		"status":     "待完工",
		"createTime": "1757059200000",
		"activeTime": "2026-09-05 10:00:00",
		"content":    `{"维修项目":"水龙头类","故障地点":"望江东苑5栋101","故障描述":"水龙头漏水"}`,
	}
	ticket := parseRepairTicket(row)
	if ticket.ID != "A1" || ticket.Status != "待完工" {
		t.Fatalf("ticket = %+v", ticket)
	}
	if ticket.Content != "水龙头漏水" || ticket.ProjectName != "水龙头类" || ticket.AreaName != "望江东苑5栋101" {
		t.Fatalf("content 解析 = %+v", ticket)
	}
	if ticket.CreateTime != 1757059200000 {
		t.Fatalf("createTime = %d", ticket.CreateTime)
	}

	// 缺故障描述时回退字段拼接，绝不回退为原始 JSON。
	row["content"] = `{"维修项目":"水管爆裂","故障地点":"东苑1栋"}`
	ticket = parseRepairTicket(row)
	if ticket.Content != "水管爆裂 · 东苑1栋" {
		t.Fatalf("回退拼接 = %q", ticket.Content)
	}

	// 完全无法解析的损坏转义 JSON → 空串。
	row["content"] = `"{\"损坏\"`
	if ticket = parseRepairTicket(row); ticket.Content != "" {
		t.Fatalf("损坏 JSON 应回退空串，得到 %q", ticket.Content)
	}
}

func TestParseRepairAreaTree(t *testing.T) {
	raw := map[string]interface{}{
		"id": "1", "name": "望江学生区",
		"children": []interface{}{
			map[string]interface{}{"id": "11", "name": "东苑五栋"},
		},
	}
	node := parseRepairAreaTree(raw, "")
	if node.FullName != "望江学生区" {
		t.Fatalf("根节点 fullName = %q", node.FullName)
	}
	if len(node.Children) != 1 || node.Children[0].FullName != "望江学生区/东苑五栋" {
		t.Fatalf("子节点 = %+v", node.Children)
	}
}

func TestFindRepairProjectLeaf(t *testing.T) {
	tree := []RepairProject{
		{Label: "水", Value: "1", Children: []RepairProject{
			{Label: "水龙头类", Value: "101"},
			{Label: "下水道类", Value: "102"},
		}},
		{Label: "木", Value: "2", Children: []RepairProject{
			{Label: "门锁窗扣类", Value: "201"},
		}},
	}
	leaf, full := FindRepairProjectLeaf(tree, "201")
	if leaf == nil || full != "木/门锁窗扣类" {
		t.Fatalf("leaf=%v full=%q", leaf, full)
	}
	if leaf, _ := FindRepairProjectLeaf(tree, "1"); leaf != nil {
		t.Fatal("大类 value 不应匹配为叶子")
	}
	if leaf, _ := FindRepairProjectLeaf(tree, "999"); leaf != nil {
		t.Fatal("不存在的 value 不应匹配")
	}
}

func TestParseRepairTicketDetail(t *testing.T) {
	raw := map[string]interface{}{
		"id":           "A1",
		"serialNumber": "202609030009",
		"status":       "3",
		"ifOnduty":     true,
		"ifCommont":    "0",
		"logVOS":       []interface{}{map[string]interface{}{"statusName": "派工", "content": "已指派", "createTime": "2026-09-03 10:00:00"}},
		"finishedInfo": map[string]interface{}{"repairId": "R88", "completeTime": "2026-09-04 12:00:00"},
	}
	detail := parseRepairTicketDetail(raw)
	if detail.SerialNumber != "202609030009" || !detail.IfOnduty {
		t.Fatalf("detail = %+v", detail)
	}
	if len(detail.Logs) != 1 || detail.Logs[0].StatusName != "派工" {
		t.Fatalf("logs = %+v", detail.Logs)
	}
	if detail.FinishedInfo == nil || detail.FinishedInfo.RepairID != "R88" {
		t.Fatalf("finishedInfo = %+v", detail.FinishedInfo)
	}

	// ifOnduty 字符串形态。
	raw["ifOnduty"] = "1"
	if detail = parseRepairTicketDetail(raw); !detail.IfOnduty {
		t.Fatal("ifOnduty='1' 应为 true")
	}
}
