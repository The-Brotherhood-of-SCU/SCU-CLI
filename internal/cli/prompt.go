package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// ErrNonInteractive 在需要交互输入、但 stdin 不是交互终端时返回。
//
// 面向 AI/脚本调用：任何交互 prompt 在非 TTY 环境下必须立即失败，
// 绝不能阻塞等待输入（AI 执行 bash 时会被挂住）。
var ErrNonInteractive = errors.New("需要交互输入，但 stdin 不是交互终端。" +
	"请改用非交互方式：scu login -u <学号> -p <密码>（验证码由内置 OCR 自动识别）")

// stdinIsTerminal 报告 stdin 是否为交互终端。
func stdinIsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// stdinReader 是全进程共享的 stdin 读取器。promptLine 每次新建
// bufio.Reader 会把上一调用预读进缓冲的输入丢弃（粘贴多行时丢第二行）。
var stdinReader = bufio.NewReader(os.Stdin)

// promptLine 从 stdin 读取一行文本。仅在交互终端下可用。
func promptLine(label string) (string, error) {
	if !stdinIsTerminal() {
		return "", ErrNonInteractive
	}
	fmt.Fprintf(os.Stderr, "%s: ", label)
	line, err := stdinReader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// promptPassword 读取密码（不回显）。仅在交互终端下可用。
func promptPassword() (string, error) {
	if !stdinIsTerminal() {
		return "", ErrNonInteractive
	}
	fmt.Fprint(os.Stderr, "密码: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(pw), nil
}
