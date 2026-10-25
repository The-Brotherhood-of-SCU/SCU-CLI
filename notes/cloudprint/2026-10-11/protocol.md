# 川大联创云打印：上传协议调查

日期：2026-10-11。此文件记录第一轮协议调查；随后参数补测、DOM/JS对应与最新结论见 [investigate-report.md](investigate-report.md)。第一轮上传用户所选文件一次；第二轮使用自造文档并清理自有测试任务，未执行打印/付款、未删除用户任务。CLI 代码未修改。

## 直接观察

- 服务：`http://202.115.54.240`，页脚 `V3.0.20220822`。
- 点「云打印」后进入统一身份认证；用户完成登录后返回 `/client/new/cprintPc/cprint.html`。
- 上传前 `GET /api/client/PrintJob/Get`：`code=0`，0 个任务。
- 用户选择 PDF，174715 字节。提交前表单：黑白 `1`、A4 `9`、单面 `1`、全部页 `0`、1 份。未读取 PDF 内容；文件名不存入调查材料。
- 点「确定」后页面显示「上传成功」。XHR 最终请求为：
  `/client/new/cprintPc/printDoc.html?t=<timestamp>&code=0&fee=20&message=`，HTTP 200。
- 随后跳到 `/client/new/cprintPc/printDoc.html`，列表新增 1 个任务。结果见 `observations.json`：2 页、2 张 A4 黑白、单面、1 份、`dwStatus=1`。
- 登录后 `POST /api/client/Auth/Check` 返回 `code=0`、非空 `SessionId`；可见 cookie 名 `OSESSIONID`。不保存 cookie/session 值或个人字段。
- `fee=20` 是返回 URL 参数；列表 `dwFee=0`。金额单位、计费时间未查清，不能解读成已扣款。

## 抓包证据边界

WebBridge `network` 在上传前已开启，但上传重定向后，同一 requestId `27997.38` 仅保留最终 GET；`detail` 返回 `No resource with given identifier found`。**原始 multipart 字节、上传响应的原始状态码/Location 未留存**。

以下 POST 路径与字段由公开前端源码、提交前实际 FormData 核对；成功结论来自页面提示、最终 `code=0`、列表从 0 到 1 三项实测。不能把源码推导说成完整原始 POST 抓包。未重复提交用户文档以补抓包。

## 上传协议（前端源码）

`POST /api/client/CloudPrint/Upload`，`multipart/form-data`。同源 XHR，浏览器自动携带服务 cookie；前端未显式设置 Bearer 或 CSRF header。未验证服务端是否还有额外要求。

| 字段 | 语义 | 本次值 |
|---|---|---|
| `szPath` | 文件 part，含 filename 与文件数据 | 用户所选 PDF |
| `dwColor` | `1` 黑白；`2` 彩色 | `1` |
| `dwPaperId` | `-1` 原文档纸型；`9` A4；`8` A3 | `9` |
| `dwDuplex` | `1` 单面；`2` 长边；`3` 短边（前端隐藏，补测后端接受） | `1` |
| `dwFrom` | 全部页 `0`；部分页为起始页 | `0` |
| `dwTo` | 全部页 `0`；部分页为结束页，JS 写入 FormData | `0`（源码推导） |
| `dwCopies` | 份数 | `1` |
| `BackURL` | 返回打印列表 URL，加 `t` 时间戳 | `http://202.115.54.240/client/new/cprintPc/printDoc.html?t=<timestamp>` |

注意：下载的 HTML 对 `dwDuplex=2` 标「双面短边」，浏览器实际 DOM 标「双面长边」。补测查明语言字典覆盖静态标签，后端 value 2→vdup、3→hdup，列表模板分别显示长边、短边。value 3 的 label 被注释。物理翻转效果仍需线下打印验证。

前端：`new FormData(form)` → 全部页写 `dwTo=0`，部分页重写 `dwFrom/dwTo` → XHR POST → 自动跟随跳转 → 从最终 `responseURL` 读 `code` 和 `message`。`code=0` 显示成功，然后导航到打印列表；非零显示错误。HTTP 413 提示文件过大；其他非 200 提示失败。当前 `set_filesize` 为空，不代表服务端没有限制。

公开前端支持 PDF、图片、TXT、Office 等后缀；仅实测 PDF。前端后缀白名单不是服务端格式保证。

## 列表与预览（源码与部分实测）

- `GET /api/client/PrintJob/Get`：JSON 包层 `code/message/result`，本次实测。
- 关键任务字段：`dwJobId`、`dwOldJobId`、`dwStatus`、`dwPages`、`dwCopies`、`szAttribe`、`szPaperDetail`；最后一个字段是 JSON **字符串**，需二次解析。
- `GET /api/client/PrintJob/PreviewPage?dwJobId=...&dwCreateTime=...&pageNo=...`：源码显示 `result` 是 PNG base64。第一轮未请求用户文件预览；第二轮仅预览自造文档，确认范围与颜色，见补测报告。
- `POST /api/client/PrintJob/Del`：源码显示 JSON `{dwJobId,dwOldJobId}`。第一轮未调用；第二轮仅删除5个自有测试任务，用户任务保留，见 `cleanup.json`。
- 帮助页说明：上传生成待打印任务；实际打印需到馆内终端刷校园卡确认。上传不等于执行打印。

## SSO 与 CLI 凭据复用

公开接口实测：

