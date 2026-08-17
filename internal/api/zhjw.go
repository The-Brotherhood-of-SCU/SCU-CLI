// Package api 实现各子系统的业务 API（L1，无认证状态）。
package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

// Option 是 HTML select 选项。
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ExamInfo 是一场考试的安排（从考表 HTML 解析）。
type ExamInfo struct {
	CourseName   string `json:"course_name"`
	Week         string `json:"week"`
	Date         string `json:"date"`
	Weekday      string `json:"weekday"`
	TimeRange    string `json:"time_range"`
	Location     string `json:"location"`
	SeatNumber   string `json:"seat_number"`
	TicketNumber string `json:"ticket_number"`
	Tip          string `json:"tip"`
}

const zbase = auth.ZhjwBase

var zhtmlHeaders = map[string]string{
	"Accept":     "text/html,*/*",
	"Referer":    zbase + "/",
	"User-Agent": auth.DefaultUserAgent,
}

var zajaxHeaders = map[string]string{
	"Accept":           "application/json, text/javascript, */*; q=0.01",
	"Content-Type":     "application/x-www-form-urlencoded; charset=UTF-8",
	"Referer":          zbase + "/student/courseSelect/calendarSemesterCurriculum/index",
	"User-Agent":       auth.DefaultUserAgent,
	"X-Requested-With": "XMLHttpRequest",
}

// ZhjwService 教务系统业务 API：课表、成绩、考表、教室、培养方案、班级课表。
type ZhjwService struct {
	auth *auth.ZhjwAuth
}

func NewZhjwService(a *auth.ZhjwAuth) *ZhjwService { return &ZhjwService{auth: a} }

// request 是统一的重试边界：认证失效时 invalidate 教务 session 并重放一次。
func (s *ZhjwService) request(fn func(*auth.CookieClient) (interface{}, error)) (interface{}, error) {
	return auth.RetryOnUnauthenticated(s.auth.GetClient, fn, s.auth.Invalidate)
}

// checkSessionExpiry 识别教务会话过期：302、空 body 或 HTML 登录页。
func checkZhjwSessionExpiry(body string, statusCode int) error {
	trimmed := strings.TrimSpace(body)
	if statusCode == 302 || trimmed == "" {
		return &auth.UnauthenticatedError{Msg: "教务 session 已过期"}
	}
	if strings.HasPrefix(trimmed, "<") && strings.Contains(trimmed, "login") {
		return &auth.UnauthenticatedError{Msg: "教务 session 已过期"}
	}
	return nil
}

// parseJSON 解析 JSON 响应，非 JSON 视为服务错误。
func parseJSON(body string, api string, out interface{}) error {
	if err := json.Unmarshal([]byte(body), out); err != nil {
		return &auth.ServiceError{Msg: fmt.Sprintf("[%s] 响应解析失败", api)}
	}
	return nil
}

// zhjwGet 发起 GET 并做统一会话过期检查，返回 TrimSpace 后的 body。
func zhjwGet(c *auth.CookieClient, url string, headers map[string]string) (string, error) {
	resp, err := c.Get(url, headers)
	if err != nil {
		return "", err
	}
	body := strings.TrimSpace(string(resp.Body))
	if err := checkZhjwSessionExpiry(body, resp.StatusCode); err != nil {
		return "", err
	}
	return body, nil
}

// zhjwPost 发起表单 POST 并做统一会话过期检查，返回 TrimSpace 后的 body。
func zhjwPost(c *auth.CookieClient, url string, headers map[string]string, form url.Values) (string, error) {
	resp, err := c.PostForm(url, headers, form)
	if err != nil {
		return "", err
	}
	body := strings.TrimSpace(string(resp.Body))
	if err := checkZhjwSessionExpiry(body, resp.StatusCode); err != nil {
		return "", err
	}
	return body, nil
}

// zhjwGetJSON 是 zhjwGet + parseJSON 的组合。
func zhjwGetJSON(c *auth.CookieClient, url, api string, headers map[string]string, out interface{}) error {
	body, err := zhjwGet(c, url, headers)
	if err != nil {
		return err
	}
	return parseJSON(body, api, out)
}

