# SCU-CLI

四川大学校园服务命令行工具，面向 **AI agent 与脚本**设计。统一身份认证登录后，可操作教务、电费、体测、第二课堂等系统，所有命令输出 JSON。协议层移植自 [Bugaoshan](https://github.com/The-Brotherhood-of-SCU/Bugaoshan)（AGPL-3.0），验证码 OCR 移植自 [scu-plus](https://github.com/The-Brotherhood-of-SCU/scu-plus)（GPL-3.0）。

## 安装

### 快速安装

推荐发给Agent一键安装prompt(完整指导skill):

```
请根据 https://skillhub.cn/install/skillhub.md，安装 @user_611f6f00/scu-cli。
```

### 手动安装

```bash
npm install -g @the-brotherhood-of-scu/scu-cli   # 需要 Node.js ≥ 18
```

无 Node 环境时，从 [Releases](https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases) 下载对应平台单二进制（linux / darwin / windows × amd64 / arm64）放入 `PATH`，或用 Go 安装：

```bash
go install github.com/The-Brotherhood-of-SCU/SCU-CLI/cmd/scu@latest
```

## 快速开始

```bash
scu login -u 2023xxxxxx -p '密码'   # 验证码由内置本地 OCR 自动识别；省略 -p 则交互输入（不回显）
scu zhjw schedule --plan 2025-2026-2-1
```

凭据保存于用户配置目录的 `credentials.json`（0600 权限），会话过期自动重新登录。非 TTY 环境下需要交互输入时会立即报错而非阻塞。

## 输出约定

stdout 恒为单个 JSON 对象，诊断信息走 stderr：

```json
{"ok": true, "data": { ... }}
{"ok": false, "error": {"kind": "unauthenticated", "message": "..."}}
```

失败时退出码非 0。`error.kind`：`unauthenticated`（重新 `scu login`）、`login`（登录阶段错误）、`service`（服务端/网络错误）、`config` / `input`（本地输入错误）。

## 功能一览(详情请查看skill)

编号类参数（planCode、校区/教学楼/学院/房间 code 等）全部由对应的发现命令逐级提供，不用猜；各命令 `--help` 标明来源。完整参数文档见 Skill（`skill/scu-cli/SKILL.md`，npm 安装时已自动安装）。

### 认证

| 命令 | 说明 |
|---|---|
| `scu login [-u 学号] [-p 密码] [--no-ocr]` | 登录，验证码本地 OCR 识别，失败自动换新重试 |
| `scu whoami` / `scu logout` | 查看凭据状态 / 退出并清除凭据 |
| `scu captcha --solve` | 单独取验证码（OCR 连续失败时的人工备用通道） |

### 教务 `scu zhjw`

| 命令 | 说明 |
|---|---|
| `week` / `semesters` / `calendar` | 当前教学周 / 学期列表 / 校历（免认证） |
| `schedule --plan <planCode>` | 课表 |
| `schedule/exams/calendar --ics <file>` | 导出 ICS 日历文件（课表按周次/单双周/节次展开，节次时段按校区自动探测；支持 `--start-date`、`--campus` 覆盖） |
| `grades [--scheme]` | 及格成绩 / 方案成绩 |
| `exams` / `completion` | 考表 / 计划完成度（多份培养方案，主修/辅修/微专业各一份） |
| `classroom index → types → query` | 空闲教室查询（逐级取校区/教学楼编号） |
| `program colleges → grades → search → detail → course` | 培养方案查询 |
| `class options → subjects → list → schedule` | 班级课表 |
| `course index → search → schedule` | 课程课表（按教学班查排课安排） |

### 用户（微服务）`scu user`

| 命令 | 说明 |
|---|---|
| `info` / `labels` / `devices` | 基本信息 / 用户标签 / 校园网在线设备 |
| `offline --device-id <id> --ip <ip>` | 强制指定校园网设备下线（参数取自 devices 输出） |

### 电费 `scu balance`

| 命令 | 说明 |
|---|---|
| `campus → buildings → units` | 逐级发现校区/楼栋/单元 code |
| `query --type 1\|2 [--school-code … --room …]` | 照明/空调电费余额；首次带房间参数即绑定，之后直接查；每次成功自动记录本地趋势快照 |
| `trend [--type 1\|2] [--days N]` | 余额趋势分析（北京日历日聚合、充值段自动跳过，输出日均消耗/累计与 daily_points 序列） |

### 体测 `scu fitness`

| 命令 | 说明 |
|---|---|
| `notices` / `score [--year 2025]` | 体测通知 / 体测成绩 |

### 第二课堂 `scu ccyl`

| 命令 | 说明 |
|---|---|
| `activities` / `mine` / `subscribed` / `orgs` | 搜索活动库 / 我参与的 / 已预约的活动系列 / 组织列表 |
| `dicts <groupCode...>` | 数据字典（activities 筛选参数的合法枚举来源） |
| `lib-detail → detail` | 活动系列详情 → 场次活动详情 |
| `score-types → signup` / `cancel` | 报名（需能力类型 id）/ 取消报名 |
| `subscribe` / `unsubscribe` | 预约 / 取消预约活动系列 |
| `credits` / `export <email> <creditId...>` | 成绩单 / 导出成绩单到邮箱 |

### 办事大厅 `scu service`

| 命令 | 说明 |
|---|---|
| `applications [--status 0\|1\|3] [--page N]` | 我的申请列表（请假/报备等事项的进度，状态看 `inst_status` 字段） |
| `form <app_id>` | 查看事项表单结构（字段 key/类型/选项/必填/预填值/日期校验） |
| `submit <app_id> --fields '<json>' [--attach Key=path] [--dry-run]` | 动态表单提交（选项/日期/省市区/附件类型感知，ShowHide 显隐与必填本地校验，先 `--dry-run` 预览提交体） |

### 在线报修 `scu repair`

| 命令 | 说明 |
|---|---|
| `addresses` / `areas` / `projects <areaId>` | 常用地址 / 区域树 / 维修项目两级树（submit 参数的发现链） |
| `book-dates` / `book-times <date>` | 可预约上门日期 / 时段 |
| `list` / `detail <id>` | 我的报修工单（中文状态）/ 工单详情（进度时间线 + 评价对象 id） |
| `submit --address-id … --project … --content …` | 提交报修工单（`--image` 附图、`--allow-absent` 无人值守、`--dry-run` 预览提交体） |
| `withdraw <id>` / `evaluate <repairId> --star N` | 撤回工单 / 评价工单（支持按评价项 `--stars id:分数`） |
| `save-address --area-id … --area-name … --detail … --phone …` | 保存常用报修地址 |

### 无感认证 `scu passpoint`

| 命令 | 说明 |
|---|---|
| `devices` / `user` | 已绑定无感设备列表 / 校园网账户信息 |
| `add --mac <MAC> [--days N] [--exit 运营商]` | 绑定设备 MAC（绑定后连校园网自动认证；`--days 0` 为最长 6 年） |
| `cancel --mac <MAC>` | 取消指定设备的无感认证 |

## 许可证

[AGPL-3.0](LICENSE)。
