package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ─── zhhq 平台 AES 加解密（与 Bugaoshan zhhq_crypto.dart 一致） ────
//
// zhhq 前端（webpack 模块 7b7e）使用 AES-128-CBC + PKCS7：
//   - 响应体解密默认 key/iv：1974051005060708
//   - Token 加密：key = clientSecret、iv = clientId
//
// clientId/clientSecret 是前端 JS 里的公开常量（webpack 模块 83d6），
// 作用只是让请求通过服务端签名校验，不是凭据。

const (
	// ZhhqResponseKey/ZhhqResponseIV 响应体解密默认 key/iv（16 字节 ASCII）。
	ZhhqResponseKey = "1974051005060708"
	ZhhqResponseIV  = "1974051005060708"

	// ZhhqClientID/ZhhqClientSecret 智慧后勤客户端固定配置。
	ZhhqClientID     = "web201911chengdu"
	ZhhqClientSecret = "bf8ec0449942e7f4"
)

// zhhqPad PKCS7 填充。
func zhhqPad(plaintext []byte) []byte {
	pad := aes.BlockSize - len(plaintext)%aes.BlockSize
	return append(plaintext, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

// zhhqUnpad PKCS7 去填充。
func zhhqUnpad(padded []byte) ([]byte, error) {
	if len(padded) == 0 || len(padded)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度非块大小整数倍")
	}
	pad := int(padded[len(padded)-1])
	if pad == 0 || pad > aes.BlockSize || pad > len(padded) {
		return nil, fmt.Errorf("非法 PKCS7 填充")
	}
	for _, b := range padded[len(padded)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("非法 PKCS7 填充")
		}
	}
	return padded[:len(padded)-pad], nil
}

// ZhhqEncrypt AES-128-CBC(PKCS7) 加密，返回 base64 密文（与前端 s() 一致）。
func ZhhqEncrypt(plaintext, key, iv string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	padded := zhhqPad([]byte(plaintext))
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(iv)).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

// ZhhqDecrypt 解密 base64 密文，返回 UTF-8 明文（与前端 l() 一致）。
func ZhhqDecrypt(ciphertextBase64, key, iv string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertextBase64))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度非块大小整数倍")
	}
	mode := cipher.NewCBCDecrypter(block, []byte(iv))
	plain := make([]byte, len(data))
	mode.CryptBlocks(plain, data)
	return zhhqUnpad(plain)
}

// ZhhqDecodeResponse 解密 zhhq 响应体并解析为 JSON Map（解密/解析失败返回错误）。
func ZhhqDecodeResponse(body string) (map[string]interface{}, error) {
	plain, err := ZhhqDecrypt(body, ZhhqResponseKey, ZhhqResponseIV)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// zhhqGUID 生成 v4 风格 GUID（crypto/rand，与 Bugaoshan Random.secure 对应）。
func zhhqGUID() (string, error) {
	pattern := "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"
	var b [1]byte
	var out strings.Builder
	for _, c := range pattern {
		switch c {
		case 'x', 'y':
			if _, err := rand.Read(b[:]); err != nil {
				return "", err
			}
			n := int(b[0]) & 0xF
			if c == 'x' {
				out.WriteString(fmt.Sprintf("%x", n))
			} else {
				out.WriteString(fmt.Sprintf("%x", 3&n|8))
			}
		default:
			out.WriteRune(c)
		}
	}
	return out.String(), nil
}

// ZhhqBuildToken 生成业务请求的 Token 头：AES 加密
// {tokenKey, clientId, timestamp, GUID}（key=clientSecret, iv=clientId）。
func ZhhqBuildToken(tokenKey string) (string, error) {
	guid, err := zhhqGUID()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]interface{}{
		"tokenKey":  tokenKey,
		"clientId":  ZhhqClientID,
		"timestamp": time.Now().UnixMilli(),
		"GUID":      guid,
	})
	if err != nil {
		return "", err
	}
	return ZhhqEncrypt(string(payload), ZhhqClientSecret, ZhhqClientID)
}

// ─── zhhq 认证（与 Bugaoshan zhhq_auth.dart 一致） ────────────────
//
// zhhq 走 SCU 统一身份认证的 scdxplugin_jwt31 SSO 链（与教务 jwt23 同构）：
//  1. SCU Bearer token 访问 casUrl → 重定向回跳 zhhq /account/login?userinfo=<加密串>
//  2. userinfo 调 POST /api/auth/login/auto 换 tokenKey
//  3. 业务请求头带 Token（每次新生成）+ TokenKey
//
// CLI 是单进程短生命周期，tokenKey 只在进程内缓存，不做持久化。

// ZhhqAuth 智慧后勤子系统认证。
type ZhhqAuth struct {
	scu *ScuAuth

	mu        sync.Mutex
	tokenKey  string
	lastScuCl *CookieClient
}

