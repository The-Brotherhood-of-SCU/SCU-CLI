---
name: scu-cli
description: 使用 scu CLI 操作四川大学校园服务（教务课表/成绩/考表/教室/培养方案/校历、电费余额、体测成绩、第二课堂活动/报名/学分等）。当用户是四川大学学生或教职工，需要查询或操作校园服务时使用。需要先安装 scu 二进制并完成统一认证登录。
slug: scu-cli
displayName: 四川大学校园服务 CLI
version: 0.4.1
summary: 四川大学校园服务命令行 Skill——教务课表/成绩/考表/校历（支持 ICS 导出）、电费余额与消耗趋势、体测成绩、第二课堂活动/报名/学分、办事大厅事项查询与动态表单提交。
license: AGPL-3.0-only
tags:
  - scu
  - 四川大学
  - 校园服务
  - cli
homepage: https://github.com/The-Brotherhood-of-SCU/SCU-CLI
---

# SCU-CLI：四川大学校园服务命令行

`scu` 把四川大学各类校园服务封装成机器可读的命令行，面向 AI 与脚本设计。统一认证登录后，教务/微服务/缴费/体测/第二课堂的子系统 SSO 由 CLI 自动建立，会话过期自动重新认证并重放请求一次——这些对你完全透明，只需调用业务命令。

## 1. 安装 CLI

先探测：`scu --version` 能跑通则跳过本节。

**方式 A — npm（首选，一条命令）**：

```bash
npm install -g @the-brotherhood-of-scu/scu-cli
```

需要 Node.js ≥ 18。自动按平台/架构（linux / darwin / windows × amd64 / arm64）下载二进制（SHA256 校验），并把本 Skill 安装到 `~/.claude/skills/scu-cli/`（用户设 `SCU_CLI_NO_SKILL=1` 时会跳过）。

**方式 B — GitHub Release 二进制**（无 Node 环境时；资产附带 `checksums.txt` 可校验 SHA256）：

| 平台 | 资产 |
|---|---|
| Linux amd64 / arm64 | `scu-linux-amd64` / `scu-linux-arm64` |
| macOS Intel / Apple Silicon | `scu-darwin-amd64` / `scu-darwin-arm64` |
| Windows amd64 / arm64 | `scu-windows-amd64.exe` / `scu-windows-arm64.exe` |

```bash
# Linux / macOS（以 linux-amd64 为例）
curl -L -o scu https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases/latest/download/scu-linux-amd64
chmod +x scu && sudo mv scu /usr/local/bin/
```

```powershell
# Windows PowerShell
Invoke-WebRequest -Uri https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases/latest/download/scu-windows-amd64.exe -OutFile scu.exe
```

**方式 C**：`go install github.com/The-Brotherhood-of-SCU/SCU-CLI/cmd/scu@latest`

验证：`scu --version`、`scu --help`。

## 2. 登录（统一认证）

```bash
scu login -u <学号> -p <密码>
```

- 验证码由 CLI **内置本地 OCR 自动识别**，识别错误自动换新验证码重试（最多 5 次），不需要人工参与。
- 登录态持久化在凭据文件（Linux/macOS `~/.config/scu-cli/`，Windows `%APPDATA%\scu-cli\`，0600 权限；可用 `SCU_CLI_CONFIG_DIR` 覆盖），本地 1 小时 TTL；过期后任何命令都会用保存的凭据 + OCR 静默重新登录，对你透明。
- `-p` 会进入 shell 历史/进程列表；共享环境建议隔离 `SCU_CLI_CONFIG_DIR`。
- 查状态：`scu whoami`（返回 principal 与本地 TTL 是否到期）；退出：`scu logout`（清除凭据）。
- **非 TTY 保证**：所有交互式输入（学号/密码/人工验证码）仅在 stdin 是终端时才会发起，非 TTY 下立即报错退出、绝不阻塞——所以调用时必须把参数带齐。
- 备用（OCR 连续失败时）：`scu captcha --solve` 取验证码（输出 `captcha_code` + OCR 结果 + 图片路径），再 `scu login -u <学号> --captcha-code <code> --captcha-text <文本>`。

## 3. 输出约定（必读）

所有命令向 stdout 输出 JSON 包层，诊断信息走 stderr（解析时只读 stdout）：

```json
{"ok": true, "data": { ... }}
{"ok": false, "error": {"kind": "unauthenticated", "message": "..."}}
```

失败时退出码非 0。`error.kind` 处理建议：

| kind | 含义 | 处理 |
|---|---|---|
| `unauthenticated` | 未登录或会话失效且自动恢复失败 | 引导用户重新 `scu login` 后重试 |
| `service` | 服务端业务错误 / 网络错误 | 按 message 判断，不要盲目重试 |
| `login` | 登录阶段错误（验证码/密码错误等） | 修正后重试 |
| `config` / `input` | 本地配置或输入错误 | 检查输入 |

## 4. 参数发现链（所有 num/id/code 都不用猜）

每个编号参数都有无参的根命令逐级发现；每条命令的 `--help` 也标注了参数来源：

| 参数 | 来源 |
|---|---|
| `planCode` | `scu zhjw semesters` 输出的 `value`（如 2025-2026-2-1） |
| 校区/教学楼编号与名称 | `scu zhjw classroom index` → `campuses[].campusNumber/campusName`、`buildings[].teachingBuildingNumber/teachingBuildingName` |
| 学院/年级代码 | `scu zhjw program colleges` / `grades` 输出的 `value` |
| `fajhh`（培养方案号） | `scu zhjw program search` 输出记录 |
| `urlPath`（课程详情路径） | `scu zhjw program detail` 输出 `treeList` 节点 |
| 院系/专业/班级编号 | `scu zhjw class options` → `subjects <dept>` → `class list`（记录含 `id.executiveEducationPlanNumber`、`id.classNum`） |
| `schoolCode`/`regCode`/`unitCode` | `scu balance campus` → `buildings` → `units`，每级输出 `[{name, code}]`，上一级 `code` 是下一级入参 |
| `activityLibraryId` | `scu ccyl activities` 输出记录的 `id` |
| `activityId` | `scu ccyl lib-detail <activityLibraryId>` 输出中各场次活动的 `id` |
| `--score-type` | `scu ccyl score-types <activityLibraryId>` 输出的能力类型 `id` |
| `creditId` | `scu ccyl credits` 输出记录 |

## 5. 教务系统 `scu zhjw`

```bash
scu zhjw week                          # 当前教学周（含 on_vacation）
scu zhjw semesters                     # 学期列表（value 为 planCode，如 2025-2026-2-1）
scu zhjw schedule --plan 2025-2026-2-1 # 课表（原始 JSON）
scu zhjw grades                        # 及格成绩
scu zhjw grades --scheme               # 方案成绩
scu zhjw exams                         # 考表（考试安排）
scu zhjw completion                    # 计划完成度
scu zhjw calendar                      # 校历（免认证，网络优先、失败回退本地缓存）

