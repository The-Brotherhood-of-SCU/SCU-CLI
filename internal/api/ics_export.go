package api

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/ics"
)

// ─── 课表解析（xkxx → 课程槽位） ─────────────────────────────────

// WeekType 周次类型（与 Bugaoshan WeekType 对应）。
type WeekType int

const (
	WeekEvery WeekType = iota
	WeekOdd
	WeekEven
)

// WeekSegment 是一段连续周次区间。
type WeekSegment struct {
	StartWeek int
	EndWeek   int
	Type      WeekType
}

// CourseSlot 是一门课的一个上课安排（星期几 + 节次 + 周次区间集合）。
type CourseSlot struct {
	Name         string // 课程名（含课序号后缀，与 App 一致）
	Teacher      string
	Location     string // teachingBuildingName + classroomName
	DayOfWeek    int    // 1=周一 … 7=周日
	StartSection int
	EndSection   int
	Segments     []WeekSegment
}

// ParseScheduleCourses 解析课表原始 JSON（FetchSchedule 输出）为课程槽位列表。
// 结构：xkxx: [{ <key>: {courseName, id.coureSequenceNumber, attendClassTeacher,
// timeAndPlaceList: [{classDay, classSessions, continuingSession,
// teachingBuildingName, classroomName, classWeek}] } }]
func ParseScheduleCourses(schedule map[string]interface{}) []CourseSlot {
	xkxx, _ := schedule["xkxx"].([]interface{})
	var courses []CourseSlot
	for _, item := range xkxx {
		courseMap, _ := item.(map[string]interface{})
		for _, v := range courseMap {
			details, _ := v.(map[string]interface{})
			if details == nil {
				continue
			}
			rawName, _ := details["courseName"].(string)
			seq := ""
			if id, ok := details["id"].(map[string]interface{}); ok {
				seq, _ = id["coureSequenceNumber"].(string)
			}
			name := rawName
			if seq != "" {
				name = rawName + " (" + seq + ")"
			}
			teacher, _ := details["attendClassTeacher"].(string)
			tpl, _ := details["timeAndPlaceList"].([]interface{})
			for _, tp := range tpl {
				m, _ := tp.(map[string]interface{})
				if m == nil {
					continue
				}
				dayOfWeek := int(floatField(m, "classDay"))
				startSection := int(floatField(m, "classSessions"))
				continuing := int(floatField(m, "continuingSession"))
				if dayOfWeek < 1 || dayOfWeek > 7 || startSection < 1 || continuing < 1 {
					continue
				}
				building, _ := m["teachingBuildingName"].(string)
				classroom, _ := m["classroomName"].(string)
				classWeek, _ := m["classWeek"].(string)
				segments := ParseClassWeekSegments(classWeek)
				if len(segments) == 0 {
					continue
				}
				courses = append(courses, CourseSlot{
					Name:         name,
					Teacher:      teacher,
					Location:     building + classroom,
					DayOfWeek:    dayOfWeek,
					StartSection: startSection,
					EndSection:   startSection + continuing - 1,
					Segments:     segments,
				})
			}
		}
	}
	return courses
}

