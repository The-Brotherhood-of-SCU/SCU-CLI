package api

// 电费/空调余额趋势计算（移植自 Bugaoshan balance_trend_calculator.dart /
// beijing_time.dart）。
//
// 原始快照按"北京日历日（UTC+8）"聚合，每日取最后一条作为日代表点，
// 再对相邻日代表点按段加权平均：
//   段消耗 Δb = b_a - b_b（余额减少为正），段均价 p_avg = (p_a + p_b) / 2；
//   充值段（Δb < 0）跳过不计入消耗；日均 = Σ(Δb × p_avg) / Σ(Δt_days)。
// 存储统一为 UTC 毫秒，仅聚合/日界判定固定 +8h 偏移。

import (
	"sort"
	"time"
)

// BalanceRecord 是一条余额快照（每次 balance query 成功后记录）。
type BalanceRecord struct {
	RoomKey     string  `json:"room_key"`     // schoolCode_regCode_unitCode_roomNo
	BalanceType int     `json:"balance_type"` // 1 照明电费，2 空调电费
	Timestamp   int64   `json:"timestamp"`    // UTC 毫秒
	Balance     float64 `json:"balance"`
	Price       float64 `json:"price"`
}

// Time 返回快照的 UTC 时间。
func (r BalanceRecord) Time() time.Time { return time.UnixMilli(r.Timestamp).UTC() }

const beijingOffset = 8 * time.Hour

// BeijingDayBucket 返回 t 所在的北京日历日（UTC 午夜标记，仅作聚合 key）。
func BeijingDayBucket(t time.Time) time.Time {
	b := t.UTC().Add(beijingOffset)
	return time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
}

// FormatBeijing 按北京时区格式化 UTC 时间。
func FormatBeijing(t time.Time, layout string) string {
	return t.UTC().Add(beijingOffset).Format(layout)
}

// BalanceTrendResult 是趋势计算结果。
type BalanceTrendResult struct {
	DailyPoints             []BalanceRecord // 日代表点序列（按时间升序）
	DailyAvgCost            float64         // 日均电费（元/天）
	DailyAvgKwh             float64         // 日均消耗（度/天）
	TotalCost               float64         // 累计消耗金额（元）
	TotalKwh                float64         // 累计消耗度数（度）
	TotalDays               float64         // 统计总天数（跳过充值段）
	SkippedRechargeSegments int             // 已识别并跳过的充值段数
	RecordCount             int             // 原始记录总条数
	FirstRecordTime         time.Time       // 最早一条原始记录（无记录为零值）
	LastRecordTime          time.Time       // 最新一条原始记录
	CurrentPrice            float64         // 当前单价（最后日代表点）
}

// CalculateBalanceTrend 对原始快照（任意顺序）计算趋势。
func CalculateBalanceTrend(records []BalanceRecord) BalanceTrendResult {
	if len(records) == 0 {
		return BalanceTrendResult{DailyPoints: []BalanceRecord{}}
	}
	sorted := make([]BalanceRecord, len(records))
	copy(sorted, records)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Timestamp < sorted[j].Timestamp })

	// 1. 按北京日历日聚合，取每日最后一条（严格 isAfter，同刻保留先到者）。
	dailyMap := map[time.Time]BalanceRecord{}
	for _, r := range sorted {
		day := BeijingDayBucket(r.Time())
		if existing, ok := dailyMap[day]; !ok || r.Time().After(existing.Time()) {
			dailyMap[day] = r
		}
	}
	dailyPoints := make([]BalanceRecord, 0, len(dailyMap))
	for _, r := range dailyMap {
		dailyPoints = append(dailyPoints, r)
	}
	sort.Slice(dailyPoints, func(i, j int) bool { return dailyPoints[i].Timestamp < dailyPoints[j].Timestamp })

	// 2. 按段加权平均。
	var totalCost, totalKwh, totalDays float64
	skippedRecharge := 0
	for i := 0; i+1 < len(dailyPoints); i++ {
		a, b := dailyPoints[i], dailyPoints[i+1]
		dt := float64(b.Time().Sub(a.Time()).Minutes()) / 1440.0
		if dt <= 0 {
			continue
		}
		db := a.Balance - b.Balance
		if db < 0 { // 余额上升说明发生充值，跳过该段
			skippedRecharge++
			continue
		}
		pAvg := (a.Price + b.Price) / 2
		totalCost += db * pAvg
		totalKwh += db
		totalDays += dt
	}

	res := BalanceTrendResult{
		DailyPoints:             dailyPoints,
		TotalCost:               totalCost,
		TotalKwh:                totalKwh,
		TotalDays:               totalDays,
		SkippedRechargeSegments: skippedRecharge,
		RecordCount:             len(sorted),
		FirstRecordTime:         sorted[0].Time(),
		LastRecordTime:          sorted[len(sorted)-1].Time(),
		CurrentPrice:            dailyPoints[len(dailyPoints)-1].Price,
	}
	if totalDays > 0 {
		res.DailyAvgCost = totalCost / totalDays
		res.DailyAvgKwh = totalKwh / totalDays
	}
	return res
}
