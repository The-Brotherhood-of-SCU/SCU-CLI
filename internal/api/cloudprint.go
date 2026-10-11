package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

type CloudPrintOptions struct {
	Color  string `json:"color"`
	Paper  string `json:"paper"`
	Duplex string `json:"duplex"`
	Copies int    `json:"copies"`
	Pages  string `json:"pages"`
}

// Fields validates user options and translates only the observed protocol values.
func (o CloudPrintOptions) Fields() (map[string]string, error) {
	color := map[string]string{"bw": "1", "color": "2"}[strings.ToLower(o.Color)]
	paper := map[string]string{"a4": "9", "a3": "8", "original": "-1"}[strings.ToLower(o.Paper)]
	duplex := map[string]string{"simplex": "1", "long-edge": "2", "short-edge": "3"}[o.Duplex]
	if color == "" {
		return nil, fmt.Errorf("--color 必须为 bw 或 color")
	}
	if paper == "" {
		return nil, fmt.Errorf("--paper 必须为 a4、a3 或 original")
	}
	if duplex == "" {
		return nil, fmt.Errorf("--duplex 必须为 simplex、long-edge 或 short-edge")
	}
	if o.Copies < 1 || uint64(o.Copies) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("--copies 必须为正整数（不超过 uint32 范围）")
	}
	from, to := "0", "0"
	if o.Pages != "all" {
		parts := strings.Split(o.Pages, "-")
		if len(parts) < 1 || len(parts) > 2 {
			return nil, fmt.Errorf("--pages 必须为 all、单页编号或连续范围（如 2-3）")
		}
		first, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil || first == 0 {
			return nil, fmt.Errorf("--pages 起始页必须为正整数")
		}
		last := first
		if len(parts) == 2 {
			last, err = strconv.ParseUint(parts[1], 10, 32)
			if err != nil || last < first {
				return nil, fmt.Errorf("--pages 结束页必须不小于起始页")
			}
		}
		from, to = strconv.FormatUint(first, 10), strconv.FormatUint(last, 10)
	}
	return map[string]string{"dwColor": color, "dwPaperId": paper, "dwDuplex": duplex,
		"dwCopies": strconv.Itoa(o.Copies), "dwFrom": from, "dwTo": to}, nil
}

var cloudPrintExtensions = strings.Fields("jpg jpeg heic png pdf txt doc xls docx xlsx ppt pptx wps wpt dot rtf dotx docm dotm xml mht mhtml html htm xps odt xlsm xlsb csv xltx xltm xlt prn dif slk xla xlam ods dps dpt pot pps pptm potm ppsm")

