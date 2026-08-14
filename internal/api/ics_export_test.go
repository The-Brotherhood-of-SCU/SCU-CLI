package api

import (
	"strings"
	"testing"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/ics"
)

func TestParseClassWeekSegments(t *testing.T) {
	cases := []struct {
		in   string
		want []WeekSegment
	}{
		{"111100", []WeekSegment{{1, 4, WeekEvery}}},
		{"1010", []WeekSegment{{1, 3, WeekOdd}}},
		{"0101", []WeekSegment{{2, 4, WeekEven}}},
		{"1101", []WeekSegment{{1, 2, WeekEvery}, {4, 4, WeekEvery}}},
		{"1", []WeekSegment{{1, 1, WeekEvery}}},
		{"", nil},
		{"0000", nil},
		// 交错但有缺口（1,3,6）：不完整交错，按连续段拆
		{"101001", []WeekSegment{{1, 1, WeekEvery}, {3, 3, WeekEvery}, {6, 6, WeekEvery}}},
	}
	for _, c := range cases {
		got := ParseClassWeekSegments(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("%q: got %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%q: got %v, want %v", c.in, got, c.want)
			}
		}
	}
}

func TestDateForCourseDaySundayStart(t *testing.T) {
	// 2021-08-29 是周日（校历真实场景：学期从周日开始，周日属于第一周）
	start := time.Date(2021, 8, 29, 0, 0, 0, 0, ics.Shanghai())
	cases := []struct {
		week, dow int
		want      string
	}{
		{1, 7, "2021-08-29"}, // 第一周周日 = 起始日当天
		{1, 1, "2021-08-30"}, // 第一周周一
		{1, 6, "2021-09-04"},
		{2, 1, "2021-09-06"},
		{2, 7, "2021-09-05"},
		{16, 5, "2021-12-17"},
	}
	for _, c := range cases {
		got := dateForCourseDay(start, c.week, c.dow).Format("2006-01-02")
		if got != c.want {
			t.Errorf("week=%d dow=%d: got %s, want %s", c.week, c.dow, got, c.want)
		}
	}
}

func TestDateForCourseDayMondayStart(t *testing.T) {
	// 周一开学：2025-09-01 是周一
	start := time.Date(2025, 9, 1, 0, 0, 0, 0, ics.Shanghai())
	if got := dateForCourseDay(start, 1, 1).Format("2006-01-02"); got != "2025-09-01" {
		t.Errorf("got %s", got)
	}
	if got := dateForCourseDay(start, 1, 7).Format("2006-01-02"); got != "2025-08-31" {
		t.Errorf("第一周周日应为开学前一天: got %s", got)
	}
}

func TestParseScheduleCourses(t *testing.T) {
	raw := map[string]interface{}{
		"xkxx": []interface{}{
			map[string]interface{}{
				"001": map[string]interface{}{
					"courseName":         "高等数学",
					"id":                 map[string]interface{}{"coureSequenceNumber": "01"},
					"attendClassTeacher": "张三",
					"timeAndPlaceList": []interface{}{
						map[string]interface{}{
							"classDay":             float64(1),
							"classSessions":        float64(3),
							"continuingSession":    float64(2),
							"teachingBuildingName": "江安一教A",
							"classroomName":        "101",
							"classWeek":            "1111",
						},
					},
				},
			},
		},
	}
	courses := ParseScheduleCourses(raw)
	if len(courses) != 1 {
		t.Fatalf("want 1 course, got %d", len(courses))
	}
	c := courses[0]
	if c.Name != "高等数学 (01)" || c.Teacher != "张三" || c.Location != "江安一教A101" {
		t.Errorf("unexpected course: %+v", c)
	}
	if c.DayOfWeek != 1 || c.StartSection != 3 || c.EndSection != 4 {
		t.Errorf("unexpected slot: %+v", c)
	}
	if len(c.Segments) != 1 || c.Segments[0] != (WeekSegment{1, 4, WeekEvery}) {
		t.Errorf("unexpected segments: %+v", c.Segments)
	}
}

