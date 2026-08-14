package api

import (
	"testing"
	"time"
)

func rec(ts time.Time, balance, price float64) BalanceRecord {
	return BalanceRecord{RoomKey: "1_101__101", BalanceType: 1, Timestamp: ts.UTC().UnixMilli(), Balance: balance, Price: price}
}

func TestBeijingDayBucket(t *testing.T) {
	// UTC 2026-08-14 17:00 = 北京 2026-08-15 01:00，bucket 应为 08-15
	b := BeijingDayBucket(time.Date(2026, 8, 14, 17, 0, 0, 0, time.UTC))
	if b.Format("2006-01-02") != "2026-08-15" {
		t.Errorf("bucket=%s, want 2026-08-15", b.Format("2006-01-02"))
	}
	// UTC 2026-08-14 15:59 = 北京 08-14 23:59，仍为 08-14
	b = BeijingDayBucket(time.Date(2026, 8, 14, 15, 59, 0, 0, time.UTC))
	if b.Format("2006-01-02") != "2026-08-14" {
		t.Errorf("bucket=%s, want 2026-08-14", b.Format("2006-01-02"))
	}
}

func TestCalculateBalanceTrendEmpty(t *testing.T) {
	res := CalculateBalanceTrend(nil)
	if res.RecordCount != 0 || res.DailyAvgCost != 0 || res.CurrentPrice != 0 || len(res.DailyPoints) != 0 {
		t.Errorf("空记录应全零: %+v", res)
	}
}

func TestCalculateBalanceTrendDailyAggregation(t *testing.T) {
	// 同一天三条（北京时间），取最后一条 95；跨北京日界的 UTC 记录应归到正确日。
	records := []BalanceRecord{
		rec(time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC), 100, 0.5),  // 北京 08-10 09:00
		rec(time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC), 98, 0.5),   // 北京 08-10 17:00
		rec(time.Date(2026, 8, 10, 16, 30, 0, 0, time.UTC), 95, 0.5), // 北京 08-11 00:30 → 08-11
		rec(time.Date(2026, 8, 12, 3, 0, 0, 0, time.UTC), 90, 0.5),   // 北京 08-12 11:00
	}
	res := CalculateBalanceTrend(records)
	if len(res.DailyPoints) != 3 {
		t.Fatalf("日代表点=%d, want 3（08-10 / 08-11 / 08-12）", len(res.DailyPoints))
	}
	if res.DailyPoints[0].Balance != 98 {
		t.Errorf("08-10 应取当日最后一条 98, got %v", res.DailyPoints[0].Balance)
	}
	// 段1: 98→95 耗 3 度, dt=0.5d；段2: 95→90 耗 5 度, dt≈1.104d
	// totalKwh=8, totalCost=8*0.5=4
	if res.TotalKwh != 8 || res.TotalCost != 4 {
		t.Errorf("累计不符: kwh=%v cost=%v", res.TotalKwh, res.TotalCost)
	}
	if res.SkippedRechargeSegments != 0 || res.RecordCount != 4 {
		t.Errorf("计数不符: %+v", res)
	}
	if res.CurrentPrice != 0.5 {
		t.Errorf("currentPrice=%v", res.CurrentPrice)
	}
	if res.FirstRecordTime.Format("2006-01-02 15:04") != "2026-08-10 01:00" {
		t.Errorf("firstRecordTime 应取自原始记录: %v", res.FirstRecordTime)
	}
}

func TestCalculateBalanceTrendSkipsRecharge(t *testing.T) {
	records := []BalanceRecord{
		rec(time.Date(2026, 8, 10, 2, 0, 0, 0, time.UTC), 100, 0.5),
		rec(time.Date(2026, 8, 11, 2, 0, 0, 0, time.UTC), 90, 0.5),
		rec(time.Date(2026, 8, 12, 2, 0, 0, 0, time.UTC), 150, 0.5), // 充值 +60
		rec(time.Date(2026, 8, 13, 2, 0, 0, 0, time.UTC), 140, 0.5),
	}
	res := CalculateBalanceTrend(records)
	if res.SkippedRechargeSegments != 1 {
		t.Errorf("应跳过 1 个充值段, got %d", res.SkippedRechargeSegments)
	}
	// 只计入 100→90 与 150→140 两段：各 10 度、各 1 天
	if res.TotalKwh != 20 || res.TotalDays != 2 {
		t.Errorf("充值段不应计入: kwh=%v days=%v", res.TotalKwh, res.TotalDays)
	}
	if res.DailyAvgKwh != 10 || res.DailyAvgCost != 5 {
		t.Errorf("日均不符: %+v", res)
	}
}

func TestCalculateBalanceTrendPriceInterpolation(t *testing.T) {
	// 电价波动时段均价取两端平均 (0.5+0.6)/2=0.55
	records := []BalanceRecord{
		rec(time.Date(2026, 8, 10, 2, 0, 0, 0, time.UTC), 100, 0.5),
		rec(time.Date(2026, 8, 12, 2, 0, 0, 0, time.UTC), 80, 0.6),
	}
	res := CalculateBalanceTrend(records)
	if res.TotalKwh != 20 || res.TotalDays != 2 {
		t.Fatalf("段统计不符: %+v", res)
	}
	if res.TotalCost != 20*0.55 {
		t.Errorf("totalCost=%v, want 11（pAvg=0.55）", res.TotalCost)
	}
	if res.CurrentPrice != 0.6 {
		t.Errorf("currentPrice 应为最后日代表点价格 0.6, got %v", res.CurrentPrice)
	}
}

func TestCalculateBalanceTrendSinglePoint(t *testing.T) {
	res := CalculateBalanceTrend([]BalanceRecord{rec(time.Date(2026, 8, 10, 2, 0, 0, 0, time.UTC), 42, 0.55)})
	if res.RecordCount != 1 || len(res.DailyPoints) != 1 || res.TotalDays != 0 {
		t.Errorf("单点应零统计: %+v", res)
	}
	if res.DailyAvgCost != 0 || res.DailyAvgKwh != 0 {
		t.Errorf("totalDays==0 时日均应为 0: %+v", res)
	}
	if res.CurrentPrice != 0.55 {
		t.Errorf("currentPrice=%v", res.CurrentPrice)
	}
}
