package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/config"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/ics"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/output"
	"github.com/spf13/cobra"
)

// runJSON 统一执行：成功输出 JSON 包层，失败输出错误包层。
func runJSON(fn func() (interface{}, error)) error {
	data, err := fn()
	if err != nil {
		return output.Fail(errorKind(err), err)
	}
	return output.JSON(data)
}

// errorKind 按错误类型给出机器可读分类。
func errorKind(err error) string {
	switch {
	case auth.IsUnauthenticated(err):
		return "unauthenticated"
	case config.IsConfigError(err):
		// 凭据读写/解析失败是本地配置问题，不能归为 service——
		// 那会误导调用方按网络/服务端故障处理。
		return "config"
	default:
		return "service"
	}
}

func newZhjwService() (*api.ZhjwService, error) {
	a, err := newScuAuth()
	if err != nil {
		return nil, err
	}
	return api.NewZhjwService(auth.NewZhjwAuth(a)), nil
}

// ─── 课表 ────────────────────────────────────────────────────────

var zhjwWeekCmd = &cobra.Command{
	Use:   "week",
	Short: "获取当前教学周数",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			week, onVacation, err := s.FetchCurrentWeek()
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"week": week, "on_vacation": onVacation}, nil
		})
	},
}

var zhjwSemestersCmd = &cobra.Command{
	Use:   "semesters",
	Short: "获取学期列表（value 即 schedule/class list --plan 所需的 planCode）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchSemesters()
		})
	},
}

var zhjwSchedulePlan string
var zhjwScheduleICS struct{ file, startDate, campus string }

var zhjwScheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "获取课表（--plan 指定 planCode，如 2025-2026-2-1；--ics 导出日历文件）",
	RunE: func(cmd *cobra.Command, args []string) error {
		if zhjwScheduleICS.startDate != "" {
			if _, err := time.Parse("2006-01-02", zhjwScheduleICS.startDate); err != nil {
				return output.Fail("input", fmt.Errorf("--start-date 格式应为 YYYY-MM-DD: %v", err))
			}
		}
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			raw, err := s.FetchSchedule(zhjwSchedulePlan)
			if err != nil {
				return nil, err
			}
			if zhjwScheduleICS.file == "" {
				return raw, nil
			}
			return exportScheduleICS(raw, zhjwSchedulePlan, zhjwScheduleICS.startDate, zhjwScheduleICS.campus, zhjwScheduleICS.file)
		})
	},
}

// exportScheduleICS 课表 → ICS 文件：解析课程、匹配学期起始日（校历）、按周次展开事件。
func exportScheduleICS(raw map[string]interface{}, planCode, startDate, campus, file string) (interface{}, error) {
	var semStart time.Time
	var semName string
	var totalWeeks int
	if startDate != "" {
		t, _ := time.Parse("2006-01-02", startDate) // RunE 已校验格式
		semStart = t
	} else {
		calendar, err := api.FetchAcademicCalendar()
		if err != nil {
			return nil, err
		}
		n, t, w, err := api.MatchSemesterStart(calendar, planCode, time.Now())
		if err != nil {
			return nil, err
		}
		semName, semStart, totalWeeks = n, t, w
	}
	courses := api.ParseScheduleCourses(raw)
	events, truncated := api.BuildScheduleEvents(courses, semStart, campus)
	if len(events) == 0 {
		return nil, &auth.ServiceError{Msg: "课表中没有可展开的课程"}
	}
	if err := os.WriteFile(file, []byte(ics.Build("Course Schedule", events)), 0o644); err != nil {
		return nil, &auth.ServiceError{Msg: "写入 ICS 文件失败: " + err.Error()}
	}
	return map[string]interface{}{
		"file":           file,
		"events":         len(events),
		"courses":        len(courses),
		"semester":       semName,
		"semester_start": semStart.Format("2006-01-02"),
		"total_weeks":    totalWeeks,
		"truncated":      truncated,
	}, nil
}

// ─── 成绩 / 考表 / 计划完成度 ───────────────────────────────────

var zhjwGradesScheme bool

var zhjwGradesCmd = &cobra.Command{
	Use:   "grades",
	Short: "获取成绩（默认及格成绩，--scheme 为方案成绩）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			if zhjwGradesScheme {
				return s.FetchSchemeScores()
			}
			return s.FetchPassingScores()
		})
	},
}

var zhjwExamsICSFile string