// NewZhhqAuth 创建 zhhq 认证。
func NewZhhqAuth(scu *ScuAuth) *ZhhqAuth { return &ZhhqAuth{scu: scu} }

// zhhq 的 CAS 登录入口（account/config.json 的 casUrl）。
const zhhqCasURL = "https://id.scu.edu.cn/enduser/sp/sso/scdxplugin_jwt31?enterpriseId=scdx"

const zhhqLoginAutoURL = "https://zhhq.scu.edu.cn/api/auth/login/auto"

func (z *ZhhqAuth) ModuleID() string { return "zhhq" }

// Session 返回 SCU client 与 zhhq tokenKey（必要时走 SSO 建立）。
func (z *ZhhqAuth) Session() (*CookieClient, string, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	scuClient, err := z.scu.GetClient()
	if err != nil {
		return nil, "", err
	}
	if scuClient != z.lastScuCl {
		z.lastScuCl = scuClient
		z.tokenKey = ""
	}
	if z.tokenKey != "" {
		return scuClient, z.tokenKey, nil
	}
	token, err := z.scu.GetAccessToken()
	if err != nil {
		return nil, "", err
	}
	resp, err := scuClient.FollowRedirects(zhhqCasURL, map[string]string{
		"Accept":        "text/html,application/xhtml+xml,*/*",
		"User-Agent":    DefaultUserAgent,
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, "", &UnauthenticatedError{Msg: "zhhq SSO 登录已失效"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, "", &ServiceError{Msg: fmt.Sprintf("zhhq SSO 中继失败(HTTP %d)", resp.StatusCode)}
	}
	// 从回跳 URL 解析 userinfo（account/login?userinfo=<加密串>）。
	var userinfo string
	if resp.URL != nil {
		userinfo = resp.URL.Query().Get("userinfo")
	}
	if userinfo == "" {
		return nil, "", &UnauthenticatedError{Msg: "zhhq 认证回跳缺少 userinfo"}
	}
	tokenKey, err := z.exchangeTokenKey(scuClient, userinfo)
	if err != nil {
		return nil, "", err
	}
	z.tokenKey = tokenKey
	return scuClient, tokenKey, nil
}

// exchangeTokenKey 用 userinfo 调 login/auto 换 tokenKey。
// 响应体为 AES 加密 JSON，data 即 tokenKey。
func (z *ZhhqAuth) exchangeTokenKey(client *CookieClient, userinfo string) (string, error) {
	timestamp, err := ZhhqEncrypt(fmt.Sprintf("%d", time.Now().UnixMilli()), ZhhqResponseKey, ZhhqResponseIV)
	if err != nil {
		return "", err
	}
	// userinfo 先做一次百分号编码再进表单（服务端按两段解码，与 Dart
	// Uri.encodeComponent + http form 编码的双层编码行为一致）。
	form := url.Values{
		"userInfo":   {url.QueryEscape(userinfo)},
		"clientId":   {ZhhqClientID},
		"timestamp":  {timestamp},
		"schoolCode": {"10610"},
		"schoolName": {"四川大学"},
	}
	resp, err := client.PostForm(zhhqLoginAutoURL, map[string]string{
		"Accept":           "application/json, text/plain, */*",
		"Content-Type":     "application/x-www-form-urlencoded; charset=UTF-8",
		"Origin":           "https://zhhq.scu.edu.cn",
		"Referer":          "https://zhhq.scu.edu.cn/account/login",
		"User-Agent":       DefaultUserAgent,
		"X-Requested-With": "XMLHttpRequest",
	}, form)
	if err != nil {
		return "", err
	}
	return z.parseLoginAutoResponse(string(resp.Body), resp.StatusCode)
}

// parseLoginAutoResponse 解析 login/auto 的加密响应，返回 tokenKey。
func (z *ZhhqAuth) parseLoginAutoResponse(body string, statusCode int) (string, error) {
	if statusCode == 302 || statusCode == 401 || statusCode == 403 || strings.TrimSpace(body) == "" {
		return "", &UnauthenticatedError{Msg: "zhhq 登录已失效"}
	}
	json, err := ZhhqDecodeResponse(body)
	if err != nil {
		return "", &UnauthenticatedError{Msg: "zhhq tokenKey 获取失败（响应解密失败）"}
	}
	code := fmt.Sprint(json["errorCode"])
	status := fmt.Sprint(json["status"])
	if code != "0" && code != "" && status == "error" {
		msg, _ := json["message"].(string)
		return "", &UnauthenticatedError{Msg: "zhhq 登录失败: " + msg}
	}
	tokenKey, _ := json["data"].(string)
	if len(tokenKey) < 16 {
		return "", &UnauthenticatedError{Msg: "zhhq tokenKey 获取失败"}
	}
	return tokenKey, nil
}

// Invalidate 清除 tokenKey（业务请求 4010-4017 后由恢复边界调用）。
func (z *ZhhqAuth) Invalidate() {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.tokenKey = ""
	z.lastScuCl = nil
}