// zhjwPostJSON 是 zhjwPost + parseJSON 的组合。
func zhjwPostJSON(c *auth.CookieClient, url, api string, headers map[string]string, form url.Values, out interface{}) error {
	body, err := zhjwPost(c, url, headers, form)
	if err != nil {
		return err
	}
	return parseJSON(body, api, out)
}

// ─── 课表 ────────────────────────────────────────────────────────

var weekRegexp = regexp.MustCompile(`第(\d+)周`)

// FetchCurrentWeek 获取当前教学周数；假期返回 (0, true, nil)。
func (s *ZhjwService) FetchCurrentWeek() (week int, onVacation bool, err error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		body, err := zhjwGet(c, zbase+"/", zhtmlHeaders)
		if err != nil {
			return nil, err
		}
		if m := weekRegexp.FindStringSubmatch(body); m != nil {
			var w int
			fmt.Sscanf(m[1], "%d", &w)
			return w, nil
		}
		if strings.Contains(body, "当前处于假期时间") {
			return -1, nil
		}
		return nil, &auth.ServiceError{Msg: "无法获取当前周数，请检查教务系统状态"}
	})
	if err != nil {
		return 0, false, err
	}
	if v.(int) == -1 {
		return 0, true, nil
	}
	return v.(int), false, nil
}

var optionRegexp = regexp.MustCompile(`(?s)<option[^>]+value="([^"]+)"[^>]*>(.*?)</option>`)
var htmlTagRegexp = regexp.MustCompile(`<[^>]+>`)

// FetchSemesters 获取历年学期列表（value 为 planCode，如 2025-2026-2-1）。
func (s *ZhjwService) FetchSemesters() ([]Option, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		body, err := zhjwGet(c, zbase+"/student/courseSelect/calendarSemesterCurriculum/index", zhtmlHeaders)
		if err != nil {
			return nil, err
		}
		var semesters []Option
		for _, m := range optionRegexp.FindAllStringSubmatch(body, -1) {
			semesters = append(semesters, Option{
				Value: strings.TrimSpace(m[1]),
				Label: strings.TrimSpace(htmlTagRegexp.ReplaceAllString(m[2], "")),
			})
		}
		if len(semesters) == 0 {
			return nil, &auth.ServiceError{Msg: "无法获取学期列表，请检查登录状态"}
		}
		return semesters, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]Option), nil
}