var zhjwExamsCmd = &cobra.Command{
	Use:   "exams",
	Short: "获取考试安排（考表；--ics 导出日历文件）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			exams, err := s.FetchExamPlan()
			if err != nil {
				return nil, err
			}
			if zhjwExamsICSFile == "" {
				return exams, nil
			}
			events, skipped := api.BuildExamEvents(exams)
			if len(events) == 0 {
				return nil, &auth.ServiceError{Msg: "考表中没有可导出的考试"}
			}
			if err := os.WriteFile(zhjwExamsICSFile, []byte(ics.Build("Exam Schedule", events)), 0o644); err != nil {
				return nil, &auth.ServiceError{Msg: "写入 ICS 文件失败: " + err.Error()}
			}
			return map[string]interface{}{
				"file": zhjwExamsICSFile, "events": len(events), "skipped_unparseable": skipped,
			}, nil
		})
	},
}

var zhjwCompletionCmd = &cobra.Command{
	Use:   "completion",
	Short: "获取计划完成度（多份培养方案，主修/辅修/微专业各一份）",
	Long: `获取计划完成度（zTree 节点）。输出 plans 数组，每份培养方案一项：
单方案用户只有一项（id 为空）；多方案用户（主修+辅修等）每份方案一项，
id 为方案 ID、name 为方案名。nodes 为 zTree 节点原始 JSON
（name 含 HTML，sfwc=="是" 表示已完成）。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchPlanCompletion()
		})
	},
}

// ─── 教室 ────────────────────────────────────────────────────────

var zhjwClassroomCmd = &cobra.Command{
	Use:   "classroom",
	Short: "教室查询（index / types / query）",
}

var zhjwClassroomIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "获取校区与教学楼列表（types/query 的编号来源）",
	Long: `获取校区与教学楼列表，是 classroom types/query 所有编号参数的来源：
  campuses[].campusNumber / campusName              → --campus-num / --campus-name
  buildings[].teachingBuildingNumber / ...Name      → --building-num / --building-name`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchClassroomIndex()
		})
	},
}

var classroomTypesArgs struct{ campusNum, buildingNum, campusName, buildingName string }

var zhjwClassroomTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "获取教学楼教室类型列表（编号取自 classroom index）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			a := classroomTypesArgs
			return s.FetchClassroomTypes(a.campusNum, a.buildingNum, a.campusName, a.buildingName)
		})
	},
}

var classroomQueryArgs struct{ campusNum, buildingNum, classType, className, seatFrom, seatTo, date string }

var zhjwClassroomQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "查询教室占用情况（编号取自 classroom index，类型取自 classroom types）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			a := classroomQueryArgs
			return s.FetchClassroomAvailability(a.campusNum, a.buildingNum, a.classType, a.className, a.seatFrom, a.seatTo, a.date)
		})
	},
}

// ─── 培养方案 ────────────────────────────────────────────────────

var zhjwProgramCmd = &cobra.Command{
	Use:   "program",
	Short: "培养方案（colleges / grades / search / detail / course）",
}

var zhjwProgramCollegesCmd = &cobra.Command{
	Use:   "colleges",
	Short: "获取学院列表（value 供 program search --college 使用）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchColleges()
		})
	},
}

var zhjwProgramGradesCmd = &cobra.Command{
	Use:   "grades",
	Short: "获取年级列表（value 供 program search --grade 使用）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchGrades()
		})
	},
}

var programSearchArgs struct{ college, grade string }

var zhjwProgramSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "搜索培养方案（代码取自 program colleges/grades；输出含 fajhh）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.SearchPrograms(programSearchArgs.college, programSearchArgs.grade)
		})
	},
}

var zhjwProgramDetailCmd = &cobra.Command{
	Use:   "detail <fajhh>",
	Short: "获取培养方案详情（fajhh 取自 program search 记录；treeList 含 urlPath）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchProgramDetail(args[0])
		})
	},
}

var zhjwProgramCourseCmd = &cobra.Command{
	Use:   "course <urlPath>",
	Short: "获取课程详情（urlPath 来自方案详情 treeList 节点）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchCourseDetail(args[0])
		})
	},
}

// ─── 班级课表 ────────────────────────────────────────────────────

var zhjwClassCmd = &cobra.Command{
	Use:   "class",
	Short: "班级课表查询（options / subjects / list / schedule）",
}

var zhjwClassOptionsCmd = &cobra.Command{
	Use:   "options",
	Short: "获取班级课表筛选项（semesters/grades/departments，供 subjects/list 使用）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchClassScheduleInquiryIndex()
		})
	},
}

var zhjwClassSubjectsCmd = &cobra.Command{
	Use:   "subjects <departmentNum>",
	Short: "根据院系获取专业列表（departmentNum 取自 options 的 departments value；输出含 subjectCode）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchSubjectsByDepartment(args[0])
		})
	},
}

var classListArgs struct{ plan, year, dept, subject, classNum string }

var zhjwClassListCmd = &cobra.Command{
	Use:   "list",
	Short: "搜索班级列表（参数取自 options/subjects；输出 id.executiveEducationPlanNumber 与 id.classNum）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			a := classListArgs
			return s.FetchClassList(1, 100, a.plan, a.year, a.dept, a.subject, a.classNum)
		})
	},
}

var zhjwClassScheduleCmd = &cobra.Command{
	Use:   "schedule <planCode> <classCode>",
	Short: "获取指定班级课表（planCode=class list 的 id.executiveEducationPlanNumber，classCode=id.classNum）",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchClassSchedule(args[0], args[1])
		})
	},
}

// ─── 课程课表 ────────────────────────────────────────────────────

var zhjwCourseCmd = &cobra.Command{
	Use:   "course",
	Short: "课程课表查询（index / search / schedule）",
	Long: `课程课表：按学期/院系/课程名等条件搜索课程（教学班），
再查看某门课程的排课安排（返回结构与班级课表一致）。发现链：

  semester   ← zhjw course index 输出的 semesters[].value（如 2026-2027-1-1）
  department ← zhjw course index 输出的 departments[].value
  category   ← zhjw course index 输出的 categories[].value
  planCode/courseCode/courseSeq ← zhjw course search 输出记录的
                ZXJXJHH / KCH / KXH 字段`,
}

var zhjwCourseIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "获取课程课表筛选选项（semesters/departments/categories）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchCourseCurriculumIndex()
		})
	},
}

var courseSearchArgs struct {
	semester, department, name, code, seq, category string
	page, pageSize                                  int
}

var zhjwCourseSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "搜索课程列表（教学班；输出记录含 ZXJXJHH/KCH/KXH 三元组）",
	Long: `搜索课程（教学班）。筛选参数取自 zhjw course index：
--semester（学年学期 value）、--department（开课院系 value）、
--name（课程名关键词）、--code（课程号）、--seq（课序号）、--category（课程类别 value）。
输出 records（KCM 课程名 / JSM 教师 / KKXSM 开课院系 / XF 学分等）与 total。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			a := courseSearchArgs
			return s.FetchCourseList(a.page, a.pageSize, a.semester, a.department, a.name, a.code, a.seq, a.category)
		})
	},
}

