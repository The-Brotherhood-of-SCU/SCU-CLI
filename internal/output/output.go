// Package output 统一 CLI 的输出格式，方便 AI 或其他程序消费。
//
// 默认输出带包层的 JSON：
//
//	{"ok": true, "data": ...}
//	{"ok": false, "error": {"kind": "...", "message": "..."}}
//
// 所有业务数据写入 stdout；诊断、提示信息写入 stderr。
package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// Envelope 是统一输出包层。
type Envelope struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error *ErrInfo    `json:"error,omitempty"`
}

// ErrInfo 描述结构化错误。
type ErrInfo struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// JSON 将 data 以成功包层输出到 stdout。
func JSON(data interface{}) error {
	return write(Envelope{OK: true, Data: data})
}

// Fail 将错误以失败包层输出到 stdout（保持机器可读），并返回非零退出语义的 error。
func Fail(kind string, err error) error {
	_ = write(Envelope{OK: false, Error: &ErrInfo{Kind: kind, Message: err.Error()}})
	return err
}

func write(e Envelope) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return fmt.Errorf("输出 JSON 失败: %w", err)
	}
	return nil
}

// Info 向 stderr 打印面向人类的提示信息。
func Info(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
