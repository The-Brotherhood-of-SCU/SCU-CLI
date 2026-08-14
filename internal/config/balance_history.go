package config

// 电费余额历史快照与房间绑定的本地持久化。
// balance_history.json / balance_bindings.json 均在配置目录下（0600），
// 快照保留 365 天（与 Bugaoshan _balanceHistoryRetention 一致）。

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// BalanceHistoryRetention 快照保留时长。
const BalanceHistoryRetention = 365 * 24 * time.Hour

// BalanceSnapshot 是一条余额历史快照（字段与 api.BalanceRecord 对齐，
// config 包不依赖 api 包，避免循环引用）。
type BalanceSnapshot struct {
	RoomKey     string  `json:"room_key"`
	BalanceType int     `json:"balance_type"`
	Timestamp   int64   `json:"timestamp"` // UTC 毫秒
	Balance     float64 `json:"balance"`
	Price       float64 `json:"price"`
}

// BalanceBinding 是 balance query 绑定过的房间（按类型各存一份）。
type BalanceBinding struct {
	SchoolCode string `json:"school_code"`
	RegCode    string `json:"reg_code"`
	UnitCode   string `json:"unit_code"`
	RoomNo     string `json:"room_no"`
}

// RoomKey 房间标识：仅由房间属性构成，同房间跨账号共享历史。
func (b BalanceBinding) RoomKey() string {
	return b.SchoolCode + "_" + b.RegCode + "_" + b.UnitCode + "_" + b.RoomNo
}

type balanceHistoryFile struct {
	Records []BalanceSnapshot `json:"records"`
}

func balanceHistoryPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "balance_history.json"), nil
}

func balanceBindingsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "balance_bindings.json"), nil
}

// writeJSON0600 原子写 JSON（tmp + rename，与 SaveCredentials 一致）。
func writeJSON0600(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// AppendBalanceSnapshot 追加一条快照并清理超过保留期的记录。
func AppendBalanceSnapshot(s BalanceSnapshot) error {
	path, err := balanceHistoryPath()
	if err != nil {
		return err
	}
	var f balanceHistoryFile
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("解析余额历史失败: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取余额历史失败: %w", err)
	}
	cutoff := time.Now().UTC().Add(-BalanceHistoryRetention).UnixMilli()
	kept := f.Records[:0]
	for _, r := range f.Records {
		if r.Timestamp >= cutoff {
			kept = append(kept, r)
		}
	}
	f.Records = append(kept, s)
	return writeJSON0600(path, f)
}

// LoadBalanceSnapshots 读取指定房间与类型的快照（按时间升序）。
func LoadBalanceSnapshots(roomKey string, balanceType int) ([]BalanceSnapshot, error) {
	path, err := balanceHistoryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []BalanceSnapshot{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取余额历史失败: %w", err)
	}
	var f balanceHistoryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("解析余额历史失败: %w", err)
	}
	out := make([]BalanceSnapshot, 0, len(f.Records))
	for _, r := range f.Records {
		if r.RoomKey == roomKey && r.BalanceType == balanceType {
			out = append(out, r)
		}
	}
	return out, nil
}

// SaveBalanceBinding 保存某类型最后绑定的房间。
func SaveBalanceBinding(balanceType int, b BalanceBinding) error {
	bindings, err := loadBalanceBindings()
	if err != nil {
		return err
	}
	bindings[strconv.Itoa(balanceType)] = b
	path, err := balanceBindingsPath()
	if err != nil {
		return err
	}
	return writeJSON0600(path, bindings)
}

// LoadBalanceBinding 读取某类型最后绑定的房间；未绑定过返回 ok=false。
func LoadBalanceBinding(balanceType int) (BalanceBinding, bool, error) {
	bindings, err := loadBalanceBindings()
	if err != nil {
		return BalanceBinding{}, false, err
	}
	b, ok := bindings[strconv.Itoa(balanceType)]
	return b, ok, nil
}

func loadBalanceBindings() (map[string]BalanceBinding, error) {
	path, err := balanceBindingsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]BalanceBinding{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取房间绑定失败: %w", err)
	}
	var out map[string]BalanceBinding
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("解析房间绑定失败: %w", err)
	}
	if out == nil {
		out = map[string]BalanceBinding{}
	}
	return out, nil
}