// OpenCloudPrintFile verifies a readable, nonempty regular file before any SSO
// or write request. The returned open file is also used for upload (no reopen).
func OpenCloudPrintFile(path string) (*os.File, os.FileInfo, error) {
	if strings.ContainsAny(filepath.Base(path), "\r\n") {
		return nil, nil, fmt.Errorf("文件名不能含换行符")
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	supported := false
	for _, candidate := range cloudPrintExtensions {
		if ext == candidate {
			supported = true
			break
		}
	}
	if !supported {
		return nil, nil, fmt.Errorf("不支持文件后缀 %q；支持 PDF、图片、TXT 和 Office 文档等网页允许的格式", ext)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("无法读取文件: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		f.Close()
		return nil, nil, fmt.Errorf("必须提供非空普通文件")
	}
	return f, info, nil
}

type CloudPrintService struct {
	auth auth.SubsystemAuth
	base string
}

func NewCloudPrintService(a auth.SubsystemAuth) *CloudPrintService {
	return &CloudPrintService{auth: a, base: auth.CloudPrintBase}
}

func (s *CloudPrintService) Check() error {
	_, err := auth.RetryOnUnauthenticated(s.auth.GetClient, func(c *auth.CookieClient) (bool, error) {
		return true, auth.CheckCloudPrint(c, s.base)
	}, s.auth.Invalidate)
	return err
}

func (s *CloudPrintService) Jobs() ([]map[string]interface{}, error) {
	return auth.RetryOnUnauthenticated(s.auth.GetClient, func(c *auth.CookieClient) ([]map[string]interface{}, error) {
		resp, err := c.Get(s.base+"/api/client/PrintJob/Get", map[string]string{"Accept": "application/json", "Referer": s.base + auth.CloudPrintJobsPage})
		if err != nil {
			return nil, &auth.ServiceError{Msg: "云打印任务列表请求失败（网络或超时）"}
		}
		envelope, err := auth.DecodeCloudPrintResponse(resp)
		if err != nil {
			return nil, err
		}
		jobs := make([]map[string]interface{}, 0)
		if len(envelope.Result) == 0 {
			return nil, &auth.ServiceError{Msg: "云打印任务列表缺少 result 字段"}
		}
		if string(envelope.Result) != "null" {
			if json.Unmarshal(envelope.Result, &jobs) != nil {
				return nil, &auth.ServiceError{Msg: "云打印任务列表 result 不是数组"}
			}
		}
		for i, rawJob := range jobs {
			// Keep task metadata; account/card/name fields and server file paths
			// are unnecessary for printing and should not enter agent logs.
			job := make(map[string]interface{})
			for _, key := range []string{"dwJobId", "dwOldJobId", "dwCreateDate", "dwCreateTime", "dwPrintDate", "dwPrintTime", "dwPages", "dwCopies", "dwStatus", "dwFee", "szAttribe", "szPaperDetail", "szJobName"} {
				if value, exists := rawJob[key]; exists {
					job[key] = value
				}
			}
			jobs[i] = job
			if raw, ok := job["szPaperDetail"].(string); ok && raw != "" {
				var detail interface{}
				if json.Unmarshal([]byte(raw), &detail) == nil {
					job["szPaperDetail"] = detail
				}
			}
		}
		return jobs, nil
	}, s.auth.Invalidate)
}

type CloudPrintUploadResult struct {
	Status       string                   `json:"status"`
	FileName     string                   `json:"file_name"`
	Settings     CloudPrintOptions        `json:"settings"`
	FeeRaw       string                   `json:"fee_raw,omitempty"`
	Jobs         []map[string]interface{} `json:"jobs"`
	Verification string                   `json:"verification"`
	Message      string                   `json:"message"`
	Warning      string                   `json:"warning,omitempty"`
}

// CloudPrintUploadUncertainError must never trigger automatic upload replay.
type CloudPrintUploadUncertainError struct{ Reason string }

func (e *CloudPrintUploadUncertainError) Error() string {
	return e.Reason + "；上传未自动重发，请用 scu cloudprint jobs 核对任务后再决定是否重新上传"
}

// multipartFile streams the opened file, preserving Content-Length for this
// older server, without buffering the file or creating a replayable request.
func multipartFile(f *os.File, info os.FileInfo, fields map[string]string) (io.Reader, string, int64, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, name := range []string{"dwColor", "dwPaperId", "dwDuplex", "dwCopies", "dwFrom", "dwTo", "BackURL"} {
		if err := w.WriteField(name, fields[name]); err != nil {
			return nil, "", 0, err
		}
	}
	h := make(textproto.MIMEHeader)
	// Match browser FormData: this older server does not understand RFC 5987's
	// filename*= form for non-ASCII names. Keep UTF-8 in the quoted filename.
	escape := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	h.Set("Content-Disposition", fmt.Sprintf("form-data; name=\"szPath\"; filename=\"%s\"", escape.Replace(info.Name())))
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name())))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	h.Set("Content-Type", contentType)
	if _, err := w.CreatePart(h); err != nil {
		return nil, "", 0, err
	}
	head := append([]byte(nil), buf.Bytes()...)
	buf.Reset()
	if err := w.Close(); err != nil {
		return nil, "", 0, err
	}
	tail := append([]byte(nil), buf.Bytes()...)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, "", 0, err
	}
	return io.MultiReader(bytes.NewReader(head), io.LimitReader(f, info.Size()), bytes.NewReader(tail)), w.FormDataContentType(), int64(len(head)) + info.Size() + int64(len(tail)), nil
}