func floatField(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// ParseClassWeekSegments 将教务周次位串（"11100…"，第 i 位为 '1' 表示第 i+1 周有课）
// 拆成区间：完整交错序列合并为单/双周区间，连续周合并为 every 区间。
// 移植自 Bugaoshan class_week_parser.dart。
func ParseClassWeekSegments(classWeek string) []WeekSegment {
	var active []int
	for i := 0; i < len(classWeek); i++ {
		if classWeek[i] == '1' {
			active = append(active, i+1)
		}
	}
	if len(active) == 0 {
		return nil
	}
	// 完整交错（每周差 2）→ 单/双周区间
	if len(active) > 1 {
		alternating := true
		for i := 1; i < len(active); i++ {
			if active[i]-active[i-1] != 2 {
				alternating = false
				break
			}
		}
		if alternating {
			t := WeekOdd
			if active[0]%2 == 0 {
				t = WeekEven
			}
			return []WeekSegment{{StartWeek: active[0], EndWeek: active[len(active)-1], Type: t}}
		}
	}
	var segments []WeekSegment
	start, end := active[0], active[0]
	for _, w := range active[1:] {
		if w == end+1 {
			end = w
			continue
		}
		segments = append(segments, WeekSegment{StartWeek: start, EndWeek: end, Type: WeekEvery})
		start, end = w, w
	}
	segments = append(segments, WeekSegment{StartWeek: start, EndWeek: end, Type: WeekEvery})
	return segments
}

// ─── 课表 ICS 展开 ───────────────────────────────────────────────

// dateForCourseDay 计算某教学周某星期几的日期。
// 移植自 Bugaoshan ScheduleConfig.dateForCourseDay：semesterStartDate 允许是周日
// （周日属于第一教学周，故周日映射为周一前一天）。
func dateForCourseDay(semesterStart time.Time, week, dayOfWeek int) time.Time {
	start := time.Date(semesterStart.Year(), semesterStart.Month(), semesterStart.Day(), 0, 0, 0, 0, ics.Shanghai())
	startWeekday := int(start.Weekday()) // Go: 周日=0
	if startWeekday == 0 {
		startWeekday = 7
	}
	mondayOffset := ((1-startWeekday)%7 + 7) % 7 // Go 取模可为负，需归一
	daysFromMonday := dayOfWeek - 1
	if dayOfWeek == 7 {
		daysFromMonday = -1
	}
	return start.AddDate(0, 0, (week-1)*7+mondayOffset+daysFromMonday)
}

// BuildScheduleEvents 将课程槽位按周次/单双周/节次展开为日历事件。
// campus 为空时按课程地点关键词探测校区时段表，均不命中用默认（江安）时段表。
// 返回事件列表与被截断节次的课程数（节次超出时段表时截断）。
func BuildScheduleEvents(courses []CourseSlot, semesterStart time.Time, campus string) (events []ics.Event, truncated int) {
	flagSlots := ics.TimeSlotsForCampus(campus)
	for _, c := range courses {
		slots := flagSlots
		if slots == nil {
			// 按地点关键词探测（如 "江安一教A"）
			loc := c.Location
			for _, kw := range []string{"江安", "望江", "华西"} {
				if strings.Contains(loc, kw) {
					slots = ics.TimeSlotsForCampus(kw)
					break
				}
			}
		}
		if slots == nil {
			slots = ics.DefaultTimeSlots()
		}
		startIdx := c.StartSection - 1
		endIdx := c.EndSection - 1
		if endIdx >= len(slots) {
			endIdx = len(slots) - 1
			truncated++
		}
		if startIdx >= len(slots) {
			continue
		}
		title, geo := ics.ResolveLocation(c.Location)
		desc := ""
		if c.Teacher != "" {
			desc = "教师: " + c.Teacher
		}
		// 稳定 UID 种子：课程身份 + 上课时间地点
		seed := fmt.Sprintf("course|%s|%d|%d-%d|%s", c.Name, c.DayOfWeek, c.StartSection, c.EndSection, c.Location)
		sum := sha1.Sum([]byte(seed))
		courseID := hex.EncodeToString(sum[:])[:12]
		for _, seg := range c.Segments {
			for week := seg.StartWeek; week <= seg.EndWeek; week++ {
				if seg.Type == WeekOdd && week%2 == 0 {
					continue
				}
				if seg.Type == WeekEven && week%2 == 1 {
					continue
				}
				day := dateForCourseDay(semesterStart, week, c.DayOfWeek)
				st := slots[startIdx]
				en := slots[endIdx]
				events = append(events, ics.Event{
					Title:       c.Name,
					Location:    title,
					Description: desc,
					UID:         fmt.Sprintf("%s_%d@bugaoshan", courseID, week),
					Start:       time.Date(day.Year(), day.Month(), day.Day(), st.StartHour, st.StartMin, 0, 0, ics.Shanghai()),
					End:         time.Date(day.Year(), day.Month(), day.Day(), en.EndHour, en.EndMin, 0, 0, ics.Shanghai()),
					Geo:         geo,
				})
			}
		}
	}
	return events, truncated
}

// MatchSemesterStart 从校历匹配学期起始日。planCode 形如 2025-2026-2-1
//（第三段 1=秋季 2=春季）；planCode 为空时取 today 所在的学期。
func MatchSemesterStart(calendar map[string]interface{}, planCode string, today time.Time) (name string, start time.Time, totalWeeks int, err error) {
	semesters, _ := calendar["semesters"].([]interface{})
	wantYear, wantSeason := "", ""
	if planCode != "" {
		parts := strings.Split(planCode, "-")
		if len(parts) >= 3 {
			wantYear = parts[0] + "-" + parts[1]
			switch parts[2] {
			case "1":
				wantSeason = "秋季"
			case "2":
				wantSeason = "春季"
			}
		}
	}
	for _, s := range semesters {
		sem, _ := s.(map[string]interface{})
		if sem == nil {
			continue
		}
		n, _ := sem["name"].(string)
		sd, _ := sem["startDate"].(string)
		w := int(floatField(sem, "totalWeeks"))
		t, perr := time.Parse("2006-01-02", sd)
		if perr != nil {
			continue
		}
		if planCode != "" {
			if wantYear != "" && strings.Contains(n, wantYear) && (wantSeason == "" || strings.Contains(n, wantSeason)) {
				return n, t, w, nil
			}
			continue
		}
		// 未指定学期：取 today 落在 [start, start+weeks*7) 内的学期
		if !today.Before(t) && today.Before(t.AddDate(0, 0, w*7)) {
			return n, t, w, nil
		}
	}
	if planCode != "" {
		return "", time.Time{}, 0, fmt.Errorf("校历中未找到与 %s 匹配的学期，可用 --start-date 手动指定", planCode)
	}
	return "", time.Time{}, 0, fmt.Errorf("校历中未找到当前学期，可用 --start-date 手动指定")
}

// ─── 考表 ICS ────────────────────────────────────────────────────

var (
	examICSDateRe = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
	examICSTimeRe = regexp.MustCompile(`^(\d{2}):(\d{2})-(\d{2}):(\d{2})$`)
	examDoneRe    = regexp.MustCompile(`\s*[（(]\s*已结束\s*[）)]\s*`)
	multiSpace    = regexp.MustCompile(`\s+`)
)

// BuildExamEvents 将考表转为日历事件；日期/时间无法解析的考试静默跳过并计数。
// 移植自 Bugaoshan IcsService.genExamCalendarEvents。
func BuildExamEvents(exams []ExamInfo) (events []ics.Event, skipped int) {
	for _, exam := range exams {
		dm := examICSDateRe.FindStringSubmatch(exam.Date)
		tm := examICSTimeRe.FindStringSubmatch(exam.TimeRange)
		if dm == nil || tm == nil {
			skipped++
			continue
		}
		y, _ := strconv.Atoi(dm[1])
		mo, _ := strconv.Atoi(dm[2])
		d, _ := strconv.Atoi(dm[3])
		sh, _ := strconv.Atoi(tm[1])
		sm, _ := strconv.Atoi(tm[2])
		eh, _ := strconv.Atoi(tm[3])
		em, _ := strconv.Atoi(tm[4])
		name := normalizeExamName(exam.CourseName)
		title := name
		if !strings.HasSuffix(title, "考试") {
			title += "考试"
		}
		lines := []string{exam.Week, "座位号: " + exam.SeatNumber}
		if exam.TicketNumber != "" {
			lines = append(lines, "准考证号: "+exam.TicketNumber)
		}
		if exam.Tip != "" && exam.Tip != "无" {
			lines = append(lines, "提示: "+exam.Tip)
		}
		loc, geo := ics.ResolveLocation(exam.Location)
		sum := sha1.Sum([]byte("exam|" + name))
		events = append(events, ics.Event{
			Title:       title,
			Location:    loc,
			Description: strings.Join(lines, "\n"),
			UID:         "exam-" + hex.EncodeToString(sum[:])[:24] + "@bugaoshan",
			Start:       time.Date(y, time.Month(mo), d, sh, sm, 0, 0, ics.Shanghai()),
			End:         time.Date(y, time.Month(mo), d, eh, em, 0, 0, ics.Shanghai()),
			Geo:         geo,
		})
	}
	return events, skipped
}

func normalizeExamName(name string) string {
	return strings.TrimSpace(multiSpace.ReplaceAllString(examDoneRe.ReplaceAllString(name, ""), " "))
}

// ─── 校历 ICS ────────────────────────────────────────────────────

// BuildCalendarEvents 将展开后的校历（FetchAcademicCalendar 输出）转为全天日历事件。
func BuildCalendarEvents(calendar map[string]interface{}) []ics.Event {
	semesters, _ := calendar["semesters"].([]interface{})
	var events []ics.Event
	for _, s := range semesters {
		sem, _ := s.(map[string]interface{})
		if sem == nil {
			continue
		}
		semName, _ := sem["name"].(string)
		list, _ := sem["events"].([]interface{})
		for _, e := range list {
			ev, _ := e.(map[string]interface{})
			if ev == nil {
				continue
			}
			label, _ := ev["label"].(string)
			dateStr, _ := ev["date"].(string)
			start, err := time.Parse("2006-01-02", dateStr)
			if err != nil || label == "" {
				continue
			}
			end := start
			if ed, _ := ev["endDate"].(string); ed != "" {
				if t, err := time.Parse("2006-01-02", ed); err == nil {
					end = t
				}
			}
			title := label
			if semName != "" {
				title = semName + " · " + label
			}
			events = append(events, ics.Event{
				Title:  title,
				UID:    ics.StableUID("calendar|"+semName+"|"+label+"|"+dateStr, "scu-cli"),
				Start:  start,
				End:    end.AddDate(0, 0, 1), // 全天事件 End 为不含尾日的下一日
				AllDay: true,
			})
		}
	}
	return events
}