// FetchSchedule 获取指定学期课表原始 JSON（planCode 如 2025-2026-2-1）。
func (s *ZhjwService) FetchSchedule(planCode string) (map[string]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		var out map[string]interface{}
		if err := zhjwPostJSON(c, zbase+"/student/courseSelect/thisSemesterCurriculum/ajaxStudentSchedule/callback",
			"jwxt/schedule", zajaxHeaders, url.Values{"planCode": {planCode}}, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// ─── 成绩 ────────────────────────────────────────────────────────

// fetchScoresTwoStep 成绩查询两步：index HTML 提取 callback URL，再 GET JSON。
func (s *ZhjwService) fetchScoresTwoStep(kind string) (map[string]interface{}, error) {
	indexPath := zbase + "/student/integratedQuery/scoreQuery/" + kind + "/index"
	callbackRe := regexp.MustCompile(`var\s+url\s*=\s*"(/student/integratedQuery/scoreQuery/[^/]+/` + kind + `/callback)"`)
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		indexBody, err := zhjwGet(c, indexPath, map[string]string{
			"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Referer":    zbase + "/",
			"User-Agent": auth.DefaultUserAgent,
		})
		if err != nil {
			return nil, err
		}
		m := callbackRe.FindStringSubmatch(indexBody)
		if m == nil {
			if strings.Contains(indexBody, "login") || strings.Contains(indexBody, "Login") {
				return nil, &auth.UnauthenticatedError{Msg: "教务 session 已过期"}
			}
			return nil, &auth.ServiceError{Msg: "无法从页面提取 " + kind + " callback URL"}
		}
		var out map[string]interface{}
		if err := zhjwGetJSON(c, zbase+m[1], kind+"/callback", map[string]string{
			"Accept":     "application/json, text/plain, */*",
			"Referer":    indexPath,
			"User-Agent": auth.DefaultUserAgent,
		}, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// FetchPassingScores 获取及格成绩。
func (s *ZhjwService) FetchPassingScores() (map[string]interface{}, error) {
	return s.fetchScoresTwoStep("allPassingScores")
}

// FetchSchemeScores 获取方案成绩。
func (s *ZhjwService) FetchSchemeScores() (map[string]interface{}, error) {
	return s.fetchScoresTwoStep("schemeScores")
}

// ─── 考表 ────────────────────────────────────────────────────────

var (
	examBlockRe   = regexp.MustCompile(`(?s)<div class="widget-box widget-color-\w+(?: collapsed)?">(.*?)</div>\s*</div>\s*</div>\s*</div>`)
	examTitleRe   = regexp.MustCompile(`(?s)<h5 class="widget-title smaller">\s*(.*?)\s*</h5>`)
	examWeekRe    = regexp.MustCompile(`(\d+)周`)
	examDateRe    = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})\s*&nbsp;`)
	examWeekdayRe = regexp.MustCompile(`(星期[一二三四五六日])`)
	examTimeRe    = regexp.MustCompile(`&nbsp;(\d{2}:\d{2}-\d{2}:\d{2})`)
	examLocRe     = regexp.MustCompile(`(?s)地点:&nbsp;(.+?)</br>`)
	examSeatRe    = regexp.MustCompile(`座位号:&nbsp;(\d+)`)
	examTicketRe  = regexp.MustCompile(`(?s)准考证号:&nbsp;(.*?)</br>`)
	examTipRe     = regexp.MustCompile(`(?s)考试提示信息：&nbsp;(.*?)</span>`)
	examEndedRe   = regexp.MustCompile(`\s*（已结束）`)
)

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// FetchExamPlan 获取考试安排列表（解析考表 HTML 卡片）。
func (s *ZhjwService) FetchExamPlan() ([]ExamInfo, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		body, err := zhjwGet(c, zbase+"/student/examinationManagement/examPlan/index", map[string]string{
			"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Referer":    zbase + "/",
			"User-Agent": auth.DefaultUserAgent,
		})
		if err != nil {
			return nil, err
		}
		var exams []ExamInfo
		for _, block := range examBlockRe.FindAllStringSubmatch(body, -1) {
			b := block[1]
			name := examEndedRe.ReplaceAllString(firstSubmatch(examTitleRe, b), "")
			if name == "" {
				name = "未知"
			}
			week := firstSubmatch(examWeekRe, b)
			weekStr := "未知"
			if week != "" {
				weekStr = "第 " + week + " 周"
			}
			exams = append(exams, ExamInfo{
				CourseName:   name,
				Week:         weekStr,
				Date:         orDefault(firstSubmatch(examDateRe, b), "未知"),
				Weekday:      orDefault(firstSubmatch(examWeekdayRe, b), "未知"),
				TimeRange:    orDefault(firstSubmatch(examTimeRe, b), "未知"),
				Location:     orDefault(strings.ReplaceAll(firstSubmatch(examLocRe, b), "&nbsp;", " "), "未知"),
				SeatNumber:   orDefault(firstSubmatch(examSeatRe, b), "未知"),
				TicketNumber: firstSubmatch(examTicketRe, b),
				Tip:          orDefault(firstSubmatch(examTipRe, b), "无"),
			})
		}
		return exams, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]ExamInfo), nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ─── 教室 ────────────────────────────────────────────────────────

func extractHiddenInputJSON(body, id string) (string, error) {
	re := regexp.MustCompile(`<input[^>]+id="` + id + `"[^>]+value='([^']+)'`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		return "", &auth.ServiceError{Msg: "无法解析 " + id}
	}
	return m[1], nil
}

