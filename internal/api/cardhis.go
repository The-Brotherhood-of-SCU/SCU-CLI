package api

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// 校园卡（电子卡）交易记录。
//
// 数据源：微服务「电子卡交易记录」轻应用（appkey: cardhis，
// 页面 /site/idCard/history）。接口与页面一致：
// POST /cardhis/wap/default/get-index（form-urlencoded，sdate/edate 可空）。
// 响应 d.list 以 "YYYY-MM-DD" 为 key 分组，字段：
//   toname 消费地点（含机器号）、jdate unix 秒、xfje 消费金额、
//   syje 消费后余额、type 类型（消费/充值等）、time HH:mm:ss。

// CardTxn 是单条校园卡交易。
type CardTxn struct {
	Toname string  `json:"toname"` // 消费地点（含末级机器号）
	Jdate  int64   `json:"jdate"`  // unix 秒
	Xfje   float64 `json:"xfje"`   // 消费金额（元）
	Syje   float64 `json:"syje"`   // 交易后余额（元）
	Type   string  `json:"type"`   // 消费 / 充值 等
	Time   string  `json:"time"`   // HH:mm:ss（服务端原文）
	Day    string  `json:"day"`    // 归属日期 YYYY-MM-DD
}

// CardHistory 是一次查询结果：按日期升序的平铺明细 + 汇总。
type CardHistory struct {
	Transactions []CardTxn          `json:"transactions"`  // 按时间升序
	ByDay        map[string]float64 `json:"by_day"`        // 每日消费合计（仅消费类）
	TotalCost    float64            `json:"total_cost"`    // 区间消费合计（仅消费类）
	TxnCount     int                `json:"txn_count"`     // 明细条数
	Balance      *float64           `json:"balance"`       // 最新一条记录的余额（无记录为 null）
	BalanceAsOf  string             `json:"balance_as_of"` // 余额对应交易时间（本地时区）
}

// FetchCardHistory 查询校园卡交易记录。sdate/edate 形如 "2026-09-01"，
// 均可传空串（服务端默认返回最近若干天）。
func (s *WfwService) FetchCardHistory(sdate, edate string) (*CardHistory, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		form := url.Values{"sdate": {sdate}, "edate": {edate}}
		resp, err := c.PostForm(wfwBase+"/cardhis/wap/default/get-index", nil, form)
		if err != nil {
			return nil, err
		}
		json, err := decodeWfwResponse(string(resp.Body), resp.StatusCode)
		if err != nil {
			return nil, err
		}
		return json, nil
	})
	if err != nil {
		return nil, err
	}
	root, _ := v.(map[string]interface{})
	if !wfwSuccess(root) {
		return nil, wfwBusinessError(root, "[cardhis] 查询失败")
	}
	return parseCardhisResponse(root)
}

// parseCardhisResponse 从 get-index 响应中解析明细（与网络解耦，便于单测）。
func parseCardhisResponse(root map[string]interface{}) (*CardHistory, error) {
	d, _ := root["d"].(map[string]interface{})
	list, _ := d["list"].(map[string]interface{})
	if d == nil || list == nil {
		return nil, &auth.ServiceError{Msg: "[cardhis] 响应缺少 d.list"}
	}

	out := &CardHistory{ByDay: map[string]float64{}, Transactions: []CardTxn{}}
	for day, rawRows := range list {
		rows, ok := rawRows.([]interface{})
		if !ok {
			continue
		}
		for _, rawRow := range rows {
			row, ok := rawRow.(map[string]interface{})
			if !ok {
				continue
			}
			txn := CardTxn{Day: day, Toname: jsonStr(row["toname"]), Type: jsonStr(row["type"])}
			txn.Jdate = int64(jsonNum(row["jdate"]))
			txn.Xfje = jsonNum(row["xfje"])
			txn.Syje = jsonNum(row["syje"])
			txn.Time = jsonStr(row["time"])
			out.Transactions = append(out.Transactions, txn)
			if strings.EqualFold(txn.Type, "消费") {
				out.ByDay[day] += txn.Xfje
				out.TotalCost += txn.Xfje
			}
		}
	}
	sort.Slice(out.Transactions, func(i, j int) bool {
		return out.Transactions[i].Jdate < out.Transactions[j].Jdate
	})
	out.TxnCount = len(out.Transactions)
	if out.TxnCount > 0 {
		last := out.Transactions[out.TxnCount-1]
		bal := last.Syje
		out.Balance = &bal
		out.BalanceAsOf = time.Unix(last.Jdate, 0).Format("2006-01-02 15:04:05")
	}
	return out, nil
}

func jsonStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func jsonNum(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f
	}
	return 0
}
