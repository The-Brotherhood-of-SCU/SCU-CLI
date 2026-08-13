# SCU-CLI

四川大学校园服务命令行工具，面向 **AI 代理与脚本**设计。通过统一身份认证登录后，可操作教务系统、微服务、缴费平台、体测系统、第二课堂等校园服务。

协议与认证架构移植自校园生活助手 App [Bugaoshan](https://github.com/The-Brotherhood-of-SCU/Bugaoshan)。

## 特性

- **统一认证**：SM2 加密密码登录、验证码、token 持久化、1 小时 TTL 自动续期
- **子系统 SSO 自动建立**：教务 JWT SSO、微服务 CAS 重定向链、缴费/体测 SSO 中继、第二课堂 OAuth
- **会话过期自愈**：识别各后端过期信号，自动重新认证并重放业务请求一次
- **机器可读输出**：所有命令输出统一 JSON 包层到 stdout，诊断信息走 stderr

## 安装

```bash
go install github.com/The-Brotherhood-of-SCU/SCU-CLI/cmd/scu@latest
```

或从源码构建：

```bash
git clone https://github.com/The-Brotherhood-of-SCU/SCU-CLI
cd SCU-CLI
go build -o scu ./cmd/scu
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

### 交互式登录

```bash
scu login
# 提示输入学号、密码（不回显），验证码图片保存到配置目录 captcha.png 并提示识别
```

### AI 两步登录（推荐 AI 使用）

```bash
# 第一步：获取验证码（图片路径与 code 以 JSON 返回）
scu captcha
# => {"ok":true,"data":{"captcha_code":"...","image_path":".../captcha.png",...}}

# 第二步：识别图片文本后登录
scu login -u 2023xxxxxx --captcha-code <code> --captcha-text <验证码文本>
```

密码建议交互输入而非命令行传递（避免进入 shell 历史）。登录成功后保存账号密码用于会话过期自动重新登录（此时若 token 彻底失效仍需验证码，命令会提示重新 login）。

### 其他认证命令

```bash
scu whoami    # 当前账号与凭据状态
scu logout    # 退出登录并清除凭据
```

## 教务系统 `scu zhjw`

```bash
scu zhjw week                          # 当前教学周
scu zhjw semesters                     # 学期列表（value 为 planCode，如 2025-2026-2-1）
scu zhjw schedule --plan 2025-2026-2-1 # 课表（原始 JSON）
scu zhjw grades                        # 及格成绩
scu zhjw grades --scheme               # 方案成绩
scu zhjw exams                         # 考表（考试安排）
scu zhjw completion                    # 计划完成度
scu zhjw calendar                      # 校历（免认证，网络优先、失败回退本地缓存）

# 教室查询
scu zhjw classroom index               # 校区与教学楼列表
scu zhjw classroom types --campus-num 1 --building-num 101 --campus-name 江安 --building-name 一教A
scu zhjw classroom query --campus-num 1 --building-num 101 --date 2026-08-14

# 培养方案
scu zhjw program colleges              # 学院列表
scu zhjw program grades                # 年级列表
scu zhjw program search --college 301 --grade 2023
scu zhjw program detail <fajhh>        # 方案详情（含课程树）
scu zhjw program course <urlPath>      # 课程详情（urlPath 来自方案详情 treeList）

# 班级课表
scu zhjw class options                 # 筛选项（学期/年级/院系）
scu zhjw class subjects <departmentNum>
scu zhjw class list --plan 2025-2026-2-1 --dept 301
scu zhjw class schedule <planCode> <classCode>
```

## 用户信息 `scu user`（微服务）

```bash
scu user info       # 基本信息（realname、role.number 学号等）
scu user labels     # 用户标签
scu user devices    # 校园网在线设备
```

## 缴费平台 `scu balance`

```bash
scu balance campus                    # 校区列表
scu balance buildings <schoolCode>    # 楼栋列表
scu balance units <schoolCode> <regCode>

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

```bash
scu ccyl activities --name 讲座 --page 1 --size 10   # 搜索活动库
scu ccyl mine                                        # 我参与的活动
scu ccyl orgs                                        # 组织列表
scu ccyl detail <activityId>                         # 活动详情
scu ccyl lib-detail <activityLibraryId>              # 活动系列详情
scu ccyl score-types <activityLibraryId>             # 能力类型（报名参数）
scu ccyl signup <activityId> --score-type <id>       # 报名
scu ccyl cancel <activityId>                         # 取消报名
scu ccyl subscribe <activityLibraryId>               # 预约活动系列
scu ccyl unsubscribe <activityLibraryId>
scu ccyl credits                                     # 成绩单（学分）
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
  config/           凭据持久化（0600）
  output/           统一 JSON 输出包层
```

## 技术栈

- [cobra](https://github.com/spf13/cobra) — CLI 框架
- [resty](https://github.com/go-resty/resty) — HTTP 客户端
- [emmansun/gmsm](https://github.com/emmansun/gmsm) — SM2 国密加密
- golang.org/x/term — 密码不回显输入
