# 云打印 investigate report

日期：2026-10-11。结论：**上传协议已实测；长边、短边后端都支持，当前网页隐藏短边。CLI 尚未实现。**

## 1. DOM → JS → 后端

上传：`POST http://202.115.54.240/api/client/CloudPrint/Upload`，multipart 文件字段 `szPath`。

| DOM 字段/值 | 实际含义 | 后端证据 |
|---|---|---|
| `dwColor=1 / 2` | 黑白 / 彩色 | 黑白预览为灰度；彩色任务 `single,color,`，预览有红蓝色块 |
| `dwPaperId=9 / 8` | A4 / A3 | `szPaperDetail.dwPaperID=9 / 8`，预览尺寸分别 1487×2105 / 2105×2977 |
| `dwPaperId=-1` | 原文档纸型 | DOM/源码对应，未上传实测 |
| `dwDuplex=1` | 单面 | `szAttribe=single,` |
| `dwDuplex=2` | 双面长边 | `szAttribe=vdup,`；2 页对应每份 1 张 |
| `dwDuplex=3` | 双面短边 | `szAttribe=hdup,`；4 页对应每份 2 张。协议探测接受，网页无可选项 |
| `dwCopies=2` | 2 份 | 后端 `dwCopies=2` |
| `dwFrom/dwTo=2/3` | 原文第 2–3 页 | 后端 2 页；预览分别显示原文 page 2 / page 3 |
| `dwFrom/dwTo=0/0` | 全部页 | 4 页测试文档保留 4 页 |
| `BackURL` | 返回打印列表 | 最终 URL `code=0`，随后出现待打印任务 |

`szPaperDetail` 是 JSON 字符串，页数/张数为每份；份数另外保存在 `dwCopies`，不能把纸张数量误当总份数。

## 2. 为什么长边、短边标签看起来乱

直接证据：

1. `cprint.html:162–175`：value 1/2 正常存在，value 3 的整个 label 被 HTML 注释。
2. 静态 HTML 对 value 2 写「短边」，对被注释的 value 3 写「长边」。这些占位文字不能当最终语义。
3. `common.js` 的 `Cprint.switchLang()` 从 `/api/system/MulLan/Get?dwSubSysSN=6` 或 session 缓存读取字典：`640026=双面长边`、`640027=双面短边`，覆盖对应 `.idQuery[data-id]` 的文字。
4. `printDoc.html:90–95`：`vdup` 显示「双面长边」；非 single/vdup 的路径显示「双面短边」。
5. 自造文档提交 value 2、value 3，后端分别返回 `vdup,`、`hdup,`。

**原因：第三项被前端注释，不是后端取消短边。** 未查版本历史，不能断言以前哪次更新隐藏了它。value 3 测试通过临时修改自有页面中选中 radio 的 value 完成；未改服务器或产品代码。

## 3. 额外发现：改份数会重置页范围

复现：选「部分」，输入 2–3，再将份数改成 2 → 页面文本框变回 1–1，实际 XHR 也提交 1–1，任务仅 1 页。

原因：`cprint.js:14–24` 把 change handler 委托给所有 `.attr-wrap input`，并非仅页范围 radio。份数输入触发 change 时进入通用 else，执行 `$('#dwFrom').val(1); $('#dwTo').val(1)`。

反证/复测：先设置份数，再输入范围 → XHR 确为 2/3，后端 2 页，预览内容为原文第 2、3 页。问题在前端状态重置，**不是后端忽略页范围**。未修第三方网页；未来 CLI 直接提交明确页范围即可避开 UI bug。

## 4. 实测矩阵

使用 4 页、1933 字节的自造彩色 PDF，无个人信息。未测重复上传。

| 用例 | 最终请求 | 后端结果 | 结论 |
|---|---|---|---|
| 基础 | A4、黑白、单面、全部、1 份 | 4 页、4 张、single | 通过 |
| 页范围后改份数 | A4、长边、2 份；JS 将范围重置 1–1 | 1 页、vdup | UI bug 已复现；非 2–3 成功用例 |
| 先份数后页范围 | A4、长边、2 份、2–3 | 2 页、每份 1 张、vdup | 通过；预览核对内容 |
| 彩色 A3 | A3、彩色、单面、第 1 页 | 1 页、1 张、single/color | 通过；预览红蓝像素均存在 |
| 隐藏短边 | A4、黑白、value 3、全部、1 份 | 4 页、每份 2 张、hdup | 后端接受 |

5 个自有测试任务全部删除，删除响应均 `code=0`；最后列表只剩原用户任务。未读取用户文件内容，未执行打印或付款。

## 5. CLI 接入结论

- 复用 `ScuAuth` 已存凭据及 root cookie session，不另存密码。
- `SSoPage` 返回 **JSON 登录链接**，不是 302；先解析 result，再走 libic/CAS 中继，最后以 `Auth/Check code=0` 验证子站就绪。
- 上传成功不是 JSON：XHR 最终回跳 URL 提供 `code/message`，再查询任务。必须区别「待打印」与「已打印/已付费」。`fee` 单位未查清，不可当已扣金额。
- `CookieClient.Do` 对传输错误会自动重发 POST；接入非幂等上传前应处理此边界。用户本轮不要求重复上传测试，未追加测试。
- 建议支持 `simplex/long-edge/short-edge` → 1/2/3；提供颜色、纸型、份数、页范围即可。CLI 实际 SSO 与上传仍需单独实现/验收，本轮未触碰产品代码。

## 证据与边界

同目录 `scu-probe-*.json`：实际 XHR.send 字段、最终响应 URL、对应任务。`dom-js-mapping.json`：DOM、字典与重置复现。`preview-*.png`：自造文档预览；`color-preview-check.json`：像素检查；`cleanup.json`：清理结果。2026-10-25 根据 review 移除第三方网页源码副本；来源 URL 与调查版本 SHA256 保留在 `protocol.md`。

原始 HTTP 重定向状态/Location 未保留；XHR 记录器观察 send 和最终 responseURL，不等于完整线上原始字节抓包。短边/长边的任务编码已验证，**纸张物理翻转效果未验证，需要线下刷卡打印**。服务为 HTTP，传输无 TLS。Go 测试未运行：无 Go 代码改动。

## Sources

- [上传 DOM](http://202.115.54.240/client/new/cprintPc/cprint.html)
- [上传与页范围 JS](http://202.115.54.240/client/new/cprintPc/js/cprint.js)
- [语言字典绑定 JS](http://202.115.54.240/client/new/cprintPc/js/common.js)
- [打印列表模板](http://202.115.54.240/client/new/cprintPc/printDoc.html)
- [打印列表/预览 JS](http://202.115.54.240/client/new/cprintPc/js/printDoc.js)
