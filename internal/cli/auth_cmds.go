package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/config"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/ocr"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	loginCaptchaCode string
	loginCaptchaText string
)

func init() {
	loginCmd.Flags().StringVar(&loginCaptchaCode, "captcha-code", "", "验证码标识（配合 captcha 命令使用）")
	loginCmd.Flags().StringVar(&loginCaptchaText, "captcha-text", "", "验证码文本（配合 captcha 命令使用）")
	loginCmd.Flags().Bool("no-ocr", false, "禁用本地 OCR，人工识别验证码")
	captchaCmd.Flags().Bool("solve", false, "同时输出本地 OCR 识别结果")
}

// saveCaptchaImage 解码验证码图片并写入配置目录，返回文件路径。
func saveCaptchaImage(c *auth.Captcha) (string, error) {
	raw := c.ImageBase64
	if i := strings.Index(raw, ","); i >= 0 {
		raw = raw[i+1:]
	}
	img, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("验证码图片解码失败: %w", err)
	}
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "captcha.png")
	if err := os.WriteFile(path, img, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// interactiveCaptchaSolver 保存验证码图片并提示用户输入。
func interactiveCaptchaSolver(c *auth.Captcha) (string, error) {
	path, err := saveCaptchaImage(c)
	if err != nil {
		return "", err
	}
	output.Info("验证码已保存到: %s（请打开查看）", path)
	return promptLine("请输入验证码")
}

// solveCaptchaWithOCR 优先本地 OCR，失败时在 TTY 下回退人工识别。
func solveCaptchaWithOCR(c *auth.Captcha) (string, error) {
	if text, err := ocr.RecognizeBase64(c.ImageBase64); err == nil && text != "" {
		return text, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("OCR 识别失败（低置信度），且当前非交互终端")
	}
	return interactiveCaptchaSolver(c)
}

// newScuAuth 创建带 OCR 验证码求解器的根认证。
func newScuAuth() (*auth.ScuAuth, error) {
	a, err := auth.NewScuAuth()
	if err != nil {
		return nil, err
	}
	a.SolveCaptcha = solveCaptchaWithOCR
	return a, nil
}

func runLogin(cmd *cobra.Command) error {
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	noOCR, _ := cmd.Flags().GetBool("no-ocr")

	a, err := newScuAuth()
	if err != nil {
		return output.Fail("config", err)
	}

	if username == "" {
		if username, err = promptLine("学号"); err != nil {
			return output.Fail("input", err)
		}
	}
	if password == "" {
		if password, err = promptPassword(); err != nil {
			return output.Fail("input", err)
		}
	}

	// 模式一：调用方已提供验证码（AI 两步登录）。
	if (loginCaptchaCode != "") != (loginCaptchaText != "") {
		return output.Fail("input", errors.New("--captcha-code 与 --captcha-text 必须同时提供"))
	}
	if loginCaptchaCode != "" && loginCaptchaText != "" {
		if err := a.Login(username, password, loginCaptchaCode, loginCaptchaText); err != nil {
			return output.Fail("login", err)
		}
		output.Info("登录成功: %s", username)
		return output.JSON(map[string]interface{}{"principal": username, "message": "登录成功"})
	}

	// 模式二：自动获取验证码 + OCR 识别；invalid_captcha 时换新验证码重试。
	// --no-ocr 强制人工识别。
	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		captcha, err := auth.FetchCaptcha()
		if err != nil {
			return output.Fail("login", err)
		}
		var text string
		if !noOCR {
			text, err = solveCaptchaWithOCR(captcha)
		} else {
			path, serr := saveCaptchaImage(captcha)
			if serr != nil {
				return output.Fail("login", serr)
			}
			output.Info("验证码已保存到: %s", path)
			text, err = promptLine("请输入验证码")
		}
		if err != nil {
			return output.Fail("captcha", err)
		}
		err = a.Login(username, password, captcha.Code, text)
		if err == nil {
			output.Info("登录成功: %s", username)
			return output.JSON(map[string]interface{}{"principal": username, "message": "登录成功"})
		}
		var le *auth.LoginError
		if errors.As(err, &le) && le.InvalidCaptcha && attempt < maxAttempts {
			output.Info("验证码识别错误，换一张重试 (%d/%d)", attempt, maxAttempts)
			continue
		}
		return output.Fail("login", err)
	}
	return output.Fail("login", errors.New("登录失败：验证码多次识别错误"))
}

func runLogout(cmd *cobra.Command) error {
	a, err := newScuAuth()
	if err != nil {
		return output.Fail("config", err)
	}
	if err := a.Logout(); err != nil {
		return output.Fail("logout", err)
	}
	output.Info("已退出登录")
	return output.JSON(map[string]interface{}{"message": "已退出登录"})
}

func runWhoami(cmd *cobra.Command) error {
	a, err := newScuAuth()
	if err != nil {
		return output.Fail("config", err)
	}
	if !a.LoggedIn() {
		return output.JSON(map[string]interface{}{
			"logged_in": false,
			"message":   "未登录，请先执行 scu login",
		})
	}
	// login_time / expires_in_seconds 供调用方判断凭据新鲜度；
	// 本地 TTL 到期不影响使用（请求时会自动续期）。
	out := map[string]interface{}{
		"logged_in": true,
		"principal": a.Principal(),
		"expired":   a.Expired(),
		"message":   "已登录（expired 表示本地 TTL 到期，下次请求会自动续期）",
	}
	if lt := a.LoginTime(); !lt.IsZero() {
		out["login_time"] = lt.Format(time.RFC3339)
		out["expires_in_seconds"] = int(a.ExpiresIn().Seconds())
	}
	return output.JSON(out)
}

// captcha 命令：获取验证码（AI 两步登录的第一步）。
var captchaCmd = &cobra.Command{
	Use:   "captcha",
	Short: "获取统一认证验证码（保存图片并返回标识；--solve 同时输出本地 OCR 结果）",
	RunE: func(cmd *cobra.Command, args []string) error {
		solve, _ := cmd.Flags().GetBool("solve")
		captcha, err := auth.FetchCaptcha()
		if err != nil {
			return output.Fail("captcha", err)
		}
		path, err := saveCaptchaImage(captcha)
		if err != nil {
			return output.Fail("captcha", err)
		}
		output.Info("验证码图片: %s", path)
		data := map[string]interface{}{
			"captcha_code": captcha.Code,
			"image_path":   path,
		}
		if solve {
			text, _ := ocr.RecognizeBase64(captcha.ImageBase64)
			if text == "" {
				data["message"] = "OCR 置信度不足，请人工识别图片文本"
			} else {
				data["captcha_text"] = text
				data["message"] = "本地 OCR 已识别，可直接用于 scu login --captcha-code/--captcha-text"
			}
		} else {
			data["message"] = "识别图片中的验证码文本（或用 --solve 本地 OCR），然后执行: scu login -u <学号> --captcha-code <code> --captcha-text <文本>"
		}
		return output.JSON(data)
	},
}
