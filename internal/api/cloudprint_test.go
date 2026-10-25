package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

type cloudPrintTestAuth struct {
	client        *auth.CookieClient
	invalidations int
}

func (a *cloudPrintTestAuth) ModuleID() string                       { return "cloudprint" }
func (a *cloudPrintTestAuth) GetClient() (*auth.CookieClient, error) { return a.client, nil }
func (a *cloudPrintTestAuth) Invalidate()                            { a.invalidations++ }

func cloudPrintFixture(t *testing.T) (*os.File, os.FileInfo) {
	t.Helper()
	return cloudPrintNamedFixture(t, "测试 document.pdf")
}

func cloudPrintNamedFixture(t *testing.T, name string) (*os.File, os.FileInfo) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("%PDF-1.4\nfixture document\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f, info, err := OpenCloudPrintFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f, info
}

func prepareCloudPrintSettings(t *testing.T, options CloudPrintOptions) *CloudPrintSettings {
	t.Helper()
	settings, err := options.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	return settings
}

func TestCloudPrintOptionsMatchInvestigatedProtocol(t *testing.T) {
	for _, tc := range []struct {
		color, paper, duplex, pages string
		copies                      int
		want                        map[string]string
	}{
		{"bw", "a4", "simplex", "all", 1, map[string]string{"dwColor": "1", "dwPaperId": "9", "dwDuplex": "1", "dwCopies": "1", "dwFrom": "0", "dwTo": "0"}},
		{"color", "a3", "long-edge", "2-3", 2, map[string]string{"dwColor": "2", "dwPaperId": "8", "dwDuplex": "2", "dwCopies": "2", "dwFrom": "2", "dwTo": "3"}},
		{"bw", "original", "short-edge", "4", 1, map[string]string{"dwColor": "1", "dwPaperId": "-1", "dwDuplex": "3", "dwCopies": "1", "dwFrom": "4", "dwTo": "4"}},
	} {
		settings, err := (CloudPrintOptions{Color: tc.color, Paper: tc.paper, Duplex: tc.duplex, Pages: tc.pages, Copies: tc.copies}).Prepare()
		if err != nil {
			t.Fatal(err)
		}
		got := settings.Fields()
		for key, want := range tc.want {
			if got[key] != want {
				t.Errorf("%s=%s, want %s", key, got[key], want)
			}
		}
	}
	for _, pages := range []string{"0", "3-2", "1,3", "-1", "", "1-2-3", "1-", "4294967296"} {
		if _, err := (CloudPrintOptions{Color: "bw", Paper: "a4", Duplex: "simplex", Copies: 1, Pages: pages}).Prepare(); err == nil {
			t.Errorf("accepted invalid range %q", pages)
		}
	}
}

