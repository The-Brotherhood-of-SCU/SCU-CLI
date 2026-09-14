package api

import (
	"testing"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
)

func TestDecodeNewServiceResponse(t *testing.T) {
	out, err := decodeNewServiceResponse(`{"e":"OK","d":{"errorCode":0,"data":[]}}`, 200)
	if err != nil {
		t.Fatalf("成功响应: %v", err)
	}
	d, err := checkNewServiceData(out)
	if err != nil {
		t.Fatalf("业务校验: %v", err)
	}
	if _, ok := d["data"]; !ok {
		t.Fatalf("d = %v", d)
	}
}

func TestDecodeNewServiceResponseUnauth(t *testing.T) {
	if _, err := decodeNewServiceResponse("", 302); !auth.IsUnauthenticated(err) {
		t.Fatalf("302 应为 unauthenticated: %v", err)
	}
	if _, err := decodeNewServiceResponse(`{"e":"UN_AUTH","m":"会话失效"}`, 200); !auth.IsUnauthenticated(err) {
		t.Fatalf("UN_AUTH 应为 unauthenticated: %v", err)
	}
	// 登录页强特征（issue #282：不做裸 login 子串匹配）。
	if _, err := decodeNewServiceResponse(`<html><body>please login here</body></html>`, 200); !auth.IsUnauthenticated(err) {
		t.Fatalf("登录页应为 unauthenticated: %v", err)
	}
}

func TestDecodeNewServiceResponseBusinessError(t *testing.T) {
	_, err := decodeNewServiceResponse(`{"e":"FAIL","m":"系统繁忙"}`, 200)
	if err == nil || err.Error() != "系统繁忙" {
		t.Fatalf("业务错误 = %v", err)
	}
	out, _ := decodeNewServiceResponse(`{"e":"OK","d":{"errorCode":1001,"errorMessage":"MAC 已绑定"}}`, 200)
	_, err = checkNewServiceData(out)
	if err == nil || err.Error() != "MAC 已绑定" {
		t.Fatalf("d 内业务错误 = %v", err)
	}
}

func TestParsePasspointDevice(t *testing.T) {
	dev := parsePasspointDevice(map[string]interface{}{
		"userMac":            "AA:BB:CC:DD:EE:FF",
		"macExpireTime":      "2026-09-02",
		"defaultServiceName": "中国电信",
		"isOnline":           true,
	})
	if !dev.IsOnline || dev.MacExpireTime != "2026-09-02" || dev.UserMac != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("device = %+v", dev)
	}

	// 空/非法到期时间（"最长有效期 6 年"场景）→ 空串。
	dev = parsePasspointDevice(map[string]interface{}{"userMac": "11:22:33:44:55:66", "macExpireTime": "", "isOnline": "1"})
	if dev.MacExpireTime != "" || !dev.IsOnline {
		t.Fatalf("device = %+v", dev)
	}
	dev = parsePasspointDevice(map[string]interface{}{"userMac": "x", "macExpireTime": "garbage", "isOnline": "false"})
	if dev.MacExpireTime != "" || dev.IsOnline {
		t.Fatalf("device = %+v", dev)
	}
}
