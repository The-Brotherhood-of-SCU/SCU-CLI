package cli

import (
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
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
	Short: "获取学期列表（value 为 planCode）",
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

var zhjwScheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "获取课表（--plan 指定 planCode，如 2025-2026-2-1）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchSchedule(zhjwSchedulePlan)
		})
	},
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

var zhjwExamsCmd = &cobra.Command{
	Use:   "exams",
	Short: "获取考试安排（考表）",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJSON(func() (interface{}, error) {
			s, err := newZhjwService()
			if err != nil {
				return nil, err
			}
			return s.FetchExamPlan()
		})
	},
}

var zhjwCompletionCmd = &cobra.Command{
	Use:   "completion",
	Short: "获取计划完成度（zTree 节点）",
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
	Short: "获取校区与教学楼列表",
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
	Short: "获取教学楼教室类型列表",
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
	Short: "查询教室占用情况",
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
	Short: "获取学院列表",
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
	Short: "获取年级列表",
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
	Short: "搜索培养方案（--college 学院代码 --grade 年级）",
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
	Short: "获取培养方案详情",
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
	Short: "获取班级课表筛选选项（学期/年级/院系）",
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
	Short: "根据院系获取专业列表",
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
	Short: "搜索班级列表",
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
	Short: "获取指定班级课表",
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

func init() {
	zhjwScheduleCmd.Flags().StringVar(&zhjwSchedulePlan, "plan", "", "学期 planCode（如 2025-2026-2-1，省略取当前学期语义由教务决定）")
	zhjwGradesCmd.Flags().BoolVar(&zhjwGradesScheme, "scheme", false, "查询方案成绩而非及格成绩")

	f := zhjwClassroomTypesCmd.Flags()
	f.StringVar(&classroomTypesArgs.campusNum, "campus-num", "", "校区编号")
	f.StringVar(&classroomTypesArgs.buildingNum, "building-num", "", "教学楼编号")
	f.StringVar(&classroomTypesArgs.campusName, "campus-name", "", "校区名称")
	f.StringVar(&classroomTypesArgs.buildingName, "building-name", "", "教学楼名称")

	q := zhjwClassroomQueryCmd.Flags()
	q.StringVar(&classroomQueryArgs.campusNum, "campus-num", "", "校区编号")
	q.StringVar(&classroomQueryArgs.buildingNum, "building-num", "", "教学楼编号")
	q.StringVar(&classroomQueryArgs.classType, "type", "", "教室类型代码")
	q.StringVar(&classroomQueryArgs.className, "name", "", "教室名称")
	q.StringVar(&classroomQueryArgs.seatFrom, "seat-from", "", "座位数下限")
	q.StringVar(&classroomQueryArgs.seatTo, "seat-to", "", "座位数上限")
	q.StringVar(&classroomQueryArgs.date, "date", "", "查询日期 YYYY-MM-DD（省略为当天）")

	zhjwProgramSearchCmd.Flags().StringVar(&programSearchArgs.college, "college", "", "学院代码（xsh）")
	zhjwProgramSearchCmd.Flags().StringVar(&programSearchArgs.grade, "grade", "", "年级（nj）")

	l := zhjwClassListCmd.Flags()
	l.StringVar(&classListArgs.plan, "plan", "", "学期（executiveEducationPlanNum）")
	l.StringVar(&classListArgs.year, "year", "", "年级（yearNum）")
	l.StringVar(&classListArgs.dept, "dept", "", "院系编号（departmentNum）")
	l.StringVar(&classListArgs.subject, "subject", "", "专业编号（subjectNum）")
	l.StringVar(&classListArgs.classNum, "class-num", "", "班级编号（classNum）")

	zhjwClassroomCmd.AddCommand(zhjwClassroomIndexCmd, zhjwClassroomTypesCmd, zhjwClassroomQueryCmd)
	zhjwProgramCmd.AddCommand(zhjwProgramCollegesCmd, zhjwProgramGradesCmd, zhjwProgramSearchCmd, zhjwProgramDetailCmd, zhjwProgramCourseCmd)
	zhjwClassCmd.AddCommand(zhjwClassOptionsCmd, zhjwClassSubjectsCmd, zhjwClassListCmd, zhjwClassScheduleCmd)
	zhjwCmd.AddCommand(zhjwWeekCmd, zhjwSemestersCmd, zhjwScheduleCmd, zhjwGradesCmd, zhjwExamsCmd, zhjwCompletionCmd, zhjwClassroomCmd, zhjwProgramCmd, zhjwClassCmd)
}