# ICS 日历导出（--ics 必须带文件路径值；输出 JSON 含 file/events 等元信息）
scu zhjw schedule --plan 2025-2026-2-1 --ics schedule.ics
#   学期起始日自动从校历匹配（planCode → 校历学期名）；匹配失败时用 --start-date 2026-03-02 手动指定
#   节次时段按课程地点自动探测校区（江安/望江/华西），也可用 --campus 江安 强制
scu zhjw exams --ics exams.ics         # 考表导出（日期时间无法解析的考试自动跳过，见 skipped_unparseable）
scu zhjw calendar --ics calendar.ics   # 校历导出（全天事件，开学/假期/考试周）

# 教室查询（编号链：index → types → query）
scu zhjw classroom index               # 校区与教学楼列表（所有编号的来源）
scu zhjw classroom types --campus-num 1 --building-num 101 --campus-name 江安 --building-name 一教A
scu zhjw classroom query --campus-num 1 --building-num 101 --date 2026-08-14
#   query 可选：--type 教室类型代码（来自 types）--name 教室名 --seat-from/--seat-to 座位数区间

# 培养方案（编号链：colleges/grades → search → detail → course）
scu zhjw program colleges              # 学院列表（value = --college）
scu zhjw program grades                # 年级列表（value = --grade）
scu zhjw program search --college 301 --grade 2023   # 记录含 fajhh
scu zhjw program detail <fajhh>        # 方案详情（treeList 节点含 urlPath）
scu zhjw program course <urlPath>      # 课程详情

# 班级课表（编号链：options → subjects → list → schedule）
scu zhjw class options                 # 筛选项（semesters/grades/departments 的 value）
scu zhjw class subjects <departmentNum>                # 专业列表（含 subjectCode）
scu zhjw class list --plan 2025-2026-2-1 --dept 301    # 记录含 id.executiveEducationPlanNumber / id.classNum
scu zhjw class schedule <planCode> <classCode>         # 即上一步输出的两个字段
```

## 6. 用户信息 `scu user`（微服务）

```bash
scu user info       # 基本信息（realname 姓名、role.number 学号等）
scu user labels     # 用户标签
scu user devices    # 校园网在线设备（输出 device_id / ip）
scu user offline --device-id <id> --ip <ip>   # 强制指定设备下线（参数取自 devices 输出）
```

## 7. 缴费平台 `scu balance`

房间参数逐级发现（各级输出均为 `[{name, code}]`，上一级的 `code` 是下一级的入参）：

```bash
scu balance campus                    # 校区列表，code = schoolCode
scu balance buildings <schoolCode>    # 楼栋列表，code = regCode
scu balance units <schoolCode> <regCode>  # 单元列表，code = unitCode

# 查询余额（--type 1 照明电费 / 2 空调电费；学号与姓名自动取自 user info）
# 首次查询需绑定房间：
scu balance query --type 1 --school-code 1 --reg-code 101 --unit-code 1 --room 101
# 之后直接查询已绑定房间：
scu balance query --type 1
scu balance query --type 2            # 空调电费

