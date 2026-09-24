package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestDoReturnsRedirectAsResponse 3xx 必须作为正常响应返回（err == nil），
// 由 FollowRedirects 读 Location 逐跳跟随。
//
// 回归测试：resty.NoRedirectPolicy() 会让 net/http 把任何 3xx 包装成
// *url.Error 上抛（auto redirect is disabled），整个 SSO 链路无法工作。
// 正确做法是 CheckRedirect 返回 http.ErrUseLastResponse 哨兵。
func TestDoReturnsRedirectAsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	c, err := NewCookieClient()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Get(srv.URL+"/x", nil)
	if err != nil {
		t.Fatalf("3xx 应作为正常响应返回，实际报错: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/elsewhere" {
		t.Fatalf("Location = %q", loc)
	}
}

// TestFollowRedirectsWithSpaceInLocation 模拟真实教务 SSO 链路：
//
//	/sso → 302（Location 含字面空格，统一认证的服务端 bug：
//	      "…/sigin ?id_token=…"）→ /sigin%20 → 302 → /index(200)
//
// 浏览器与 Dart http 会自动百分号编码该空格，客户端必须同样容错，
// 且每一跳的 Set-Cookie 都要收集、逐跳携带。
func TestFollowRedirectsWithSpaceInLocation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sso", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "hop1=aaa; Path=/")
		w.Header().Set("Location", "/sigin ?id_token=abc&target_url=index")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/sigin"):
			if r.URL.Query().Get("id_token") != "abc" {
				t.Errorf("sigin 请求 query 异常: %q", r.URL.RawQuery)
			}
			w.Header().Set("Set-Cookie", "hop2=bbb; Path=/")
			w.Header().Set("Location", "/index")
			w.WriteHeader(http.StatusFound)
		case r.URL.Path == "/index":
			cookies := r.Header.Get("Cookie")
			if !strings.Contains(cookies, "hop1=aaa") || !strings.Contains(cookies, "hop2=bbb") {
				t.Errorf("/index 未携带逐跳收集的 cookie: %q", cookies)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewCookieClient()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.FollowRedirects(srv.URL+"/sso", nil)
	if err != nil {
		t.Fatalf("FollowRedirects: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("final status = %d, want 200", resp.StatusCode)
	}
	if string(resp.Body) != "ok" {
		t.Fatalf("final body = %q", resp.Body)
	}
}

// TestFollowRedirectsStripsSensitiveHeadersCrossOrigin 跨源重定向默认剥离
// Authorization 等敏感 header；显式 AllowSensitiveOrigin 后才转发。
func TestFollowRedirectsStripsSensitiveHeadersCrossOrigin(t *testing.T) {
	var bGotAuth bool
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bGotAuth = r.Header.Get("Authorization") != ""
		w.WriteHeader(http.StatusOK)
	}))
	defer b.Close()

	var aGotAuth bool
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aGotAuth = r.Header.Get("Authorization") != ""
		w.Header().Set("Location", b.URL+"/landing")
		w.WriteHeader(http.StatusFound)
	}))
	defer a.Close()

	c, err := NewCookieClient()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.FollowRedirects(a.URL+"/start", map[string]string{"Authorization": "Bearer x"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("final status = %d", resp.StatusCode)
	}
	if !aGotAuth {
		t.Error("首跳同源请求应携带 Authorization")
	}
	if bGotAuth {
		t.Error("跨源重定向不应转发 Authorization")
	}

	// 显式允许后应转发
	c2, err := NewCookieClient()
	if err != nil {
		t.Fatal(err)
	}
	c2.AllowSensitiveOrigin(b.URL)
	if _, err := c2.FollowRedirects(a.URL+"/start", map[string]string{"Authorization": "Bearer x"}); err != nil {
		t.Fatal(err)
	}
	if !bGotAuth {
		t.Error("AllowSensitiveOrigin 后跨源应转发 Authorization")
	}
}

// TestRequestTimeoutEnvOverride SCU_CLI_TIMEOUT 环境变量覆盖单次请求超时。
func TestRequestTimeoutEnvOverride(t *testing.T) {
	cases := []struct {
		env  string
		want time.Duration
	}{
		{"5", 5 * time.Second},
		{"", defaultTimeout},
		{"bogus", defaultTimeout},
		{"-3", defaultTimeout},
		{"0", defaultTimeout},
	}
	for _, tc := range cases {
		if tc.env == "" {
			t.Setenv("SCU_CLI_TIMEOUT", "")
			os.Unsetenv("SCU_CLI_TIMEOUT")
		} else {
			t.Setenv("SCU_CLI_TIMEOUT", tc.env)
		}
		if got := requestTimeout(); got != tc.want {
			t.Errorf("SCU_CLI_TIMEOUT=%q: requestTimeout() = %v, want %v", tc.env, got, tc.want)
		}
	}
}

// TestDoTimesOut 请求超过配置超时必须返回错误而非永久挂起
// （Do 的传输层重试会用同样的超时重发一次，总耗时仍有限）。
func TestDoTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("SCU_CLI_TIMEOUT", "1")
	c, err := NewCookieClient()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := c.Get(srv.URL+"/x", nil); err == nil {
		t.Fatal("超时请求应返回错误")
	}
	if elapsed := time.Since(start); elapsed >= 3*time.Second {
		t.Fatalf("超时未生效: 耗时 %v", elapsed)
	}
}