func TestCloudPrintUploadStreamsCorrectFormAndMatchesNewJob(t *testing.T) {
	f, info := cloudPrintFixture(t)
	var posts atomic.Int32
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("print upload must not carry SCU Bearer")
		}
		switch r.URL.Path {
		case "/api/client/Auth/Check":
			fmt.Fprint(w, `{"code":0,"SessionId":"session"}`)
		case "/api/client/PrintJob/Get":
			jobs := []map[string]interface{}{{"dwJobId": 100, "szJobName": info.Name()}}
			if posts.Load() > 0 {
				jobs = append(jobs, map[string]interface{}{"dwJobId": 101, "szJobName": info.Name(), "dwCopies": 2, "szTrueName": "PRIVATE", "szCardNO": "PRIVATE", "szFileName": "PRIVATE", "szPaperDetail": `[{"dwPaperID":8,"dwBWPages":0,"dwColorPages":2,"dwPaperNum":1}]`})
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "result": jobs})
		case "/api/client/CloudPrint/Upload":
			posts.Add(1)
			if r.ContentLength <= info.Size() || len(r.TransferEncoding) != 0 {
				t.Error("multipart must have Content-Length, no chunking")
			}
			if err := r.ParseMultipartForm(1024 * 1024); err != nil {
				t.Error(err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			for key, want := range map[string]string{"dwColor": "2", "dwPaperId": "8", "dwDuplex": "2", "dwCopies": "2", "dwFrom": "2", "dwTo": "3"} {
				if r.FormValue(key) != want {
					t.Errorf("%s=%s", key, r.FormValue(key))
				}
			}
			if !strings.HasPrefix(r.FormValue("BackURL"), base+auth.CloudPrintJobsPage+"?t=") {
				t.Error("wrong BackURL")
			}
			part, header, err := r.FormFile("szPath")
			if err != nil {
				t.Error(err)
				return
			}
			defer part.Close()
			content, _ := io.ReadAll(part)
			if strings.Contains(header.Header.Get("Content-Disposition"), "filename*=") {
				t.Error("older print server requires browser-style UTF-8 filename=, not filename*=")
			}
			if header.Filename != info.Name() || string(content) != "%PDF-1.4\nfixture document\n" || header.Header.Get("Content-Type") != "application/pdf" {
				t.Error("file part corrupted")
			}
			http.Redirect(w, r, auth.CloudPrintJobsPage+"?code=0&fee=40&message=", http.StatusSeeOther)
		case auth.CloudPrintJobsPage:
			fmt.Fprint(w, "print page")
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	base = srv.URL
	c, _ := auth.NewCookieClient()
	a := &cloudPrintTestAuth{client: c}
	s := &CloudPrintService{auth: a, base: base}
	settings := prepareCloudPrintSettings(t, CloudPrintOptions{Color: "color", Paper: "a3", Duplex: "long-edge", Copies: 2, Pages: "2-3"})
	// dry-run 等调用方拿到的是副本，不能改坏随后实际上传的已校验字段。
	fields := settings.Fields()
	fields["dwCopies"] = "99"
	result, err := s.Upload(f, info, settings)
	if err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 1 || result.FeeRaw != "40" || result.Verification != "matched_new_job" || len(result.Jobs) != 1 || result.Jobs[0]["dwJobId"] != float64(101) {
		t.Fatalf("result=%+v, posts=%d", result, posts.Load())
	}
	if _, ok := result.Jobs[0]["szPaperDetail"].([]interface{}); !ok {
		t.Fatal("paper detail string not decoded")
	}
	for _, key := range []string{"szTrueName", "szCardNO", "szFileName"} {
		if _, exists := result.Jobs[0][key]; exists {
			t.Errorf("private field %s leaked", key)
		}
	}
}

func TestCloudPrintUploadNeverReplaysAmbiguousOrRejectedPOST(t *testing.T) {
	for _, mode := range []string{"connection-lost", "307", "missing-code", "auth-expired", "401", "403", "callback-401", "callback-403", "413"} {
		t.Run(mode, func(t *testing.T) {
			f, info := cloudPrintFixture(t)
			var posts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/client/Auth/Check":
					fmt.Fprint(w, `{"code":0,"SessionId":"session"}`)
				case "/api/client/PrintJob/Get":
					if posts.Load() > 0 {
						json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "result": []map[string]interface{}{{"dwJobId": 7, "szJobName": info.Name()}}})
					} else {
						fmt.Fprint(w, `{"code":0,"result":[]}`)
					}
				case "/api/client/CloudPrint/Upload":
					posts.Add(1)
					io.Copy(io.Discard, r.Body)
					switch mode {
					case "connection-lost":
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						conn.Close()
					case "307":
						w.Header().Set("Location", "/api/client/CloudPrint/Upload")
						w.WriteHeader(307)
					case "missing-code":
						fmt.Fprint(w, "ok")
					case "auth-expired":
						http.Redirect(w, r, auth.CloudPrintJobsPage+"?code=4", 302)
					case "401":
						w.WriteHeader(401)
					case "403":
						w.WriteHeader(403)
					case "callback-401", "callback-403":
						http.Redirect(w, r, auth.CloudPrintJobsPage+"?code=0", 302)
					case "413":
						w.WriteHeader(413)
					}
				case auth.CloudPrintJobsPage:
					switch mode {
					case "callback-401":
						w.WriteHeader(401)
					case "callback-403":
						w.WriteHeader(403)
					default:
						fmt.Fprint(w, "print page")
					}
				}
			}))
			defer srv.Close()
			c, _ := auth.NewCookieClient()
			a := &cloudPrintTestAuth{client: c}
			s := &CloudPrintService{auth: a, base: srv.URL}
			_, err := s.Upload(f, info, prepareCloudPrintSettings(t, CloudPrintOptions{Color: "bw", Paper: "a4", Duplex: "simplex", Copies: 1, Pages: "all"}))
			if err == nil || posts.Load() != 1 || a.invalidations != 0 {
				t.Fatalf("err=%v posts=%d invalidations=%d", err, posts.Load(), a.invalidations)
			}
			var uncertain *CloudPrintUploadUncertainError
			if mode != "413" {
				if !errors.As(err, &uncertain) {
					t.Fatalf("want uncertain: %v", err)
				}
				if auth.IsUnauthenticated(err) {
					t.Fatalf("提交后不能引导按 unauthenticated 重试: %v", err)
				}
			} else {
				var rejected *auth.ServiceError
				if !errors.As(err, &rejected) || errors.As(err, &uncertain) {
					t.Fatalf("413 明确拒绝应返回 service: %v", err)
				}
			}
			if mode == "connection-lost" && !strings.Contains(err.Error(), "ID=7") {
				t.Fatalf("missing reconciliation: %v", err)
			}
		})
	}
}

