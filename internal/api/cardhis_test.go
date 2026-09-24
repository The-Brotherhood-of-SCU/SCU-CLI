package api

import (
	"encoding/json"
	"testing"
)

// 卡片流水响应结构实测自 /cardhis/wap/default/get-index（2026-09）。
const cardhisFixture = `{
  "e": 0,
  "m": "操作成功",
  "d": {
    "list": {
      "2026-09-23": [
        {"toname": "饮食中心\\江安三食堂\\232号机", "jdate": 1790159380, "xfje": 3.6, "syje": "226.51", "type": "消费", "time": "18:29:40"},
        {"toname": "饮食中心\\江安三食堂\\231号机", "jdate": 1790159401, "xfje": 8, "syje": 218.51, "type": "消费", "time": "18:30:01"}
      ],
      "2026-09-22": [
        {"toname": "饮食中心\\江安馨苑\\3号机", "jdate": 1790073524, "xfje": 10.6, "syje": "247.61", "type": "消费", "time": "18:38:44"},
        {"toname": "财务处", "jdate": 1790030000, "xfje": 100, "syje": "258.21", "type": "充值", "time": "15:13:20"}
      ]
    }
  }
}`

func TestParseCardhisResponse(t *testing.T) {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(cardhisFixture), &root); err != nil {
		t.Fatal(err)
	}
	h, err := parseCardhisResponse(root)
	if err != nil {
		t.Fatal(err)
	}

	if h.TxnCount != 4 {
		t.Errorf("txn_count = %d, want 4", h.TxnCount)
	}

	// 明细按时间升序
	if h.Transactions[0].Jdate != 1790030000 {
		t.Errorf("first jdate = %d, want 1790030000", h.Transactions[0].Jdate)
	}

	// syje 字符串与数字都能解析
	if h.Transactions[0].Syje != 258.21 {
		t.Errorf("syje = %v, want 258.21", h.Transactions[0].Syje)
	}

	// 充值不计入消费合计
	if h.TotalCost != 22.2 {
		t.Errorf("total_cost = %v, want 22.2", h.TotalCost)
	}
	if h.ByDay["2026-09-22"] != 10.6 {
		t.Errorf("by_day[09-22] = %v, want 10.6", h.ByDay["2026-09-22"])
	}

	// 余额取最新一条
	if h.Balance == nil || *h.Balance != 218.51 {
		t.Errorf("balance = %v, want 218.51", h.Balance)
	}
	if h.BalanceAsOf == "" {
		t.Error("balance_as_of 应非空")
	}
}

func TestParseCardhisResponseEmpty(t *testing.T) {
	root := map[string]interface{}{
		"e": float64(0),
		"d": map[string]interface{}{"list": map[string]interface{}{}},
	}
	h, err := parseCardhisResponse(root)
	if err != nil {
		t.Fatal(err)
	}
	if h.TxnCount != 0 || h.Balance != nil {
		t.Errorf("空响应应得到零结果, got %+v", h)
	}
}
