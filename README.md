# SCU-CLI

四川大学校园服务命令行工具，面向 **AI 代理与脚本**设计。通过统一身份认证登录后，可操作教务系统、微服务、缴费平台、体测系统、第二课堂等校园服务。

协议与认证架构移植自校园生活助手 App [Bugaoshan](https://github.com/The-Brotherhood-of-SCU/Bugaoshan)。

## 特性

- **统一认证**：SM2 加密密码登录、**内置本地 OCR 识别验证码**（零网络推理）、token 持久化、1 小时 TTL 自动续期
- **子系统 SSO 自动建立**：教务 JWT SSO、微服务 CAS 重定向链、缴费/体测 SSO 中继、第二课堂 OAuth
- **会话过期自愈**：识别各后端过期信号，自动重新认证并重放业务请求一次
- **机器可读输出**：所有命令输出统一 JSON 包层到 stdout，诊断信息走 stderr

## 安装

### npm 一键安装（推荐，面向 AI）

```bash
npm install -g @the-brotherhood-of-scu/scu-cli   # 需要 Node.js ≥ 18
```

一条命令完成两件事：

1. 按当前平台/架构（linux / darwin / windows × amd64 / arm64）自动下载预编译二进制并做 SHA256 校验（二进制托管在 GitHub Release，npm 包本身只有安装器）
2. 自动把 Claude Code Skill 安装到 `~/.claude/skills/scu-cli/`（设 `SCU_CLI_NO_SKILL=1` 可跳过）

装好后 `scu` 直接在 PATH 中，验证：`scu --version`。

### 从 Release 下载

[Releases](https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases) 提供各平台预编译单二进制（无依赖），资产附带 `checksums.txt`（SHA256）：

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

### 其他方式

```bash
go install github.com/The-Brotherhood-of-SCU/SCU-CLI/cmd/scu@latest
# 或源码构建：git clone ... && go build -o scu ./cmd/scu
```

## AI Agent 集成（Skill）

仓库附带 Agent Skill（`skill/scu-cli/SKILL.md`），Release 中以 `scu-cli-skill.zip` 分发，内含 `scu-cli/SKILL.md`（登录流程、输出约定、全部命令文档、参数发现链、写操作准则）。安装后 agent 会在涉及川大校园服务时自动使用它。

**一键安装（推荐）**：

```bash
npm install -g @the-brotherhood-of-scu/scu-cli
```

CLI 与 Skill 一次装好，之后 agent 会话中直接说"帮我查下学期的课表"即可。首次使用 agent 会按 skill 指引执行 `scu login -u <学号> -p <密码>`（验证码内置 OCR 自动识别），之后的会话续期全自动。

**手动安装（无 Node 环境）**：

**1. 安装 CLI**：按上节从 Release 下载对应平台二进制并放入 `PATH`（skill 会指导 agent 完成，也可以你提前做好）。

**2. 安装 Skill**：

```bash
# 用户级（对所有项目生效）；项目级则解压到 <项目>/.claude/skills/
mkdir -p ~/.claude/skills
curl -L -o /tmp/scu-cli-skill.zip \
  https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases/latest/download/scu-cli-skill.zip
unzip -o /tmp/scu-cli-skill.zip -d ~/.claude/skills/
# 结果：~/.claude/skills/scu-cli/SKILL.md
```

```powershell
# Windows PowerShell（用户级）
Invoke-WebRequest -Uri https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases/latest/download/scu-cli-skill.zip -OutFile $env:TEMP\scu-cli-skill.zip
Expand-Archive -Force $env:TEMP\scu-cli-skill.zip "$env:USERPROFILE\.claude\skills"
```


## 输出约定

所有命令向 stdout 输出 JSON：

```json
// 成功
{"ok": true, "data": { ... }}
// 失败（退出码非 0）
{"ok": false, "error": {"kind": "unauthenticated", "message": "..."}}
```

`error.kind` 分类：

| kind | 含义 | 建议处理 |
|---|---|---|
| `unauthenticated` | 未登录或会话已失效且自动恢复失败 | 重新执行 `scu login` |
| `service` | 服务端业务错误 / 网络错误 | 按 message 判断 |
| `login` | 登录阶段错误（验证码错误、密码错误等） | 修正后重试 |
| `config` / `input` | 本地配置或输入错误 | 检查输入 |

凭据存放于用户配置目录（Linux/macOS `~/.config/scu-cli/`，Windows `%APPDATA%\scu-cli\`）的 `credentials.json`（0600 权限）。可用环境变量 `SCU_CLI_CONFIG_DIR` 覆盖。

## 登录

### 交互式登录（默认本地 OCR 识别验证码）

```bash
scu login
# 提示输入学号、密码（不回显），验证码由内置本地 OCR 自动识别
# OCR 置信度不足时自动保存图片到配置目录 captcha.png 并提示人工识别
# --no-ocr 可强制人工识别
```

OCR 为纯本地推理（质心模板匹配，算法与权重移植自同组织的浏览器扩展 [scu-plus](https://github.com/The-Brotherhood-of-SCU/scu-plus)，GPL-3.0），无任何网络请求。服务端返回 `invalid_captcha` 时自动换新验证码重试（最多 5 次），与 App 行为一致。

### AI / 非交互登录

```bash
scu login -u 2023xxxxxx -p '密码'
# 验证码由内置本地 OCR 自动识别；服务端返回 invalid_captcha 时自动换新重试（最多 5 次）
```

所有交互式输入（学号、密码、人工验证码）**仅在 stdin 是交互终端时才会发起**；在非 TTY 环境（AI 通过 bash 调用）下会立即报错退出，绝不阻塞。

> 注意：`-p` 会进入 shell 历史和进程列表，人机使用时建议省略 `-p` 走交互输入。

OCR 为纯本地推理（质心模板匹配，算法与权重移植自同组织的浏览器扩展 [scu-plus](https://github.com/The-Brotherhood-of-SCU/scu-plus)，GPL-3.0），无任何网络请求。登录成功后保存账号密码用于会话过期自动重新登录（同样走本地 OCR，全自动）。

OCR 连续失败时的备用手段：`scu captcha --solve` 可单独取验证码（输出 `captcha_code` + 图片路径 + OCR 结果），再配合 `scu login -u <学号> --captcha-code <code> --captcha-text <文本>` 提交。

### 其他认证命令

```bash
scu whoami    # 当前账号与凭据状态
scu logout    # 退出登录并清除凭据
```

## 教务系统 `scu zhjw`

所有编号类参数都有对应的发现命令，不用猜（各命令 `--help` 也标明了来源）：

| 参数 | 来源 |
|---|---|
| `planCode` | `scu zhjw semesters` 输出的 `value` |
| 校区/教学楼编号与名称 | `scu zhjw classroom index` → `campuses[].campusNumber/campusName`、`buildings[].teachingBuildingNumber/teachingBuildingName` |
| 学院/年级代码 | `scu zhjw program colleges` / `grades` 输出的 `value` |
| `fajhh` | `scu zhjw program search` 输出记录 |
| `urlPath` | `scu zhjw program detail` 输出 `treeList` 节点 |
| 院系/专业/班级编号 | `scu zhjw class options` → `subjects` → `list` 逐级获取 |

```bash
scu zhjw week                          # 当前教学周
scu zhjw semesters                     # 学期列表（value 为 planCode，如 2025-2026-2-1）
scu zhjw schedule --plan 2025-2026-2-1 # 课表（原始 JSON）
scu zhjw grades                        # 及格成绩
scu zhjw grades --scheme               # 方案成绩
scu zhjw exams                         # 考表（考试安排）
scu zhjw completion                    # 计划完成度
scu zhjw calendar                      # 校历（免认证，网络优先、失败回退本地缓存）

# 教室查询（编号链：index → types → query）
scu zhjw classroom index               # 校区与教学楼列表（所有编号的来源）
scu zhjw classroom types --campus-num 1 --building-num 101 --campus-name 江安 --building-name 一教A
scu zhjw classroom query --campus-num 1 --building-num 101 --date 2026-08-14

# 培养方案（编号链：colleges/grades → search → detail → course）
scu zhjw program colleges              # 学院列表（value = --college）
scu zhjw program grades                # 年级列表（value = --grade）
scu zhjw program search --college 301 --grade 2023   # 记录含 fajhh
scu zhjw program detail <fajhh>        # 方案详情（treeList 含 urlPath）
scu zhjw program course <urlPath>      # 课程详情

# 班级课表（编号链：options → subjects → list → schedule）
scu zhjw class options                 # 筛选项（semesters/grades/departments 的 value）
scu zhjw class subjects <departmentNum>                # 专业列表（含 subjectCode）
scu zhjw class list --plan 2025-2026-2-1 --dept 301    # 记录含 id.executiveEducationPlanNumber / id.classNum
scu zhjw class schedule <planCode> <classCode>         # 即上一步输出的两个字段
```

## 用户信息 `scu user`（微服务）

```bash
scu user info       # 基本信息（realname、role.number 学号等）
scu user labels     # 用户标签
scu user devices    # 校园网在线设备
```

## 缴费平台 `scu balance`

房间参数逐级发现（各级输出均为 `[{name, code}]`，上一级的 `code` 是下一级的入参）：

```bash
scu balance campus                    # 校区列表，code = schoolCode
scu balance buildings <schoolCode>    # 楼栋列表，code = regCode
scu balance units <schoolCode> <regCode>  # 单元列表，code = unitCode

# 查询余额（--type 1 照明电费 / 2 空调电费）
# 首次查询需绑定房间：
scu balance query --type 1 --school-code 1 --reg-code 101 --unit-code 1 --room 101
# 之后直接查询已绑定房间：
scu balance query --type 1
scu balance query --type 2            # 空调电费
```

学号与姓名自动取自微服务用户信息（`role.number` + `realname`）。

## 体测系统 `scu fitness`

```bash
scu fitness notices        # 体测通知
scu fitness score          # 今年体测成绩
scu fitness score --year 2025
```

## 第二课堂 `scu ccyl`

ID 发现链：`activities`（→ `activityLibraryId`）→ `lib-detail`（→ `activityId`）→ `score-types`（→ `--score-type`）；`credits`（→ `creditId`）。

```bash
scu ccyl activities --name 讲座 --page 1 --size 10   # 搜索活动库，记录的 id = activityLibraryId
scu ccyl mine                                        # 我参与的活动
scu ccyl orgs                                        # 组织列表
scu ccyl lib-detail <activityLibraryId>              # 活动系列详情，含场次活动 id = activityId
scu ccyl detail <activityId>                         # 活动详情
scu ccyl score-types <activityLibraryId>             # 能力类型，id = signup --score-type
scu ccyl signup <activityId> --score-type <id>       # 报名
scu ccyl cancel <activityId>                         # 取消报名
scu ccyl subscribe <activityLibraryId>               # 预约活动系列
scu ccyl unsubscribe <activityLibraryId>
scu ccyl credits                                     # 成绩单（记录含 creditId）
scu ccyl export <email> <creditId...>                # 导出成绩单到邮箱
```

## 认证架构

三层结构，与 Bugaoshan 对应：

```
L3 ScuAuth（根认证，id.scu.edu.cn）
  ├─ SM2(C1C2C3) 加密密码 + 验证码 → rest_token 获取 access token
  ├─ session/save 绑定统一认证 cookie session
  └─ 本地 1h TTL → 过期自动续期：重绑 token → 失败则凭据+验证码自动重登
L2 子系统认证
  ├─ ZhjwAuth    JWT SSO（scdxplugin_jwt23）
  ├─ WfwAuth     get-info 重定向链预热（仅以 e==0 JSON 判就绪）
  ├─ PayAppAuth  SSO 中继（airWarrant，依赖 WFW）
  ├─ FitnessAuth SSO 中继（pead）
  └─ CcylAuth    OAuth（sp_logged 链取 code → loginByUc 换 token，绑定 principal）
L1 业务 API（无认证状态）
  └─ 识别各后端过期信号 → invalidate → 重新认证 → 业务请求只重放一次
```

关键不变量：

- CookieClient 按域存 cookie，手动跟随重定向（≤10 跳），跨源剥离 Authorization/Cookie 等敏感 header
- 根 client 身份变化时，子系统 session 缓存一律作废重建
- CCYL token 与 SCU principal 绑定，跨账号不恢复
- 只有明确的认证失效信号才触发重试；普通业务错误绝不重放（防止报名等写操作重复提交）

## 项目结构

```
cmd/scu/            入口
internal/
  cli/              cobra 命令定义
  auth/             认证：CookieClient、ScuAuth、子系统 Auth、SM2
  api/              业务 API：zhjw / wfw / payapp / fitness / ccyl
  ocr/              验证码本地 OCR（质心模板匹配，权重移植自 scu-plus）
  config/           凭据持久化（0600）
  output/           统一 JSON 输出包层
skill/scu-cli/      Agent Skill（SKILL.md，Release 打包为 scu-cli-skill.zip）
install.js bin/scu.js  npm 安装器（postinstall 从 Release 下载二进制并安装 Skill）
```

## 技术栈

- [cobra](https://github.com/spf13/cobra) — CLI 框架
- [resty](https://github.com/go-resty/resty) — HTTP 客户端
- [emmansun/gmsm](https://github.com/emmansun/gmsm) — SM2 国密加密
- golang.org/x/term — 密码不回显输入

## 许可证

[AGPL-3.0](LICENSE)。本项目协议层移植自 Bugaoshan（AGPL-3.0），OCR 算法与权重移植自 scu-plus（GPL-3.0）。
