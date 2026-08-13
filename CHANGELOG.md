# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

发布流程：推送 `vX.Y.Z` 标签触发 GitHub Action，自动从本文件提取对应版本章节作为 Release 说明。

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
- 附带 Agent Skill（`skill/SKILL.md`），Release 中以 `scu-cli-skill.zip` 分发

### 修复

- rest_token HTTP 400：请求体缓冲为字节并携带 Content-Length（chunked 传输编码被统一认证拒绝）
- SSO 重定向链无法工作：`resty.NoRedirectPolicy()` 使任何 3xx 响应变成传输错误（auto redirect is disabled），改为 `http.ErrUseLastResponse` 哨兵；同时恢复各 API 层「302 = 会话过期」的检测能力
- 容错统一认证跳教务的畸形 Location（`/sigin ?id_token=…` 含字面空格，百分号编码后与浏览器/Dart 行为一致）

[0.1.0]: https://github.com/The-Brotherhood-of-SCU/SCU-CLI/releases/tag/v0.1.0
