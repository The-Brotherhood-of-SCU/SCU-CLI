---
name: scu-cli
description: 使用 scu CLI 操作四川大学校园服务（教务课表/成绩/考表/教室、电费余额、体测、第二课堂活动报名等）。当用户是四川大学学生/教职工，需要查询或操作校园服务时使用。需要先安装 scu 二进制并完成统一认证登录。
---

# SCU-CLI：四川大学校园服务命令行

`scu` 把四川大学各类校园服务封装成机器可读的命令行。统一认证登录后，各子系统 SSO 由 CLI 自动建立与续期，你只需调用业务命令。

## 1. 安装 CLI

从 GitHub Release 下载对应平台的单个二进制（无需依赖）：

```bash
# 识别平台后下载，例如 Linux amd64：
curl -L -o /usr/local/bin/scu https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases/latest/download/scu-linux-amd64
chmod +x /usr/local/bin/scu

# macOS Apple Silicon：scu-darwin-arm64；macOS Intel：scu-darwin-amd64
# Windows（PowerShell）：scu-windows-amd64.exe
# 全部资产附带 checksums.txt（SHA256），下载后可校验
```

验证：`scu --version`、`scu --help`。

## 2. 登录（统一认证）

```bash
scu login -u <学号> -p <密码>
```

- 验证码由 CLI 内置本地 OCR 自动识别，识别错误自动换新重试（最多 5 次），**不需要人工参与**。
- 登录态持久化在凭据文件，1 小时 TTL，之后任何命令触发过期都会用保存的凭据 + OCR 静默重登，对你透明。
- 密码会出现在 shell 历史/进程列表中；共享环境改用 `SCU_CLI_CONFIG_DIR` 隔离凭据目录。
- 查登录状态：`scu whoami`；退出：`scu logout`。
- 备用（OCR 连续失败时）：`scu captcha --solve` 取验证码，再 `scu login -u <学号> --captcha-code <code> --captcha-text <文本>`。

## 3. 输出约定（必读）

- 成功：`{"ok":true,"data":...}`；失败：`{"ok":false,"error":{"kind","message"}}`，退出码非 0。
- `error.kind == "unauthenticated"` → 引导用户重新 `scu login` 后重试；`"service"` 按 message 处理；**不要**对失败命令盲目重试。
- 诊断信息在 stderr，解析时只读 stdout。

## 4. 参数不用猜：发现链

所有 num/id/code 参数都有无参的根命令逐级发现（`--help` 也标注了来源）：

| 服务 | 链 |
|---|---|
| 课表/班级 | `zhjw semesters` → `value` 即 planCode；`zhjw class options` → `subjects <dept>` → `class list` → `id.executiveEducationPlanNumber` + `id.classNum` |
| 教室 | `zhjw classroom index` → `campuses[].campusNumber` / `buildings[].teachingBuildingNumber` → `types` → `query` |
| 培养方案 | `zhjw program colleges|grades` → `search` → 记录的 `fajhh` → `detail <fajhh>` → treeList 的 `urlPath` → `course` |
| 电费 | `balance campus` → `code`=schoolCode → `buildings` → `code`=regCode → `units` → `code`=unitCode → `query` |
| 第二课堂 | `ccyl activities` → 记录 `id`=activityLibraryId → `lib-detail` → 场次 `id`=activityId → `score-types` → `id`=--score-type |

## 5. 常用命令速查

```bash
scu zhjw week                    # 当前教学周
scu zhjw schedule --plan <code>  # 课表（planCode 来自 zhjw semesters）
scu zhjw grades                  # 及格成绩（--scheme 方案成绩）
scu zhjw exams                   # 考试安排
scu zhjw calendar                # 校历（免登录）
scu user info                    # 用户基本信息（学号/姓名）
scu balance query --type 1       # 照明电费余额（首次需 --school-code 等绑定房间，见发现链）
scu fitness score                # 今年体测成绩
scu ccyl activities --name 讲座  # 搜索第二课堂活动
scu ccyl signup <activityId> --score-type <id>   # 报名活动
```

完整命令树：`scu --help`，子命令细节：`scu <组> <命令> --help`。

## 6. 行为准则

- **写操作**（`ccyl signup/cancel/subscribe/unsubscribe/export`、`balance query` 首次绑房）执行前先向用户确认目标参数；CLI 对写操作不做自动重放，失败时也不要自动重试，先把错误报给用户。
- 只读命令可自由串行调用做信息收集；优先用发现链逐级取 ID，不要向用户索要可以自动发现的编号。
- 凭据含账号密码，不要将凭据文件内容打印或外发。
