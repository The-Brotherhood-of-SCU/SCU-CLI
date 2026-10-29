package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

type cloudPrintTestRoot struct{ client *CookieClient }

func (r *cloudPrintTestRoot) GetClient() (*CookieClient, error) { return r.client, nil }

func TestCloudPrintJSONLoginCookieChainAndRootChange(t *testing.T) {
	var logins atomic.Int32
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("SCU Bearer must not reach print requests")
		}
		switch r.URL.Path {
		case "/api/client/Auth/Check":
			if r.Method != "POST" {
				t.Error("Auth/Check must be POST")
			}
			if _, err := r.Cookie("print_ready"); err == nil {
				fmt.Fprint(w, `{"code":0,"SessionId":"test-session","result":{}}`)
			} else {
				http.SetCookie(w, &http.Cookie{Name: "anonymous", Value: "same-jar", Path: "/"})
				fmt.Fprint(w, `{"code":4}`)
			}
		case "/api/client/Auth/SSoPage":
			if r.URL.Query().Get("backurl") != base+CloudPrintPage {
				t.Error("wrong backurl")
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "result": base + "/library"})
		case "/library":
			if _, err := r.Cookie("anonymous"); err != nil {
				t.Error("SSO lost anonymous cookie")
			}
			logins.Add(1)
			http.Redirect(w, r, "/cas", http.StatusFound)
		case "/cas":
			http.SetCookie(w, &http.Cookie{Name: "print_ready", Value: "yes", Path: "/"})
			http.Redirect(w, r, CloudPrintPage, http.StatusSeeOther)
		case CloudPrintPage:
			fmt.Fprint(w, "print page")
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	base = srv.URL
	client, _ := NewCookieClient()
	root := &cloudPrintTestRoot{client: client}
	a := &CloudPrintAuth{scu: root, base: base, allowed: func(u *url.URL) bool { return u.Scheme+"://"+u.Host == base }}
	for i := 0; i < 2; i++ {
		if got, err := a.GetClient(); err != nil || got != client {
			t.Fatalf("GetClient: %v", err)
		}
	}
	if logins.Load() != 1 {
		t.Fatal("successful SSO must be cached")
	}
	root.client, _ = NewCookieClient()
	if _, err := a.GetClient(); err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 2 {
		t.Fatal("root identity change must rebuild the print session")
	}
}

func TestCloudPrintRejectsMissingCodeAndSession(t *testing.T) {
	for _, body := range []string{`{}`, `{"result":[]}`, `<html>login</html>`} {
		if _, err := DecodeCloudPrintResponse(&Response{StatusCode: 200, Body: []byte(body)}); err == nil {
			t.Fatalf("accepted malformed envelope %s", body)
		}
	}
	if _, err := DecodeCloudPrintResponse(&Response{StatusCode: 200, Body: []byte(`{"code":4}`)}); !IsUnauthenticated(err) {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"code":0}`) }))
	defer srv.Close()
	client, _ := NewCookieClient()
	if err := CheckCloudPrint(client, srv.URL); !IsUnauthenticated(err) {
		t.Fatalf("code=0 without SessionId: %v", err)
	}
}

func TestCloudPrintSSOOriginPolicy(t *testing.T) {
	for _, raw := range []string{"http://202.115.54.240/x", "https://libic.scu.edu.cn/x", "https://id.scu.edu.cn/x"} {
		u, _ := url.Parse(raw)
		if !cloudPrintOriginAllowed(u) {
			t.Fatal(raw)
		}
	}
	for _, raw := range []string{"http://id.scu.edu.cn/x", "https://libic.scu.edu.cn.evil.test/x", "https://user:pass@id.scu.edu.cn/x", "https://id.scu.edu.cn:444/x", "file:///etc/passwd"} {
		u, _ := url.Parse(raw)
		if cloudPrintOriginAllowed(u) {
			t.Fatal(raw)
		}
	}
	var reached atomic.Int32
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached.Add(1) }))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, bad.URL+"/?ticket=SECRET", 302) }))
	defer good.Close()
	client, _ := NewCookieClient()
	_, err := client.FollowRedirectsAllowed(good.URL, nil, func(u *url.URL) bool { return u.Scheme+"://"+u.Host == good.URL })
	if err == nil || strings.Contains(err.Error(), "SECRET") || reached.Load() != 0 {
		t.Fatalf("unsafe redirect: err=%v, reached=%d", err, reached.Load())
	}
}