var zhjwCourseScheduleCmd = &cobra.Command{
	Use:   "schedule <planCode> <courseCode> <courseSeq>",
	Short: "获取指定课程（教学班）的课表（三元组取自 course search 记录）",
	Long: `获取某门课程（教学班）的排课安排，返回结构与 class schedule 一致：
各项含 id.skxq（星期 1-7）、id.skjc（开始节次）、cxjc（持续节数）、
kcm（课程名）、jsm（教师）、zcsm（周次说明）、xqm/jxlm/jasm（校区/教学楼/教室）。
planCode/courseCode/courseSeq 对应 course search 输出记录的
ZXJXJHH / KCH / KXH 字段。`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchCourseSchedule(args[0], args[1], args[2])
		})
	},
}

var zhjwCalendarICSFile string

var zhjwCalendarCmd = &cobra.Command{
	Use:   "calendar",
	Short: "获取校历（免认证，网络优先、失败回退本地缓存；--ics 导出日历文件）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			calendar, err := api.FetchAcademicCalendar()
			if err != nil {
				return nil, err
			}
			if zhjwCalendarICSFile == "" {
				return calendar, nil
			}
			events := api.BuildCalendarEvents(calendar)
			if len(events) == 0 {
				return nil, &auth.ServiceError{Msg: "校历中没有可导出的事件"}
			}
			if err := os.WriteFile(zhjwCalendarICSFile, []byte(ics.Build("Academic Calendar", events)), 0o644); err != nil {
				return nil, &auth.ServiceError{Msg: "写入 ICS 文件失败: " + err.Error()}
			}
			return map[string]interface{}{"file": zhjwCalendarICSFile, "events": len(events)}, nil
		})
	},
}

