package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/emmansun/gmsm/sm2"
)

// sm2C1C2C3Opts 与 Bugaoshan 的 dart_sm C1C2C3 模式对应：
// 输出为 04||C1||C2||C3 的原始拼接（非 ASN.1）。
var sm2C1C2C3Opts = sm2.NewPlainEncrypterOpts(sm2.MarshalUncompressed, sm2.C1C2C3)

// SM2EncryptWithBase64Key 用服务端返回的 base64 公钥加密明文，
// 返回 base64(04||C1||C2||C3) 密文。
//
// 公钥为未压缩点字节（可省略 04 前缀）。
func SM2EncryptWithBase64Key(plaintext, publicKeyBase64 string) (string, error) {
	pubBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return "", fmt.Errorf("SM2 公钥 base64 解码失败: %w", err)
	}
	if len(pubBytes) == 64 {
		// 缺少 04 前缀，补上未压缩点标记。
		pubBytes = append([]byte{0x04}, pubBytes...)
	}
	x, y := elliptic.Unmarshal(sm2.P256(), pubBytes)
	if x == nil {
		return "", fmt.Errorf("SM2 公钥点解析失败")
	}
	pub := &ecdsa.PublicKey{Curve: sm2.P256(), X: x, Y: y}
	cipher, err := sm2.Encrypt(rand.Reader, pub, []byte(plaintext), sm2C1C2C3Opts)
	if err != nil {
		return "", fmt.Errorf("SM2 加密失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(cipher), nil
}