1. `GET /api/client/Auth/SSoPage?backurl=<URL-encoded cprint page>` 返回 JSON `{code:0,result:<login URL>}`，**不是 302**。
2. `result` 指向 `https://libic.scu.edu.cn/authcenter/toLoginPage`：
   - `redirectUrl=http://202.115.54.240/api/client/Auth/SSOLogin`；
   - `targetPageUrl` 为 base64 目标页，包含此次会话的 `sessionId`。
3. 浏览器进入 `id.scu.edu.cn` CAS；观测 `application_key=scdxplugin_cas_apereo24`，CAS service 为 `https://libic.scu.edu.cn/authcenter/doAuth/b293393a8e774c7a8da40b9c32faaafb`。
4. 用户登录后回到云打印页，`Auth/Check code=0`。未记录登录凭据、CAS ticket 或 sessionId 值；中间完整认证回跳未抓取。

仓库复用点：

- `internal/auth/scu.go`：`ScuAuth.GetClient()` 用既有 token 调 `session/save` 建统一认证 cookie session；过期可用已存账号密码 + OCR 重登。不需新密码存储。
- `internal/auth/subsystem.go`：`SsoRelayAuth` 仅跟随 HTTP 重定向；直接把 `SSoPage` 当 `ssoURL` 会停在 JSON 200，不能代表子站已登录。
- 建议最小云打印 L2 认证：共享 root CookieClient → 取 `SSoPage` JSON 的 `result` → 跟随 libic/CAS 回跳 → `POST Auth/Check`，仅 `code=0` 才缓存就绪。
- 必须保持取 SSoPage 与回跳在同一 CookieClient；校验返回登录 URL 的允许 origin，不把 SCU Bearer/密码发到 HTTP 打印 IP。跨源敏感 header 继续剥离。
- `internal/auth/cookie_client.go` 的普通 `Do` 不自动跟随重定向；实现上传需在 multipart POST 后正确处理重定向和最终 URL，不能直接复用只执行 GET 的跟随函数。
- 上传是非幂等写入：超时或异常结果不可盲目重发。先查询任务判断是否已落地；只有明确未提交的认证失败才考虑重试。现有 `CookieClient.Do` 对任意 method 的传输错误自动重发一次，包括 POST；接入时需避免此传输层重放，仅在业务层不重试仍不够。它还全量缓冲 body，大文件内存占用需评估。

**尚未验证 CLI 实际 SSO 链路**：当前证据证明仓库具备 root 凭据/session 复用机制，不能证明现有 CLI 无改动即可完成云打印认证。

安全：打印服务为 HTTP，文件与服务会话没有 TLS 保护。不要用敏感文档继续探测。保持文档内容、身份字段、cookie、token、票据不入日志。

## 任务账本与验收

| ID | Owner | 状态 | 产物/证据 |
|---|---|---|---|
| CP-1 | agent | done | 入口、网页协议分析；来源 URL 与 SHA256 见下文 |
| CP-2 | user + agent | done | 用户选文件，agent 点击上传；`observations.json` |
| CP-3 | agent | done | 协议、认证边界；本文件 |
| CP-4 | user | superseded | 扩展本地文件权限方案改为用户手动选文件 |

已运行：前后列表检查、Auth/Check、提交前 FormData 检查、公开源码读取；第二轮补测双面长/短边、彩色、A3、份数、页范围、自造文档预览与自有任务删除。未运行：CLI 登录/上传、其他文件格式、原纸型上传、实际打印/付款。没有实现代码变更，Go 测试未运行；本调查不能充当 CLI 实现验收。

## Sources

公开网页曾于 2026-10-11 下载供协议分析。按 2026-10-25 PR review 建议，仓库不再分发第三方 HTML/JS 副本；仅保留来源 URL、当时的 SHA256、协议说明和脱敏探测记录。报告中的源码行号指当时下载版本，不保证线上页面以后保持相同。以下哈希用于核对原调查版本，不表示当前线上内容已经重验。

- [上传页面](http://202.115.54.240/client/new/cprintPc/cprint.html)
- [上传 JS](http://202.115.54.240/client/new/cprintPc/js/cprint.js)
- [认证公共 JS](http://202.115.54.240/client/new/cprintPc/js/common.js)
- [列表 JS](http://202.115.54.240/client/new/cprintPc/js/printDoc.js)
- [列表页面](http://202.115.54.240/client/new/cprintPc/printDoc.html)
- [接口地址配置 JS](http://202.115.54.240/client/new/cprintPc/js/host.js)
- [配置 JS](http://202.115.54.240/client/new/setconfig.js)
- [帮助页](http://202.115.54.240/client/new/cprintPc/help.html)

| 原文件 | SHA256（2026-10-11） |
|---|---|
| common.js | `d84af89803d134308b3ded9efb471ca641c5ac3e76bbfbdebef3037b0859ea47` |
| cprint.html | `baf476f393b048d2238e1fb41e1d570b6387407a75724dd38e55772f96c8b341` |
| cprint.js | `dcd924151678378819134ec6bca581c80230462336d6555b13702a2ae09c3404` |
| host.js | `7eb630ee19e691d74b5b63d2ac30a3cdb48b54dafba1922e2394d98d0cf27f5a` |
| printDoc.html | `8ca7e72c9d369c86825eb4b15f116a879cc8ee327cabe6295730779845994cc0` |
| printDoc.js | `66c3179c9be988a106e34a4552b185f5740d57305d7e98a1cec37bd963055369` |
| setconfig.js | `1591ea20f54b3fc72ba42e0eea5ba5ec36c39f52ffda28a77c40175faba5c7dc` |
