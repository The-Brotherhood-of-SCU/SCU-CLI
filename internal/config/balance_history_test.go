package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SCU_CLI_CONFIG_DIR", dir)
	return dir
}

func TestBalanceSnapshotAppendLoadAndPrune(t *testing.T) {
	setupTempConfigDir(t)
	now := time.Now().UTC().UnixMilli()
	old := time.Now().UTC().Add(-(BalanceHistoryRetention + 24*time.Hour)).UnixMilli()

	snaps := []BalanceSnapshot{
		{RoomKey: "1_101__101", BalanceType: 1, Timestamp: old, Balance: 99, Price: 0.5}, // 超期应被清理
		{RoomKey: "1_101__101", BalanceType: 1, Timestamp: now - 1000, Balance: 10, Price: 0.5},
		{RoomKey: "1_101__101", BalanceType: 2, Timestamp: now - 500, Balance: 20, Price: 0.6}, // 类型不同
		{RoomKey: "2_102__102", BalanceType: 1, Timestamp: now - 200, Balance: 30, Price: 0.5}, // 房间不同
		{RoomKey: "1_101__101", BalanceType: 1, Timestamp: now, Balance: 9, Price: 0.5},
	}
	for _, s := range snaps {
		if err := AppendBalanceSnapshot(s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := LoadBalanceSnapshots("1_101__101", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("应加载 2 条（超期/异型/异房均排除）, got %d: %+v", len(got), got)
	}
	if got[0].Balance != 10 || got[1].Balance != 9 {
		t.Errorf("快照内容不符: %+v", got)
	}

	// 文件层面超期记录也应被清除（而非仅加载时过滤）
	data, err := os.ReadFile(filepath.Join(os.Getenv("SCU_CLI_CONFIG_DIR"), "balance_history.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("历史文件应为非空")
	}
	all, err := LoadBalanceSnapshots("1_101__101", 2)
	if err != nil || len(all) != 1 {
		t.Errorf("类型 2 应有 1 条: %v %v", all, err)
	}
}

func TestLoadBalanceSnapshotsEmpty(t *testing.T) {
	setupTempConfigDir(t)
	got, err := LoadBalanceSnapshots("nope", 1)
	if err != nil || len(got) != 0 {
		t.Errorf("无文件应返回空列表: %v %v", got, err)
	}
}

func TestBalanceBindingRoundTrip(t *testing.T) {
	setupTempConfigDir(t)
	if _, ok, err := LoadBalanceBinding(1); err != nil || ok {
		t.Errorf("未绑定应 ok=false: %v %v", ok, err)
	}
	b := BalanceBinding{SchoolCode: "1", RegCode: "101", UnitCode: "", RoomNo: "101"}
	if b.RoomKey() != "1_101__101" {
		t.Errorf("RoomKey=%q", b.RoomKey())
	}
	if err := SaveBalanceBinding(1, b); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadBalanceBinding(1)
	if err != nil || !ok || got != b {
		t.Errorf("绑定往返不符: %+v %v %v", got, ok, err)
	}
	// 类型 2 独立
	if _, ok, _ := LoadBalanceBinding(2); ok {
		t.Error("类型 2 不应有绑定")
	}
}
