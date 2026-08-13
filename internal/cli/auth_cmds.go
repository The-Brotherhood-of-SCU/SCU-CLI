package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/config"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/output"
	"github.com/spf13/cobra"
)

var (
	loginCaptchaCode string
	loginCaptchaText string
)

func init() {
	loginCmd.Flags().StringVar(&loginCaptchaCode, "captcha-code", "", "验证码标识（配合 captcha 命令使用）")
	loginCmd.Flags().StringVar(&loginCaptchaText, "captcha-text", "", "验证码文本（配合 captcha 命令使用）")
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

// newScuAuth 创建带交互式验证码求解器的根认证。
func newScuAuth() (*auth.ScuAuth, error) {
	a, err := auth.NewScuAuth()
	if err != nil {
		return nil, err
	}
	a.SolveCaptcha = interactiveCaptchaSolver
	return a, nil
}

func runLogin(cmd *cobra.Command) error {
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")

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

	captchaCode, captchaText := loginCaptchaCode, loginCaptchaText
	if captchaCode == "" || captchaText == "" {
		captcha, err := auth.FetchCaptcha()
		if err != nil {
			return output.Fail("login", err)
		}
		path, err := saveCaptchaImage(captcha)
		if err != nil {
			return output.Fail("login", err)
		}
		output.Info("验证码已保存到: %s", path)
		captchaText, err = promptLine("请输入验证码")
		if err != nil {
			return output.Fail("input", err)
		}
		captchaCode = captcha.Code
	}

	if err := a.Login(username, password, captchaCode, captchaText); err != nil {
		return output.Fail("login", err)
	}
	output.Info("登录成功: %s", username)
	return output.JSON(map[string]interface{}{
		"principal": username,
		"message":   "登录成功",
	})
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
	return output.JSON(map[string]interface{}{
		"logged_in": true,
		"principal": a.Principal(),
		"expired":   a.Expired(),
		"message":   "已登录（expired 表示本地 TTL 到期，下次请求会自动续期）",
	})
}

// captcha 命令：获取验证码（AI 两步登录的第一步）。
var captchaCmd = &cobra.Command{
	Use:   "captcha",
	Short: "获取统一认证验证码（保存图片并返回标识，供 login --captcha-code/--captcha-text 使用）",
	RunE: func(cmd *cobra.Command, args []string) error {
		captcha, err := auth.FetchCaptcha()
		if err != nil {
			return output.Fail("captcha", err)
		}
		path, err := saveCaptchaImage(captcha)
		if err != nil {
			return output.Fail("captcha", err)
		}
		output.Info("验证码图片: %s", path)
		return output.JSON(map[string]interface{}{
			"captcha_code": captcha.Code,
			"image_path":   path,
			"message":      "识别图片中的验证码文本，然后执行: scu login -u <学号> --captcha-code <code> --captcha-text <文本>",
		})
	},
}
