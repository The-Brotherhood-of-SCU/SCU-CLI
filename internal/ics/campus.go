package ics

import "strings"

// TimeSlot 是一节课的起止墙钟时间（1-based 节次下标见 TimeSlotsForCampus）。
type TimeSlot struct {
	StartHour, StartMin int
	EndHour, EndMin     int
}

// 节次时段预设（移植自 Bugaoshan course.dart，均为 4-5-3 结构共 12 节）。
var (
	jiangAnTimeSlots = []TimeSlot{
		{8, 15, 9, 0}, {9, 10, 9, 55}, {10, 15, 11, 0}, {11, 10, 11, 55},
		{13, 50, 14, 35}, {14, 45, 15, 30}, {15, 40, 16, 25}, {16, 45, 17, 30},
		{17, 40, 18, 25}, {19, 20, 20, 5}, {20, 15, 21, 0}, {21, 10, 21, 55},
	}
	wangJiangHuaXiTimeSlots = []TimeSlot{
		{8, 0, 8, 45}, {8, 55, 9, 40}, {10, 0, 10, 45}, {10, 55, 11, 40},
		{14, 0, 14, 45}, {14, 55, 15, 40}, {15, 50, 16, 35}, {16, 55, 17, 40},
		{17, 50, 18, 35}, {19, 30, 20, 15}, {20, 25, 21, 10}, {21, 20, 22, 5},
	}
)

// TimeSlotsForCampus 按校区名返回节次时段表；无法识别时返回 nil。
func TimeSlotsForCampus(campus string) []TimeSlot {
	if strings.Contains(campus, "江安") {
		return jiangAnTimeSlots
	}
	if strings.Contains(campus, "望江") || strings.Contains(campus, "华西") {
		return wangJiangHuaXiTimeSlots
	}
	return nil
}

// DefaultTimeSlots 是兜底时段表（江安预设，Bugaoshan 注释：most common SCU schedule）。
func DefaultTimeSlots() []TimeSlot { return jiangAnTimeSlots }

// campusGeo 校区坐标（移植自 Bugaoshan calendar_event_utils.dart）。
type campusGeo struct {
	fullName string
	lat, lng float64
	keywords []string
}

var campusGeos = []campusGeo{
	{"四川大学江安校区", 30.5601863, 103.9973029, []string{"江安"}},
	{"四川大学望江校区", 30.6335392, 104.0815556, []string{"望江"}},
	{"四川大学华西校区", 30.6425541, 104.0673888, []string{"华西"}},
}

// ResolveLocation 规范化地点文本并匹配校区：命中时标题为「校区全名 · 原地点」并附坐标。
// 与 Bugaoshan CalendarLocationMapper.resolve 一致。
func ResolveLocation(raw string) (title string, geo *Geo) {
	location := strings.Join(strings.Fields(raw), " ")
	if location == "" {
		return "", nil
	}
	for _, c := range campusGeos {
		for _, kw := range c.keywords {
			if strings.Contains(location, kw) {
				if strings.Contains(location, c.fullName) {
					return location, &Geo{Lat: c.lat, Lng: c.lng}
				}
				return c.fullName + " · " + location, &Geo{Lat: c.lat, Lng: c.lng}
			}
		}
	}
	return location, nil
}