func (s *CloudPrintService) uploadResult(c *auth.CookieClient, resp *auth.Response) (string, error) {
	base, _ := url.Parse(s.base)
	for hop := 0; hop < 10; hop++ {
		switch resp.StatusCode {
		case 301, 302, 303:
			next, err := resp.URL.Parse(resp.Header.Get("Location"))
			if err != nil || resp.Header.Get("Location") == "" || next.User != nil || next.Scheme != base.Scheme || next.Host != base.Host || next.Path != auth.CloudPrintJobsPage {
				return "", &CloudPrintUploadUncertainError{Reason: "上传回跳目标异常"}
			}
			resp, err = c.Get(next.String(), map[string]string{"Referer": s.base + auth.CloudPrintPage})
			if err != nil {
				return "", &CloudPrintUploadUncertainError{Reason: "文件提交后的回跳请求失败"}
			}
			continue
		case 307, 308:
			return "", &CloudPrintUploadUncertainError{Reason: "上传返回要求重发 POST 的重定向，已停止跟随"}
		case 413:
			return "", &auth.ServiceError{Msg: "云打印服务器拒绝文件（HTTP 413，文件过大）"}
		case 401, 403:
			return "", &auth.UnauthenticatedError{Msg: "云打印上传认证被拒绝；未自动重发，请重新登录并先核对任务列表"}
		}
		if resp.StatusCode != 200 || resp.URL == nil {
			return "", &CloudPrintUploadUncertainError{Reason: fmt.Sprintf("上传响应异常（HTTP %d）", resp.StatusCode)}
		}
		codes, exists := resp.URL.Query()["code"]
		if !exists || len(codes) != 1 {
			return "", &CloudPrintUploadUncertainError{Reason: "上传响应缺少唯一的 code 参数"}
		}
		code, err := strconv.Atoi(codes[0])
		if err != nil {
			return "", &CloudPrintUploadUncertainError{Reason: "上传回跳 code 不是整数"}
		}
		if code != 0 {
			if code == 4 {
				return "", &auth.UnauthenticatedError{Msg: "云打印上传会话失效；未自动重发，请先核对任务列表"}
			}
			return "", &auth.ServiceError{Msg: fmt.Sprintf("云打印上传失败（code=%d）: %s", code, resp.URL.Query().Get("message"))}
		}
		return resp.URL.Query().Get("fee"), nil
	}
	return "", &CloudPrintUploadUncertainError{Reason: "上传回跳次数超过限制"}
}

func newCloudPrintJobs(before, after []map[string]interface{}, name string) []map[string]interface{} {
	known := make(map[string]bool)
	for _, job := range before {
		known[fmt.Sprint(job["dwJobId"])] = true
	}
	added := make([]map[string]interface{}, 0)
	for _, job := range after {
		id, exists := job["dwJobId"]
		if exists && !known[fmt.Sprint(id)] && job["szJobName"] == name {
			added = append(added, job)
		}
	}
	return added
}

// Upload retries only authentication/read operations BEFORE submission. Once
// DoOnce is called the multipart POST is never replayed, including auth failure.
func (s *CloudPrintService) Upload(f *os.File, info os.FileInfo, options CloudPrintOptions) (*CloudPrintUploadResult, error) {
	fields, err := options.Fields()
	if err != nil {
		return nil, err
	}
	if err := s.Check(); err != nil {
		return nil, err
	}
	before, err := s.Jobs()
	if err != nil {
		return nil, err
	}
	c, err := s.auth.GetClient()
	if err != nil {
		return nil, err
	}
	fields["BackURL"] = s.base + auth.CloudPrintJobsPage + "?t=" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	body, contentType, size, err := multipartFile(f, info, fields)
	if err != nil {
		return nil, err
	}
	resp, err := c.DoOnce("POST", s.base+"/api/client/CloudPrint/Upload", map[string]string{
		"Content-Type": contentType, "Origin": s.base, "Referer": s.base + auth.CloudPrintPage,
	}, body, size)
	if err != nil {
		// Reading the list is safe; an observed task remains a candidate because
		// another client could concurrently upload a file with the same name.
		reason := "文件上传请求失败，服务器是否已接收未知"
		if after, listErr := s.Jobs(); listErr == nil {
			if added := newCloudPrintJobs(before, after, info.Name()); len(added) > 0 {
				ids := make([]string, 0, len(added))
				for _, job := range added {
					ids = append(ids, fmt.Sprint(job["dwJobId"]))
				}
				reason += "，已观察到新增同名任务 ID=" + strings.Join(ids, ",")
			}
		}
		return nil, &CloudPrintUploadUncertainError{Reason: reason}
	}
	fee, err := s.uploadResult(c, resp)
	if err != nil {
		return nil, err
	}
	result := &CloudPrintUploadResult{Status: "uploaded", FileName: info.Name(), Settings: options, FeeRaw: fee,
		Jobs: make([]map[string]interface{}, 0), Verification: "pending", Message: "文件已上传到云打印系统；到馆内终端刷校园卡确认打印"}
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		after, listErr := s.Jobs()
		if listErr != nil {
			result.Warning = "上传 code=0，但任务列表核对失败；请稍后运行 scu cloudprint jobs。不要直接重复上传"
			return result, nil
		}
		result.Jobs = newCloudPrintJobs(before, after, info.Name())
		if len(result.Jobs) == 1 {
			result.Verification = "matched_new_job"
			return result, nil
		}
		if len(result.Jobs) > 1 {
			result.Verification = "ambiguous"
			result.Warning = "观察到多个新增同名任务，请核对 jobs"
			return result, nil
		}
	}
	result.Warning = "上传 code=0，任务尚未出现在列表中；请稍后运行 scu cloudprint jobs。不要直接重复上传"
	return result, nil
}