// FetchClassroomIndex 获取校区与教学楼列表（原始 JSON 数组）。
func (s *ZhjwService) FetchClassroomIndex() (map[string]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		body, err := zhjwGet(c, zbase+"/student/teachingResources/classroomUseStatus/index", zhtmlHeaders)
		if err != nil {
			return nil, err
		}
		xq, err := extractHiddenInputJSON(body, "xqList")
		if err != nil {
			return nil, err
		}
		jxl, err := extractHiddenInputJSON(body, "jxlList")
		if err != nil {
			return nil, err
		}
		var campuses, buildings []interface{}
		if err := json.Unmarshal([]byte(xq), &campuses); err != nil {
			return nil, &auth.ServiceError{Msg: "校区列表解析失败"}
		}
		if err := json.Unmarshal([]byte(jxl), &buildings); err != nil {
			return nil, &auth.ServiceError{Msg: "教学楼列表解析失败"}
		}
		return map[string]interface{}{"campuses": campuses, "buildings": buildings}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// FetchClassroomTypes 获取教学楼教室类型列表（原始 JSON 数组）。
func (s *ZhjwService) FetchClassroomTypes(campusNumber, buildingNumber, campusName, buildingName string) ([]interface{}, error) {
	path := fmt.Sprintf("%s/student/teachingResources/classroomUseStatus/%s/%s/%s/%s",
		zbase, campusNumber, buildingNumber, url.QueryEscape(campusName), url.QueryEscape(buildingName))
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		body, err := zhjwGet(c, path, zhtmlHeaders)
		if err != nil {
			return nil, err
		}
		raw, err := extractHiddenInputJSON(body, "classroomTypes")
		if err != nil {
			return []interface{}{}, nil // 无该 input 视为空列表（与 App 一致）
		}
		var types []interface{}
		if err := json.Unmarshal([]byte(raw), &types); err != nil {
			return nil, &auth.ServiceError{Msg: "教室类型解析失败"}
		}
		return types, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}

// FetchClassroomAvailability 查询教室使用情况（原始 JSON）。
func (s *ZhjwService) FetchClassroomAvailability(campusNumber, buildingNumber, classroomType, classroomName, seatFrom, seatTo, searchDate string) (map[string]interface{}, error) {
	form := url.Values{
		"xqh":        {campusNumber},
		"jxlh":       {buildingNumber},
		"jslx":       {classroomType},
		"jasm":       {classroomName},
		"zwFrom":     {seatFrom},
		"zwTo":       {seatTo},
		"searchDate": {searchDate},
	}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		var out map[string]interface{}
		if err := zhjwPostJSON(c, zbase+"/student/teachingResources/classroomUseStatus/jasInfo",
			"classroomUseStatus/jasInfo", zajaxHeaders, form, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// ─── 培养方案 ────────────────────────────────────────────────────

func parseSelectOptions(body, selectName string) []Option {
	re := regexp.MustCompile(`(?s)<select[^>]*name="` + selectName + `"[^>]*>(.*?)</select>`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	optRe := regexp.MustCompile(`(?s)<option[^>]*value="([^"]*)"[^>]*>(.*?)</option>`)
	var out []Option
	for _, om := range optRe.FindAllStringSubmatch(m[1], -1) {
		if om[1] == "" {
			continue
		}
		out = append(out, Option{
			Value: om[1],
			Label: strings.TrimSpace(htmlTagRegexp.ReplaceAllString(om[2], "")),
		})
	}
	return out
}

func (s *ZhjwService) fetchTrainProgramIndex() (string, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		return zhjwGet(c, zbase+"/student/comprehensiveQuery/search/trainProgram/index", zhtmlHeaders)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// FetchColleges 获取学院列表（select name=xsh）。
func (s *ZhjwService) FetchColleges() ([]Option, error) {
	body, err := s.fetchTrainProgramIndex()
	if err != nil {
		return nil, err
	}
	return parseSelectOptions(body, "xsh"), nil
}

// FetchGrades 获取年级列表（select name=nj）。
func (s *ZhjwService) FetchGrades() ([]Option, error) {
	body, err := s.fetchTrainProgramIndex()
	if err != nil {
		return nil, err
	}
	return parseSelectOptions(body, "nj"), nil
}

var trainProgramFormHeaders = map[string]string{
	"Accept":       "application/json, */*",
	"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
	"Referer":      zbase + "/student/comprehensiveQuery/search/trainProgram/index",
	"User-Agent":   auth.DefaultUserAgent,
}

// SearchPrograms 搜索培养方案（college/grade 可空）。
func (s *ZhjwService) SearchPrograms(college, grade string) ([]interface{}, error) {
	form := url.Values{
		"famc": {""}, "jhmc": {""}, "nj": {grade}, "xw": {""}, "xzlx": {""},
		"xdlx": {"00001"}, "xsh": {college}, "pageNum": {"1"}, "pageSize": {"100"},
	}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		var out struct {
			Data struct {
				Records []interface{} `json:"records"`
			} `json:"data"`
		}
		if err := zhjwPostJSON(c, zbase+"/student/comprehensiveQuery/search/trainProgram/load",
			"trainProgram/load", trainProgramFormHeaders, form, &out); err != nil {
			return nil, err
		}
		if out.Data.Records == nil {
			out.Data.Records = []interface{}{}
		}
		return out.Data.Records, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}

// FetchProgramDetail 获取培养方案详情（fajhh 为方案计划号）。
func (s *ZhjwService) FetchProgramDetail(fajhh string) (map[string]interface{}, error) {
	form := url.Values{"fajhh": {fajhh}, "lx": {"1"}}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		var out map[string]interface{}
		if err := zhjwPostJSON(c, zbase+"/student/comprehensiveQuery/search/trainProgram/detail",
			"trainProgram/detail", trainProgramFormHeaders, form, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// FetchCourseDetail 获取课程详情（urlPath 来自方案详情 treeList 节点）。
func (s *ZhjwService) FetchCourseDetail(urlPath string) (map[string]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		var out map[string]interface{}
		if err := zhjwGetJSON(c, zbase+urlPath, "courseDetail", map[string]string{
			"Accept":     "application/json, */*",
			"Referer":    zbase + "/student/comprehensiveQuery/search/trainProgram/index",
			"User-Agent": auth.DefaultUserAgent,
		}, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// ─── 计划完成度 ──────────────────────────────────────────────────

var zNodesRegexp = regexp.MustCompile(`(?s)var\s+zNodes\s*=\s*(\[.*?\]);`)

// FetchPlanCompletion 获取计划完成度（zTree 节点原始 JSON）。
func (s *ZhjwService) FetchPlanCompletion() ([]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		resp, err := c.Get(zbase+"/student/integratedQuery/planCompletion/index", zhtmlHeaders)
		if err != nil {
			return nil, err
		}
		body := string(resp.Body)
		if strings.Contains(body, "请勿频繁刷新") {
			return nil, &auth.RateLimitedError{Msg: "教务系统限流：请勿频繁刷新"}
		}
		trimmed := strings.TrimSpace(body)
		// 302 / 空 body 是会话过期（CookieClient 不自动跟随重定向），
		// 不能落入下方 m == nil 分支静默返回空列表。
		if resp.StatusCode == 302 || trimmed == "" {
			return nil, &auth.UnauthenticatedError{Msg: "教务 session 已过期"}
		}
		// 正常的完成度页本身是 HTML（含 zNodes），只有"是 HTML 但不含
		// zNodes"才判定为登录页，不能用 checkZhjwSessionExpiry。
		if strings.HasPrefix(trimmed, "<") && !strings.Contains(body, "zNodes") {
			return nil, &auth.UnauthenticatedError{Msg: "教务 session 已过期"}
		}
		m := zNodesRegexp.FindStringSubmatch(body)
		if m == nil {
			return []interface{}{}, nil
		}
		var nodes []interface{}
		if err := json.Unmarshal([]byte(m[1]), &nodes); err != nil {
			return []interface{}{}, nil
		}
		return nodes, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}

// ─── 班级课表 ────────────────────────────────────────────────────

// FetchClassScheduleInquiryIndex 获取班级课表筛选选项（学期/年级/院系）。
func (s *ZhjwService) FetchClassScheduleInquiryIndex() (map[string]interface{}, error) {
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		body, err := zhjwGet(c, zbase+"/student/teachingResources/classCurriculum/index", zhtmlHeaders)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"semesters":   parseSelectOptions(body, "executiveEducationPlanNum"),
			"grades":      parseSelectOptions(body, "yearNum"),
			"departments": parseSelectOptions(body, "departmentNum"),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

var classAjaxHeaders = map[string]string{
	"Accept":           "application/json, text/javascript, */*; q=0.01",
	"Referer":          zbase + "/student/teachingResources/classCurriculum/index",
	"User-Agent":       auth.DefaultUserAgent,
	"X-Requested-With": "XMLHttpRequest",
}

// FetchSubjectsByDepartment 根据院系获取专业列表（原始 JSON 数组）。
func (s *ZhjwService) FetchSubjectsByDepartment(departmentNum string) ([]interface{}, error) {
	u := zbase + "/student/teachingResources/gradeAndClassCurriculum/subjectJson?departmentNum=" + url.QueryEscape(departmentNum)
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		var out []interface{}
		if err := zhjwGetJSON(c, u, "subjectJson", classAjaxHeaders, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}

// FetchClassOptions 根据年级/院系/专业获取班级列表（原始 JSON 数组）。
func (s *ZhjwService) FetchClassOptions(yearNum, departmentNum, subjectNum string) ([]interface{}, error) {
	u := zbase + "/student/teachingResources/gradeAndClassCurriculum/classJson" +
		"?departmentNum=" + url.QueryEscape(departmentNum) +
		"&subjectNum=" + url.QueryEscape(subjectNum) +
		"&yearNum=" + url.QueryEscape(yearNum)
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		var out []interface{}
		if err := zhjwGetJSON(c, u, "classJson", classAjaxHeaders, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}

// FetchClassList 搜索班级列表。
func (s *ZhjwService) FetchClassList(pageNum, pageSize int, planNum, yearNum, departmentNum, subjectNum, classNum string) (map[string]interface{}, error) {
	form := url.Values{
		"executiveEducationPlanNum": {planNum},
		"yearNum":                   {yearNum},
		"departmentNum":             {departmentNum},
		"subjectNum":                {subjectNum},
		"classNum":                  {classNum},
		"pageNum":                   {fmt.Sprint(pageNum)},
		"pageSize":                  {fmt.Sprint(pageSize)},
	}
	headers := map[string]string{
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"Content-Type":     "application/x-www-form-urlencoded; charset=UTF-8",
		"Referer":          zbase + "/student/teachingResources/classCurriculum/index",
		"User-Agent":       auth.DefaultUserAgent,
		"X-Requested-With": "XMLHttpRequest",
	}
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		var arr []map[string]interface{}
		if err := zhjwPostJSON(c, zbase+"/student/teachingResources/classCurriculum/search",
			"classCurriculum/search", headers, form, &arr); err != nil {
			return nil, err
		}
		if len(arr) == 0 {
			return map[string]interface{}{"records": []interface{}{}, "total": 0}, nil
		}
		total := 0
		if pc, ok := arr[0]["pageContext"].(map[string]interface{}); ok {
			if n, ok := pc["totalCount"].(float64); ok {
				total = int(n)
			}
		}
		records := arr[0]["records"]
		if records == nil {
			records = []interface{}{}
		}
		return map[string]interface{}{"records": records, "total": total}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]interface{}), nil
}

// FetchClassSchedule 获取指定班级课表（planCode/classCode 来自 FetchClassList）。
func (s *ZhjwService) FetchClassSchedule(planCode, classCode string) ([]interface{}, error) {
	u := zbase + "/student/teachingResources/classCurriculum/searchCurriculumInfo/callback" +
		"?planCode=" + url.QueryEscape(planCode) + "&classCode=" + url.QueryEscape(classCode)
	v, err := s.request(func(c *auth.CookieClient) (interface{}, error) {
		var arr []interface{}
		if err := zhjwGetJSON(c, u, "searchCurriculumInfo/callback", classAjaxHeaders, &arr); err != nil {
			return nil, err
		}
		if len(arr) == 0 {
			return []interface{}{}, nil
		}
		// 服务端正常返回 [[...]]；shape 偏离时给出结构化错误而非 panic。
		list, ok := arr[0].([]interface{})
		if !ok {
			return nil, &auth.ServiceError{Msg: "[searchCurriculumInfo/callback] 响应格式异常：首元素不是数组"}
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]interface{}), nil
}