func init() {
	zhjwScheduleCmd.Flags().StringVar(&zhjwSchedulePlan, "plan", "", "学期 planCode（取自 zhjw semesters 的 value；省略取当前学期）")
	s := zhjwScheduleCmd.Flags()
	s.StringVar(&zhjwScheduleICS.file, "ics", "", "导出 ICS 日历到指定文件（如 --ics schedule.ics）")
	s.StringVar(&zhjwScheduleICS.startDate, "start-date", "", "学期起始日 YYYY-MM-DD（省略则从校历自动匹配）")
	s.StringVar(&zhjwScheduleICS.campus, "campus", "", "节次时段校区：江安 / 望江 / 华西（省略则按课程地点探测，兜底江安）")
	zhjwGradesCmd.Flags().BoolVar(&zhjwGradesScheme, "scheme", false, "查询方案成绩而非及格成绩")

	zhjwExamsCmd.Flags().StringVar(&zhjwExamsICSFile, "ics", "", "导出 ICS 日历到指定文件（如 --ics exams.ics）")
	zhjwCalendarCmd.Flags().StringVar(&zhjwCalendarICSFile, "ics", "", "导出 ICS 日历到指定文件（如 --ics calendar.ics）")

	f := zhjwClassroomTypesCmd.Flags()
	f.StringVar(&classroomTypesArgs.campusNum, "campus-num", "", "校区编号（classroom index 的 campuses[].campusNumber）")
	f.StringVar(&classroomTypesArgs.buildingNum, "building-num", "", "教学楼编号（classroom index 的 buildings[].teachingBuildingNumber）")
	f.StringVar(&classroomTypesArgs.campusName, "campus-name", "", "校区名称（campuses[].campusName）")
	f.StringVar(&classroomTypesArgs.buildingName, "building-name", "", "教学楼名称（buildings[].teachingBuildingName）")

	q := zhjwClassroomQueryCmd.Flags()
	q.StringVar(&classroomQueryArgs.campusNum, "campus-num", "", "校区编号（同 classroom types）")
	q.StringVar(&classroomQueryArgs.buildingNum, "building-num", "", "教学楼编号（同 classroom types）")
	q.StringVar(&classroomQueryArgs.classType, "type", "", "教室类型代码（取自 classroom types 输出；可空）")
	q.StringVar(&classroomQueryArgs.className, "name", "", "教室名称关键词（可空）")
	q.StringVar(&classroomQueryArgs.seatFrom, "seat-from", "", "座位数下限（可空）")
	q.StringVar(&classroomQueryArgs.seatTo, "seat-to", "", "座位数上限（可空）")
	q.StringVar(&classroomQueryArgs.date, "date", "", "查询日期 YYYY-MM-DD（省略为当天）")

	zhjwProgramSearchCmd.Flags().StringVar(&programSearchArgs.college, "college", "", "学院代码（program colleges 的 value；可空）")
	zhjwProgramSearchCmd.Flags().StringVar(&programSearchArgs.grade, "grade", "", "年级（program grades 的 value；可空）")

	l := zhjwClassListCmd.Flags()
	l.StringVar(&classListArgs.plan, "plan", "", "学期（class options 的 semesters[].value）")
	l.StringVar(&classListArgs.year, "year", "", "年级（class options 的 grades[].value）")
	l.StringVar(&classListArgs.dept, "dept", "", "院系编号（class options 的 departments[].value）")
	l.StringVar(&classListArgs.subject, "subject", "", "专业编号（class subjects 输出的 subjectCode；可空）")
	l.StringVar(&classListArgs.classNum, "class-num", "", "班级编号（可空，精确过滤）")

	c := zhjwCourseSearchCmd.Flags()
	c.StringVar(&courseSearchArgs.semester, "semester", "", "学年学期（course index 的 semesters[].value；可空）")
	c.StringVar(&courseSearchArgs.department, "department", "", "开课院系（course index 的 departments[].value；可空）")
	c.StringVar(&courseSearchArgs.name, "name", "", "课程名关键词（可空）")
	c.StringVar(&courseSearchArgs.code, "code", "", "课程号（可空）")
	c.StringVar(&courseSearchArgs.seq, "seq", "", "课序号（可空）")
	c.StringVar(&courseSearchArgs.category, "category", "", "课程类别（course index 的 categories[].value；可空）")
	c.IntVar(&courseSearchArgs.page, "page", 1, "页码")
	c.IntVar(&courseSearchArgs.pageSize, "page-size", 30, "每页条数")

	zhjwClassroomCmd.AddCommand(zhjwClassroomIndexCmd, zhjwClassroomTypesCmd, zhjwClassroomQueryCmd)
	zhjwProgramCmd.AddCommand(zhjwProgramCollegesCmd, zhjwProgramGradesCmd, zhjwProgramSearchCmd, zhjwProgramDetailCmd, zhjwProgramCourseCmd)
	zhjwClassCmd.AddCommand(zhjwClassOptionsCmd, zhjwClassSubjectsCmd, zhjwClassListCmd, zhjwClassScheduleCmd)
	zhjwCourseCmd.AddCommand(zhjwCourseIndexCmd, zhjwCourseSearchCmd, zhjwCourseScheduleCmd)
	zhjwCmd.AddCommand(zhjwWeekCmd, zhjwSemestersCmd, zhjwScheduleCmd, zhjwGradesCmd, zhjwExamsCmd, zhjwCompletionCmd, zhjwCalendarCmd, zhjwClassroomCmd, zhjwProgramCmd, zhjwClassCmd, zhjwCourseCmd)
}
