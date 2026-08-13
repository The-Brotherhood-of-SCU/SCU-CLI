#!/usr/bin/env node
// scu 的 npm shim：转发到 postinstall 下载的平台二进制，原样透传参数、stdio 与退出码。
'use strict';

const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const exe = path.join(__dirname, process.platform === 'win32' ? 'scu.exe' : 'scu');

if (!fs.existsSync(exe)) {
  process.stderr.write(
    'scu 二进制缺失（安装时可能用了 --ignore-scripts，或 postinstall 下载失败）。\n' +
      `请执行：node "${path.join(__dirname, '..', 'install.js')}"\n`
  );
  process.exit(1);
}

const r = spawnSync(exe, process.argv.slice(2), { stdio: 'inherit' });
if (r.error) {
  process.stderr.write(`无法启动 scu：${r.error.message}\n`);
  process.exit(1);
}
if (r.signal) {
  process.kill(process.pid, r.signal);
} else {
  process.exit(r.status === null ? 1 : r.status);
}
