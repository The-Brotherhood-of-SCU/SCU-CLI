# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

`scu` 是四川大学校园服务 CLI（Go 1.26，cobra），面向 AI agent 与脚本设计。协议层移植自 Dart 应用 [Bugaoshan](https://github.com/The-Brotherhood-of-SCU/Bugaoshan)，验证码 OCR 移植自 [scu-plus](https://github.com/The-Brotherhood-of-SCU/scu-plus)——修改认证/协议代码时注意与上游保持语义一致（代码注释常以"与 Bugaoshan 一致"标注对应关系）。完整命令文档见 README.md 与 `skill/scu-cli/SKILL.md`。

## 常用命令

```bash
go build ./cmd/scu          # 开发构建（Version 为 "dev"）
go test ./...               # 全部测试（纯单元测试，无网络；cookie_client_test 用 httptest）
go test ./internal/api/ -run TestCalculateBalanceTrend   # 单个测试
go vet ./...                # 无 lint 配置，vet 即可
```

发布不手动执行：推送 `vX.Y.Z` 标签触发 `.github/workflows/release.yml`，构建 6 平台二进制（`-ldflags -X .../internal/cli.Version=<tag>` 注入版本）→ GitHub Release → npm（OIDC Trusted Publishing）→ SkillHub。**CI 会从 CHANGELOG.md 提取 `## [X.Y.Z]` 章节作为 Release 说明，缺失该章节则发布失败**——发版前必须先写 CHANGELOG。

## 输出契约（所有命令必须遵守）

- stdout 恒为单个 JSON 包层：`{"ok":true,"data":...}` 或 `{"ok":false,"error":{"kind","message"}}`，由 `internal/output` 统一写入；诊断/提示一律走 stderr（`output.Info`）。
- 失败时返回非零退出码。新命令用 `runJSON` 包裹（`internal/cli/zhjw_cmds.go`），公共分类器在 `internal/cli/errors.go`：上传结果不确定优先映射 `upload_uncertain`，然后区分 `unauthenticated`、`config` 与 `service`。
- 编号类参数（planCode、楼栋 code 等）不靠猜，由"发现链"命令逐级输出；新命令的 `--help` 需标明参数来源命令。

## 架构：分层与认证模型

```
cmd/scu → internal/cli（cobra 命令，每命令组一个文件）
        → internal/api（L1 业务 API，无认证状态，构造时接收 SubsystemAuth）
        → internal/auth（L1 根认证 ScuAuth + L2 子系统认证）
        → CookieClient（共享传输层）
```

- **ScuAuth 是认证单一事实来源**（`internal/auth/scu.go`）：持有持久化凭据（`internal/config`，`credentials.json` 0600 原子写入），token 本地 TTL 1 小时，过期用保存的账号密码 + 验证码 OCR 自动重登。密码用 SM2 加密（`sm2.go`，C1C2C3 原始拼接模式）。
- **子系统认证（L2）实现 `SubsystemAuth` 接口**（`internal/auth/subsystem.go`）：`ZhjwAuth`（JWT SSO）、`WfwAuth`（CAS 预热，就绪判据是响应为 `e==0` 的 JSON，匿名 200 不算）、`SsoRelayAuth`（缴费/体测/办事大厅/newservice 通用的 SSO 中继，可声明依赖其他子系统先就绪）、`ZhhqAuth`（智慧后勤：SSO 换 tokenKey + 每请求 AES 加密 Token 头，见 `zhhq.go`）、`CcylAuth`（第二课堂独立 OAuth token，与 SCU principal 绑定持久化）。
- 子系统 client 有缓存，**根 client 身份变化时缓存自动作废**；`RetryOnUnauthenticated` 是统一恢复边界——认证失效时 invalidate + 重建 session，业务请求只重放一次。
- **云打印认证**（`internal/auth/cloudprint.go`）：`SSoPage` 返回 JSON 登录链接，需解析 `result`，用根 cookie jar 跟随 libic/CAS 回跳，最后以 `Auth/Check code=0` 且非空 `SessionId` 验证就绪。逐跳校验打印 IP、libic 和统一认证的允许 origin；SCU Bearer 与密码不能发送到 HTTP 打印 IP。
- **CookieClient 的铁律**（`internal/auth/cookie_client.go`）：重定向必须手动逐跳跟随，重定向策略必须返回 `http.ErrUseLastResponse`（不能用 `resty.NoRedirectPolicy()`，否则 3xx 被包装成 `*url.Error` 上抛）；跨源跳转剥离 `Authorization`/`Cookie` 等敏感 header，除非 `AllowSensitiveOrigin` 显式允许。教务 `zhjw.scu.edu.cn` 仅支持 HTTP。
- 错误分类在 `internal/auth/errors.go`：`UnauthenticatedError`（触发重试）、`ServiceError`（不重试）、`LoginError`（`InvalidCaptcha` 时换验证码重试）、`CcylAuthExpiredError`（CCYL 边界重试一次）。

## 其他关键点

- `internal/ocr`：验证码本地 OCR，`//go:embed model_centroid.json` 质心模型，49 维特征（48 维 8×6 二值像素 + 宽高比）。登录流程：OCR 失败换新验证码重试（最多 5 次），非 TTY 下 OCR 失败直接报错而非阻塞。
- `internal/config`：除凭据外还存电费余额趋势快照（`balance_history.json`，按房间归集保留 365 天）。配置目录可被 `SCU_CLI_CONFIG_DIR` 覆盖——**测试必须用 `t.Setenv("SCU_CLI_CONFIG_DIR", t.TempDir())` 隔离**，避免碰真实凭据。
- `internal/ics`：ICS 日历导出，校区节次时段表自动探测（`campus.go`）。
- `internal/api/cloudprint.go` / `internal/cli/cloudprint_cmds.go`：`cloudprint upload <file-path>`（或 `--file`）、`check`、`jobs`/`list`、离线 `--dry-run`。默认 **A4、黑白、长边双面、全部页、1 份**；短边/长边映射分别为 3/2。
- **云打印文件 POST 不得自动重放**：使用流式 `CookieClient.DoOnce` 和已知 Content-Length，不能改为普通 `Do` 或将整个上传包在 `RetryOnUnauthenticated` 中。认证/读请求可在提交前恢复；提交后的连接异常、307/308、缺失结果码、HTTP 401/403 或回跳 code=4 均返回 `upload_uncertain`，先核对任务，不能返回引导重传的 `unauthenticated`。仅对 301/302/303 跟随受限同源 GET；HTTP 413 明确拒绝仍为 `service`。
- 云打印选项只在入口处调用 `CloudPrintOptions.Prepare` 校验一次；`CloudPrintSettings` 保存已校验设置，dry-run 读取字段副本，上传复用同一份设置。
- 云打印中文文件名按浏览器 FormData 使用带引号 UTF-8 `filename=`；真实服务器拒绝 RFC 5987 `filename*=`。任务输出过滤姓名、账号、卡号、服务器文件路径，`szPaperDetail` 二次 JSON 解析；`fee_raw` 单位未确认，上传不代表实际打印或付款。PDF CLI 实测记录见 `notes/cloudprint/2026-10-11/cli-verification.md`。
- npm 分发：`install.js`（postinstall）按平台从 GitHub Release 下载二进制（SHA256 校验）并安装 Skill 到 `~/.claude/skills/scu-cli/`（`SCU_CLI_NO_SKILL=1` 跳过）；`bin/scu.js` 是转发 shim。**package.json 的 version 必须对应已存在的 Release 标签**（CI 发布时用标签值覆盖）。
