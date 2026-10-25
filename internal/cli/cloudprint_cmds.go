package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/api"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/auth"
	"github.com/The-Brotherhood-of-SCU/SCU-CLI/internal/output"
	"github.com/spf13/cobra"
)

func newCloudPrintService() (*api.CloudPrintService, error) {
	a, err := newScuAuth()
	if err != nil {
		return nil, err
	}
	return api.NewCloudPrintService(auth.NewCloudPrintAuth(a)), nil
}

// resolveCloudPrintPath 支持 ~/ 路径，并在 WSL 中转换 Windows 盘符路径。
func resolveCloudPrintPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	if runtime.GOOS == "linux" && len(path) > 2 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		if release, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil && strings.Contains(strings.ToLower(string(release)), "microsoft") {
			drive := strings.ToLower(path[:1])
			if drive[0] >= 'a' && drive[0] <= 'z' {
				path = "/mnt/" + drive + "/" + strings.ReplaceAll(path[3:], "\\", "/")
			}
		}
	}
	return filepath.Abs(path)
}

func newCloudPrintCmd() *cobra.Command {
	group := &cobra.Command{Use: "cloudprint", Short: "图书馆云打印（上传文件、查询待打印任务）"}
	group.AddCommand(&cobra.Command{Use: "check", Short: "验证图书馆 SSO 与云打印会话", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func() (interface{}, error) {
				s, err := newCloudPrintService()
				if err != nil {
					return nil, err
				}
				if err := s.Check(); err != nil {
					return nil, err
				}
				return map[string]interface{}{"authenticated": true}, nil
			})
		},
	})
	group.AddCommand(&cobra.Command{Use: "jobs", Aliases: []string{"list"}, Short: "列出云打印任务（szPaperDetail 自动解析为 JSON）", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJSON(func() (interface{}, error) {
				s, err := newCloudPrintService()
				if err != nil {
					return nil, err
				}
				return s.Jobs()
			})
		},
	})
	var options api.CloudPrintOptions
	var filePath string
	var dryRun bool
	upload := &cobra.Command{Use: "upload [file-path]", Short: "上传文件并设置颜色、纸型、双面、份数和页范围",
		Long: `上传文档到图书馆云打印系统，供馆内终端刷卡确认打印。
默认 A4、黑白、长边双面、全部页、1 份；--dry-run 只验证本地文件并输出参数。
打印服务使用 HTTP；上传超时不会自动重发，先用 cloudprint jobs 核对。
WSL 可使用 /mnt/c/... 或引号包裹的 C:\... 文件路径。`,
		Example: "  scu cloudprint upload '/mnt/c/Users/ykx20/Documents/论文.pdf' --duplex long-edge --copies 2 --pages 2-3\n  scu cloudprint upload --file './document.pdf' --color color --paper a3\n  scu cloudprint upload './document.pdf' --dry-run",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filePath
			if len(args) == 1 {
				if cmd.Flags().Changed("file") {
					return output.Fail("input", fmt.Errorf("文件路径不能同时作为位置参数和 --file 提供"))
				}
				path = args[0]
			}
			if path == "" {
				return output.Fail("input", fmt.Errorf("请提供文件路径或 --file"))
			}
			settings, err := options.Prepare()
			if err != nil {
				return output.Fail("input", err)
			}
			path, err = resolveCloudPrintPath(path)
			if err != nil {
				return output.Fail("input", err)
			}
			f, info, err := api.OpenCloudPrintFile(path)
			if err != nil {
				return output.Fail("input", err)
			}
			defer f.Close()
			if dryRun {
				return output.JSON(map[string]interface{}{"dry_run": true, "file_path": path, "file_name": info.Name(), "file_size": info.Size(), "settings": options,
					"endpoint": auth.CloudPrintBase + "/api/client/CloudPrint/Upload", "file_field": "szPath", "fields": settings.Fields(), "back_url_page": auth.CloudPrintBase + auth.CloudPrintJobsPage})
			}
			return runJSON(func() (interface{}, error) {
				s, err := newCloudPrintService()
				if err != nil {
					return nil, err
				}
				return s.Upload(f, info, settings)
			})
		},
	}
	upload.Flags().StringVar(&filePath, "file", "", "待上传文件路径（也可作为位置参数传入）")
	upload.Flags().StringVar(&options.Color, "color", "bw", "颜色：bw 黑白 / color 彩色")
	upload.Flags().StringVar(&options.Paper, "paper", "a4", "纸型：a4 / a3 / original 原文档纸型")
	upload.Flags().StringVar(&options.Duplex, "duplex", "long-edge", "单双面：simplex 单面 / long-edge 长边 / short-edge 短边")
	upload.Flags().IntVar(&options.Copies, "copies", 1, "打印份数（正整数）")
	upload.Flags().StringVar(&options.Pages, "pages", "all", "页范围：all 全部 / 2 单页 / 2-3 连续范围")
	upload.Flags().BoolVar(&dryRun, "dry-run", false, "只验证文件并输出设置，不登录、不上传")
	group.AddCommand(upload)
	return group
}

func init() { rootCmd.AddCommand(newCloudPrintCmd()) }