# 余额趋势：每次 query 成功自动记录本地快照（按房间+类型归集，保留 365 天）
scu balance trend --type 1            # 日均电费/日均度数/累计消耗 + daily_points 序列
scu balance trend --type 1 --days 30  # 只统计最近 30 天
```

trend 说明：快照按北京日历日聚合（每日取最后一条），充值段（余额上升）自动跳过并计入
`skipped_recharge_segments`；`record_count` 为 0 时表示还没积累快照（用 note 字段的话术回复用户，
过几天查询后再看）。

## 8. 体测系统 `scu fitness`

```bash
scu fitness notices        # 体测通知
scu fitness score          # 今年体测成绩
scu fitness score --year 2025
```

## 9. 第二课堂 `scu ccyl`

ID 发现链：`activities`（→ `activityLibraryId`）→ `lib-detail`（→ `activityId`）→ `score-types`（→ `--score-type`）；`credits`（→ `creditId`）。

```bash
scu ccyl activities --name 讲座 --page 1 --size 10   # 搜索活动库，记录的 id = activityLibraryId
#   可选筛选：--level --score-type --org --order --status --quality
scu ccyl mine                                        # 我参与的活动（--page/--size）
scu ccyl subscribed                                  # 已预约的活动系列（--name/--page/--size）
scu ccyl dicts <groupCode...>                        # 数据字典（activities 筛选参数的合法枚举来源，如 level/quality）
scu ccyl orgs                                        # 组织列表
scu ccyl lib-detail <activityLibraryId>              # 活动系列详情，含场次活动 id = activityId
scu ccyl detail <activityId>                         # 活动详情
scu ccyl score-types <activityLibraryId>             # 能力类型，id = signup --score-type
scu ccyl signup <activityId> --score-type <id>       # 报名
scu ccyl cancel <activityId>                         # 取消报名
scu ccyl subscribe <activityLibraryId>               # 预约活动系列
scu ccyl unsubscribe <activityLibraryId>             # 取消预约
scu ccyl credits                                     # 成绩单（记录含 creditId，--page/--size）
scu ccyl export <email> <creditId...>                # 导出成绩单到邮箱
```

完整命令树：`scu --help`；子命令细节与参数来源：`scu <组> <命令> --help`。

## 10. 办事大厅 `scu service`

```bash
scu service applications                    # 我的申请列表（--status 0全部/1进行中+草稿/3已完成，--page 分页，每页20条）
#   列表项：app_name 事项名、created 提交时间、inst_status 中文状态文案（不要自己映射 status 数字）
```

事项办理（动态表单，三步）：

```bash
# ① 查看表单结构：fields 按填写顺序给出 key/label/type/required/options/prefilled，
#    date_rules 为日期顺序校验（first_key 必须早于等于 second_key）
scu service form 350

# ② 组装字段并预览提交体（强烈建议先 dry-run 给用户确认）
scu service submit 350 --fields '{"Radio_30":"1","Input_31":"成都市","Calendar_40":"2026-08-14"}' --dry-run

# ③ 确认后正式提交（去掉 --dry-run）
scu service submit 350 --fields '{"Radio_30":"1","Input_31":"成都市","Calendar_40":"2026-08-14"}'
```

`--fields` 的值按 form 输出的 type 给：

| type | 值形态 |
|---|---|
| input / multiInput / dataSource | 字符串 |
| radio / select / selectV2 | options 里的 `value` 字符串 |
| checkbox | value 字符串数组 `["1","2"]` |
| calendar | `"2026-08-14"` 或 `"2026-08-14 10:00"` |
| region | `{"province":"四川省","city":"成都市","area":"武侯区","details":"详细地址"}`（省/市/区给名称或 6 位编码，直辖市可省略 city） |
| file | 不走 --fields，用 `--attach File_71=./照片.png`（可重复，上限见 max_count） |

注意：
- 只读/隐藏/user 字段由服务端透传，不要出现在 --fields 里（会报错）。
- dataSource 字段给出名称后 CLI 自动取数联动（如导师姓名 → 回填工号/日期等配对字段），取数失败降级为仅提交名称并在 warnings 中说明。
- 选项 value 合法性、必填、ShowHide 动态显隐/动态必填、日期顺序都在本地校验，报错即修正后重试。
- 输出的 warnings 非空时要如实转告用户。

## 11. 行为准则

- **写操作先确认**：`ccyl signup/cancel/subscribe/unsubscribe/export`、`user offline`、`service submit`（先 --dry-run 给用户看提交体）、`balance query` 首次绑房等写操作，执行前向用户确认目标与参数；失败后不要自动重试（CLI 自身也不会重放写请求），先把错误报给用户。
- 只读命令可自由串行调用；优先用发现链逐级取 ID，不要向用户索要可以自动发现的编号。
- 凭据文件含账号密码，不要打印、复制或外发其内容。
- `unauthenticated` 错误一律引导重新登录，不要尝试其他绕过方式。