func TestCloudPrintUploadRejectsUnpreparedSettingsBeforeAuth(t *testing.T) {
	f, info := cloudPrintFixture(t)
	s := &CloudPrintService{}
	for _, settings := range []*CloudPrintSettings{nil, {}} {
		if _, err := s.Upload(f, info, settings); err == nil {
			t.Fatal("未校验设置不应触发认证或上传")
		}
	}
}

func TestCloudPrintVerificationUsesIDsAndKeepsRenamedJobsAsCandidates(t *testing.T) {
	name := strings.Repeat("long-document-", 12) + ".pdf"
	newJob := func(id int, remoteName string, copies int, from, to interface{}) map[string]interface{} {
		job := map[string]interface{}{"dwJobId": id, "szJobName": remoteName, "dwCopies": copies}
		if from != nil {
			job["dwFrom"], job["dwTo"] = from, to
		}
		return job
	}
	for _, tc := range []struct {
		name         string
		jobs         []map[string]interface{}
		verification string
		count        int
		pages        string
	}{
		{"truncated", []map[string]interface{}{newJob(101, "long-document-long-doc.pdf", 2, 2, 3)}, "candidate_new_job", 1, "2-3"},
		{"renamed", []map[string]interface{}{newJob(101, "converted.pdf", 2, 2, 3)}, "candidate_new_job", 1, "2-3"},
		{"missing-settings", []map[string]interface{}{{"dwJobId": 101, "szJobName": "long-document.pdf"}}, "candidate_new_job", 1, "2-3"},
		{"conflicting-copies", []map[string]interface{}{newJob(101, name, 3, 2, 3)}, "pending", 0, "2-3"},
		{"conflicting-range", []map[string]interface{}{newJob(101, name, 2, 1, 3)}, "pending", 0, "2-3"},
		{"existing-id", []map[string]interface{}{newJob(100, name, 2, 2, 3)}, "pending", 0, "2-3"},
		{"multiple-candidates", []map[string]interface{}{newJob(101, "converted.pdf", 2, 2, 3), newJob(102, "another.pdf", 2, 2, 3)}, "ambiguous", 2, "2-3"},
		{"exact-name-with-other-job", []map[string]interface{}{newJob(101, name, 2, 2, 3), newJob(102, "another.pdf", 3, 2, 3)}, "matched_new_job", 1, "2-3"},
		{"duplicate-row", []map[string]interface{}{newJob(101, name, 2, 2, 3), newJob(101, name, 2, 2, 3)}, "matched_new_job", 1, "2-3"},
		{"all-pages-normalized", []map[string]interface{}{newJob(101, name, 2, 1, 20)}, "matched_new_job", 1, "all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SCU_CLI_CONFIG_DIR", t.TempDir())
			f, info := cloudPrintNamedFixture(t, name)
			var posts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/client/Auth/Check":
					fmt.Fprint(w, `{"code":0,"SessionId":"session"}`)
				case "/api/client/PrintJob/Get":
					jobs := []map[string]interface{}{newJob(100, name, 2, 2, 3)}
					if posts.Load() > 0 {
						jobs = append(jobs, tc.jobs...)
					}
					json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "result": jobs})
				case "/api/client/CloudPrint/Upload":
					posts.Add(1)
					io.Copy(io.Discard, r.Body)
					http.Redirect(w, r, auth.CloudPrintJobsPage+"?code=0", 303)
				case auth.CloudPrintJobsPage:
					fmt.Fprint(w, "print page")
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer srv.Close()
			c, err := auth.NewCookieClient()
			if err != nil {
				t.Fatal(err)
			}
			s := &CloudPrintService{auth: &cloudPrintTestAuth{client: c}, base: srv.URL}
			settings := prepareCloudPrintSettings(t, CloudPrintOptions{Color: "bw", Paper: "a4", Duplex: "long-edge", Copies: 2, Pages: tc.pages})
			result, err := s.Upload(f, info, settings)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "uploaded" || result.Verification != tc.verification || len(result.Jobs) != tc.count || posts.Load() != 1 {
				t.Fatalf("result=%+v posts=%d", result, posts.Load())
			}
			if tc.verification != "matched_new_job" && !strings.Contains(result.Warning, "不要直接重复上传") {
				t.Fatalf("missing no-replay guidance: %+v", result)
			}
		})
	}
}
