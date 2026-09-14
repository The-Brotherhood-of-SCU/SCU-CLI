package auth

import (
	"encoding/json"
	"strings"
	"testing"
)

// openssl 独立实现的 AES-128-CBC(PKCS7) 向量（key/iv 同为 "1974051005060708"），
// 用于交叉验证与 Bugaoshan Dart encrypt 包行为一致。
func TestZhhqEncryptMatchesOpenSSLVector(t *testing.T) {
	got, err := ZhhqEncrypt("hello zhhq", ZhhqResponseKey, ZhhqResponseIV)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if want := "10I2bKfTnvUh3Y3NeuvqIg=="; got != want {
		t.Fatalf("encrypt = %q, want %q", got, want)
	}
}

func TestZhhqDecryptOpenSSLVector(t *testing.T) {
	got, err := ZhhqDecrypt("KCZ+7o41+kywq/8vTPqM4n2Y5/W7Lr9uSPXTBA4DB/xbJF/pe1YLccF+GdwsOzXx5OwIzcutEX/eUdLj9fdcHg==",
		ZhhqResponseKey, ZhhqResponseIV)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if want := `{"status":"success","errorCode":"0","data":[1,2,3]}`; string(got) != want {
		t.Fatalf("decrypt = %q, want %q", got, want)
	}
}

func TestZhhqEncryptDecryptRoundtrip(t *testing.T) {
	cases := []string{
		"",                        // 空串（单块纯填充）
		strings.Repeat("a", 15),   // 块边界 -1
		strings.Repeat("b", 16),   // 恰好整块（多一层填充块）
		strings.Repeat("中文✓", 40), // 多字节 UTF-8 多块
	}
	for _, plain := range cases {
		// Token 加密参数（key=clientSecret, iv=clientId）。
		ct, err := ZhhqEncrypt(plain, ZhhqClientSecret, ZhhqClientID)
		if err != nil {
			t.Fatalf("encrypt(%q): %v", plain, err)
		}
		got, err := ZhhqDecrypt(ct, ZhhqClientSecret, ZhhqClientID)
		if err != nil {
			t.Fatalf("decrypt(%q): %v", plain, err)
		}
		if string(got) != plain {
			t.Fatalf("roundtrip = %q, want %q", got, plain)
		}
	}
}

func TestZhhqDecryptRejectsBadInput(t *testing.T) {
	if _, err := ZhhqDecrypt("not-base64!!!", ZhhqResponseKey, ZhhqResponseIV); err == nil {
		t.Fatal("非 base64 应报错")
	}
	if _, err := ZhhqDecrypt("AAAAAAAAAAAAAAAAAAAAAA==", ZhhqResponseKey, ZhhqResponseIV); err == nil {
		t.Fatal("填充不合法应报错")
	}
}

func TestZhhqDecodeResponse(t *testing.T) {
	body, err := ZhhqEncrypt(`{"status":"success","errorCode":"0","data":"abc"}`, ZhhqResponseKey, ZhhqResponseIV)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	out, err := ZhhqDecodeResponse(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["status"] != "success" || out["data"] != "abc" {
		t.Fatalf("decode = %v", out)
	}
	if _, err := ZhhqDecodeResponse("<<<html>>>"); err == nil {
		t.Fatal("非加密 JSON 应报错")
	}
}

func TestZhhqBuildToken(t *testing.T) {
	token, err := ZhhqBuildToken("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	plain, err := ZhhqDecrypt(token, ZhhqClientSecret, ZhhqClientID)
	if err != nil {
		t.Fatalf("decrypt token: %v", err)
	}
	// 各字段断言（timestamp/GUID 由 json.Unmarshal 宽松解析）。
	var payload struct {
		TokenKey  string  `json:"tokenKey"`
		ClientID  string  `json:"clientId"`
		Timestamp float64 `json:"timestamp"`
		GUID      string  `json:"GUID"`
	}
	if err := json.Unmarshal(plain, &payload); err != nil {
		t.Fatalf("parse token payload: %v", err)
	}
	if payload.TokenKey != "0123456789abcdef0123456789abcdef" || payload.ClientID != ZhhqClientID {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Timestamp <= 0 {
		t.Fatalf("timestamp = %v", payload.Timestamp)
	}
	// v4 风格 GUID 形态：xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	if len(payload.GUID) != 36 || payload.GUID[14] != '4' {
		t.Fatalf("GUID = %q", payload.GUID)
	}
	y := payload.GUID[19]
	if y != '8' && y != '9' && y != 'a' && y != 'b' {
		t.Fatalf("GUID variant = %q", y)
	}
}
