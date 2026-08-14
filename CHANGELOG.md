# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

发布流程：推送 `vX.Y.Z` 标签触发 GitHub Action，自动从本文件提取对应版本章节作为 Release 说明，并在 Release 完成后通过 OIDC Trusted Publishing 将包发布到 npm（无需长期 token）、通过 SkillHub CLI 同步发布 Agent Skill（需配置 Secret `SKILLHUB_KEY`，未配置则跳过）。

## [0.3.0] - 2026-08-14

功能全面对齐校园生活助手 App [Bugaoshan](https://github.com/The-Brotherhood-of-SCU/Bugaoshan)，本轮补齐 9 项差距；README 重写为面向 AI 的安装与功能速查。

### 新增

- 微服务：`scu user offline --device-id <id> --ip <ip>` 强制指定校园网设备下线（与 `user devices` 成对）
- 教务 ICS 日历导出：`scu zhjw schedule/exams/calendar --ics <file>`——课表按周次/单双周/节次展开为日历事件（校区节次时段表自动探测，支持 `--start-date`/`--campus` 覆盖），考表与校历直接导出
- 第二课堂：`scu ccyl subscribed`（已预约活动系列列表）、`scu ccyl dicts <groupCode...>`（数据字典查询，activities 筛选参数的合法枚举来源）
- 办事大厅（新命令组 `scu service`，SSO 中继认证）：
  - `service applications` 我的申请列表（--status 筛选档位，--page 分页）
  - `service form <app_id>` 查看事项动态表单结构（字段/类型/选项/必填/预填值/日期校验规则）
  - `service submit <app_id> --fields '<json>' [--attach Key=path] [--dry-run]` 动态表单提交——ShowHide 显隐与动态必填、Validate 日期顺序校验、DataSource 联动取数、省市区字典编码、附件 multipart 上传，全部本地校验后再提交
- 电费余额趋势：`balance query` 成功自动记录本地快照（按房间归集，保留 365 天），`scu balance trend [--type 1|2] [--days N]` 输出日均电费/日均度数/累计消耗与 daily_points 序列（北京日历日聚合，充值段自动跳过）
- CI：标签推送时同步发布 Skill 到 SkillHub（skillhub CLI + Secret `SKILLHUB_KEY`，未配置则告警跳过）

### 变更

- npm 包名改为 scoped 包 `@the-brotherhood-of-scu/scu-cli`
- npm 发布改用 OIDC Trusted Publishing，弃用长期 token
- README 精简为安装 + 功能速查，完整命令文档由 `skill/scu-cli/SKILL.md` 承载

## [0.2.0] - 2026-08-13

### 新增

- npm 分发：`npm install -g @the-brotherhood-of-scu/scu-cli` 一键安装——按平台/架构自动从 GitHub Release 下载二进制（SHA256 校验），并自动把 Claude Code Skill 安装到 `~/.claude/skills/scu-cli/`（`SCU_CLI_NO_SKILL=1` 跳过）；标签推送时 CI 在 Release 完成后通过 OIDC Trusted Publishing 同步发布到 npm

## [0.1.0] - 2026-08-13

首个公开版本。协议与认证架构移植自校园生活助手 App [Bugaoshan](https://github.com/The-Brotherhood-of-SCU/Bugaoshan)。

### 新增

- 统一认证登录：SM2(C1C2C3) 加密密码 + 验证码，token 持久化（0600 权限），1 小时 TTL 自动续期
- 内置验证码本地 OCR（质心模板匹配，算法与权重移植自 scu-plus），登录与会话续期全自动，零网络推理
- 子系统 SSO 自动建立：教务 JWT SSO、微服务 CAS 重定向链、缴费/体测 SSO 中继、第二课堂 OAuth
- 会话过期自愈：识别各后端过期信号，自动重新认证并将业务请求重放一次
- 教务系统：教学周、学期、课表、成绩（及格/方案）、考表、计划完成度、教室占用查询、培养方案、班级课表、校历
- 微服务：用户信息、用户标签、校园网在线设备
- 缴费平台：校区/楼栋/单元逐级发现、房间绑定、照明/空调电费余额查询
- 体测系统：通知列表、年度体测成绩
- 第二课堂：活动搜索、活动详情、报名/取消、预约/取消预约、学分成绩单、导出到邮箱
- 统一 JSON 输出包层（stdout）与机器可读错误分类（unauthenticated/service/login/config/input）
- 非 TTY 环境下交互输入立即失败（面向 AI 调用，绝不阻塞）
- 所有 num/id/code 参数在 `--help` 与 README 中标注发现链（来源命令 + 输出字段）
- 附带 Agent Skill（`skill/scu-cli/SKILL.md`），Release 中以 `scu-cli-skill.zip` 分发

### 修复

- rest_token HTTP 400：请求体缓冲为字节并携带 Content-Length（chunked 传输编码被统一认证拒绝）
- SSO 重定向链无法工作：`resty.NoRedirectPolicy()` 使任何 3xx 响应变成传输错误（auto redirect is disabled），改为 `http.ErrUseLastResponse` 哨兵；同时恢复各 API 层「302 = 会话过期」的检测能力
- 容错统一认证跳教务的畸形 Location（`/sigin ?id_token=…` 含字面空格，百分号编码后与浏览器/Dart 行为一致）

[0.3.0]: https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases/tag/v0.3.0
[0.2.0]: https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases/tag/v0.2.0
[0.1.0]: https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases/tag/v0.1.0