func TestBuildScheduleEventsCampusDetection(t *testing.T) {
	courses := []CourseSlot{{
		Name: "高等数学", Teacher: "张三", Location: "江安一教A101",
		DayOfWeek: 1, StartSection: 3, EndSection: 4,
		Segments: []WeekSegment{{1, 4, WeekEvery}},
	}}
	start := time.Date(2025, 9, 1, 0, 0, 0, 0, ics.Shanghai())
	events, truncated := BuildScheduleEvents(courses, start, "")
	if truncated != 0 || len(events) != 4 {
		t.Fatalf("events=%d truncated=%d", len(events), truncated)
	}
	// 江安第 3 节 10:15 开始，第 4 节 11:55 结束
	e := events[0]
	if e.Start.Format("2006-01-02 15:04") != "2025-09-01 10:15" || e.End.Format("15:04") != "11:55" {
		t.Errorf("unexpected time: %v - %v", e.Start, e.End)
	}
	if e.Geo == nil || e.Location != "四川大学江安校区 · 江安一教A101" {
		t.Errorf("unexpected location: %+v geo=%v", e.Location, e.Geo)
	}
	// UID 稳定：同一输入两次导出一致
	events2, _ := BuildScheduleEvents(courses, start, "")
	if events[0].UID != events2[0].UID {
		t.Errorf("UID 不稳定: %s vs %s", events[0].UID, events2[0].UID)
	}
	if !strings.HasSuffix(events[0].UID, "@bugaoshan") {
		t.Errorf("UID 域应为 @bugaoshan: %s", events[0].UID)
	}
}

func TestBuildScheduleEventsOddWeeks(t *testing.T) {
	courses := []CourseSlot{{
		Name: "体育", Location: "体育馆",
		DayOfWeek: 3, StartSection: 1, EndSection: 2,
		Segments: []WeekSegment{{1, 5, WeekOdd}},
	}}
	start := time.Date(2025, 9, 1, 0, 0, 0, 0, ics.Shanghai())
	events, _ := BuildScheduleEvents(courses, start, "望江") // 显式校区
	if len(events) != 3 { // 第 1/3/5 周
		t.Fatalf("want 3 events, got %d", len(events))
	}
	// 望江第 1 节 08:00
	if events[0].Start.Format("15:04") != "08:00" {
		t.Errorf("望江时段表未生效: %v", events[0].Start)
	}
}

func TestBuildExamEvents(t *testing.T) {
	exams := []ExamInfo{
		{CourseName: "高等数学（已结束）", Week: "第 18 周", Date: "2026-06-30", Weekday: "星期二",
			TimeRange: "08:30-10:30", Location: "江安综合楼B404", SeatNumber: "12", TicketNumber: "12345", Tip: "带计算器"},
		{CourseName: "数据结构考试", Week: "第 19 周", Date: "未知", TimeRange: "未知", Location: "望江", SeatNumber: "1", Tip: "无"},
	}
	events, skipped := BuildExamEvents(exams)
	if skipped != 1 || len(events) != 1 {
		t.Fatalf("events=%d skipped=%d", len(events), skipped)
	}
	e := events[0]
	if e.Title != "高等数学考试" {
		t.Errorf("title 应去（已结束）并补考试后缀: %q", e.Title)
	}
	if !strings.Contains(e.Description, "座位号: 12") || !strings.Contains(e.Description, "准考证号: 12345") || !strings.Contains(e.Description, "提示: 带计算器") {
		t.Errorf("description: %q", e.Description)
	}
	if !strings.HasPrefix(e.UID, "exam-") || !strings.HasSuffix(e.UID, "@bugaoshan") {
		t.Errorf("uid: %s", e.UID)
	}
}

