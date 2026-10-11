# 2026-10-11 云打印上传调查

- 用户在浏览器选择 PDF，agent 点击上传。页面成功提示、最终 code=0、任务从0到1；2页 A4 黑白单面1份。未打印、付款或删除，任务保留。
- 产物：`notes/cloudprint/2026-10-11/protocol.md`、`observations.json`、`public-source/`。
- 上传是 multipart POST，结果经回跳 URL 提供，不是 JSON。WebBridge 覆盖重定向前原始 POST，字段从 FormData 与源码核对。
- SSoPage 返回 JSON 登录链接，先走 libic 图书馆认证中继再进入统一认证。既有 ScuAuth 可提供 root 凭据/session，但 CLI 全链路尚未实测。
- 对接风险：CookieClient.Do 会自动重发失败 POST，可能重复创建任务；HTTP 上传没有 TLS；fee 单位未核对。
- 后续按用户要求补测参数和 DOM/JS 对应：5次自造文档测试，value2长边→vdup、隐藏value3短边→hdup；页范围2–3、2份、A3彩色有效，预览内容与像素核对。静态双面标签由语言字典覆盖。
- 发现页面bug：设置页范围后change份数会重置1–1，原因在cprint.js通用change handler。先份数后范围的复测提交2–3成功。
- 自有测试任务全部删除，用户任务保留。报告：`notes/cloudprint/2026-10-11/investigate-report.md`。用户要求推送报告；CLI SSO实测仍未执行。
- CLI 代码未改，Go 测试未运行。
- 下次若实施：先验证 CLI SSO + Auth/Check，再加上传/列表最小命令，明确非幂等恢复；勿另存密码，勿把 token 送到 HTTP 打印 IP。
