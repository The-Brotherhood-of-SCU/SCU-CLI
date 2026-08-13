package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/config"
)

// 校历数据源（与 Bugaoshan 一致，镜像优先）。
const (
	calendarMirrorURL = "https://gh-proxy.com/https://raw.githubusercontent.com/The-Brotherhood-of-SCU/Bugaoshan/refs/heads/main/assets/academic_calendar.json"
	calendarRemoteURL = "https://raw.githubusercontent.com/The-Brotherhood-of-SCU/Bugaoshan/main/assets/academic_calendar.json"
)

// FetchAcademicCalendar 获取校历（网络优先，失败回退本地缓存，免认证）。
// 返回展开后的格式：{semesters:[{name,startDate,totalWeeks,events:[{date,endDate?,label,tag}]}]}
func FetchAcademicCalendar() (map[string]interface{}, error) {
	for _, u := range []string{calendarMirrorURL, calendarRemoteURL} {
		if data, err := fetchCalendarFrom(u); err == nil {
			_ = cacheCalendar(data.raw)
			return data.expanded, nil
		}
	}
	// 网络失败：回退本地缓存。
	if cached, err := readCachedCalendar(); err == nil {
		return cached, nil
	}
	return nil, errors.New("校历获取失败（网络不可用且无本地缓存）")
}

type calendarResult struct {
	raw      string
	expanded map[string]interface{}
}

func fetchCalendarFrom(u string) (*calendarResult, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var decoded map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if _, ok := decoded["semesters"]; !ok {
		return nil, errors.New("缺少 semesters 字段")
	}
	expanded := expandCalendarJSON(decoded)
	semesters, _ := expanded["semesters"].([]interface{})
	if len(semesters) == 0 {
		return nil, errors.New("空校历")
	}
	raw, _ := json.Marshal(decoded)
	return &calendarResult{raw: string(raw), expanded: expanded}, nil
}

// expandCalendarJSON 展开紧凑格式（eventTypes 注册表）为展开格式；已展开则原样返回。
func expandCalendarJSON(compact map[string]interface{}) map[string]interface{} {
	types, ok := compact["eventTypes"].(map[string]interface{})
	if !ok {
		return compact
	}
	var out []interface{}
	for _, s := range toSlice(compact["semesters"]) {
		sem, _ := s.(map[string]interface{})
		if sem == nil {
			continue
		}
		eventsMap, _ := sem["e"].(map[string]interface{})
		var events []interface{}
		for key, value := range eventsMap {
			typeInfo, _ := types[key].(map[string]interface{})
			if typeInfo == nil {
				continue
			}
			event := map[string]interface{}{
				"label": typeInfo["l"],
				"tag":   typeInfo["t"],
			}
			switch v := value.(type) {
			case string:
				event["date"] = v
			case []interface{}:
				if len(v) >= 2 {
					event["date"] = v[0]
					event["endDate"] = v[1]
				}
			}
			events = append(events, event)
		}
		if events == nil {
			events = []interface{}{}
		}
		out = append(out, map[string]interface{}{
			"name":       sem["n"],
			"startDate":  sem["s"],
			"totalWeeks": sem["w"],
			"events":     events,
		})
	}
	return map[string]interface{}{"semesters": out}
}

func toSlice(v interface{}) []interface{} {
	if list, ok := v.([]interface{}); ok {
		return list
	}
	return nil
}

func calendarCachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cached_academic_calendar.json"), nil
}

func cacheCalendar(raw string) error {
	path, err := calendarCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(raw), 0o600)
}

func readCachedCalendar() (map[string]interface{}, error) {
	path, err := calendarCachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	return expandCalendarJSON(decoded), nil
}
