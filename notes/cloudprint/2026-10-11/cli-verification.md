# 云打印 CLI 实现与实测

日期：2026-10-11。基于 `shirakaze114/SCU-CLI` 的 `e4a2fa5`，新增云打印认证、上传、任务查询与离线参数检查，并更新 Ubuntu WSL 中的 `scu`。

## 实现

- `scu cloudprint upload <file-path>` 或 `--file`，支持颜色、纸型、单双面、份数、页范围；`--dry-run` 不登录、不上传。
- `scu cloudprint check` 与 `jobs`（`list` 别名）。复用 SCU 根凭据，在同一个 cookie jar 中解析 SSoPage JSON、走 libic/CAS 并校验 Auth/Check；已在真实接口验证成功。
- 流式 multipart，明确 Content-Length，文件 POST 不重试。301/302/303 改走受限同源 GET，307/308 不重放文件。未知结果返回 `upload_uncertain`；连接异常后可查列表提供新增同名任务候选 ID。
- 服务返回 code=0 后查列表，匹配新增同名任务；任务元数据省略姓名、账号、卡号、服务器文件路径，不输出 token、cookie、SessionId。

## 真实上传

用户明确授权上传测试。文档使用仓库已有的 4 页自造彩色 PDF；上传不执行打印或付款。三份成功任务保留供用户核对。

| 测试 | 任务 ID | 后端结果 |
|---|---|---|
| 中文及空格文件名；A4 黑白长边、2 份、第 2–3 页 | 139090 | 2 页，`vdup,`，2 份，每份 1 张 A4 |
| 彩色 A3 短边、全部页 | 139092 | 4 页，`hdup,color,`，1 份，每份 2 张 A3 |
| 原文档纸型、单面、第 1 页 | 139094 | 1 页，`single,`，1 份，识别原纸型为 A4 |

三次均返回上传 code=0，`verification=matched_new_job`，任务 `dwStatus=1`、`dwFee=0`。`fee_raw` 仅保留返回参数，不解释金额单位或已扣费。

首次使用 RFC 5987 `filename*=` 传中文文件名时返回 code=1，任务列表未新增对应任务。改成浏览器 FormData 使用的带引号 UTF-8 `filename=` 后，同一文档和设置成功；已修复为浏览器兼容格式。

完整脱敏结果见 `cli-verification.json`。Go 1.26.9、`go test ./...` 与 `go vet ./...` 通过。测试覆盖 cookie 链、认证缓存失效、重定向目标校验、协议参数映射、中文文件名及文件内容、Content-Length、任务解析、连接断开/307/缺结果码/认证失效/413 均不重复提交，以及无登录的 dry-run 与本地 input 错误。

验证边界：真实上传只测试 PDF；其他后缀的服务器转换、物理双面翻转与实际终端打印未验证。