func TestMatchSemesterStart(t *testing.T) {
	calendar := map[string]interface{}{"semesters": []interface{}{
		map[string]interface{}{"name": "2025-2026学年秋季学期", "startDate": "2025-08-31", "totalWeeks": float64(20)},
		map[string]interface{}{"name": "2025-2026学年春季学期", "startDate": "2026-03-01", "totalWeeks": float64(18)},
	}}
	name, start, weeks, err := MatchSemesterStart(calendar, "2025-2026-2-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if name != "2025-2026学年春季学期" || start.Format("2006-01-02") != "2026-03-01" || weeks != 18 {
		t.Errorf("%s %s %d", name, start, weeks)
	}
	// 空 planCode：按日期匹配所在学期
	_, start2, _, err := MatchSemesterStart(calendar, "", time.Date(2025, 10, 15, 0, 0, 0, 0, time.Local))
	if err != nil || start2.Format("2006-01-02") != "2025-08-31" {
		t.Errorf("按日期匹配失败: %v %v", start2, err)
	}
	if _, _, _, err := MatchSemesterStart(calendar, "2030-2031-1-1", time.Now()); err == nil {
		t.Error("匹配不到学期应报错")
	}
}

func TestICSBuildFormat(t *testing.T) {
	events := []ics.Event{{
		Title: "高等数学, 期末; 复习\n第二行", Location: "江安",
		Description: "教师: 张三", UID: "abc_1@bugaoshan",
		Start: time.Date(2025, 9, 1, 10, 15, 0, 0, ics.Shanghai()),
		End:   time.Date(2025, 9, 1, 11, 55, 0, 0, ics.Shanghai()),
	}}
	out := ics.Build("Course Schedule", events)
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "VERSION:2.0\r\n", "X-WR-TIMEZONE:Asia/Shanghai\r\n",
		"TZID:Asia/Shanghai\r\n", "BEGIN:VEVENT\r\n",
		"DTSTART;TZID=Asia/Shanghai:20250901T101500\r\n",
		"DTEND;TZID=Asia/Shanghai:20250901T115500\r\n",
		`SUMMARY:高等数学\, 期末\; 复习\n第二行` + "\r\n",
		"UID:abc_1@bugaoshan\r\n", "END:VCALENDAR",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ICS 缺少 %q", want)
		}
	}
}

func TestICSAllDayEvent(t *testing.T) {
	calendar := map[string]interface{}{"semesters": []interface{}{
		map[string]interface{}{"name": "2025-2026学年秋季学期", "events": []interface{}{
			map[string]interface{}{"label": "国庆节假期", "date": "2025-10-01", "endDate": "2025-10-07"},
			map[string]interface{}{"label": "教学第一周开始 / 正式行课", "date": "2025-08-31"},
		}},
	}}
	events := BuildCalendarEvents(calendar)
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	if events[0].Title != "2025-2026学年秋季学期 · 国庆节假期" {
		t.Errorf("title: %q", events[0].Title)
	}
	out := ics.Build("Academic Calendar", events)
	if !strings.Contains(out, "DTSTART;VALUE=DATE:20251001\r\n") || !strings.Contains(out, "DTEND;VALUE=DATE:20251008\r\n") {
		t.Errorf("全天事件日期范围错误:\n%s", out)
	}
}

func TestSafeFileName(t *testing.T) {
	if got := ics.SafeFileName("课表-2025-2026学年春季学期", true); got != "课表-2025-2026学年春季学期" {
		t.Errorf("got %q", got)
	}
	if got := ics.SafeFileName("a/b:c", false); got != "a_b_c" {
		t.Errorf("got %q", got)
	}
	if got := ics.SafeFileName("///", false); got != "___" {
		t.Errorf("got %q", got)
	}
	if got := ics.SafeFileName("", false); got != "calendar" {
		t.Errorf("空串应回退 calendar, got %q", got)
	}
}
