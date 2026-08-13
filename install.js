#!/usr/bin/env node
/**
 * npm postinstall：
 *   1. 按当前平台/架构从 GitHub Release 下载 scu 预编译二进制（SHA256 校验）
 *   2. 把 Claude Code Skill 安装到 ~/.claude/skills/scu-cli/（SCU_CLI_NO_SKILL=1 跳过）
 *
 * 二进制托管在 GitHub Release 而非随 npm 包分发，因此 package.json 的 version
 * 必须对应一个存在的 Release 标签 v<version>（CI 发布时会用标签值覆盖 version）。
 */
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const crypto = require('crypto');

const REPO = 'The-Brotherhood-of-SCU/SCU-CLI';
const VERSION = require(path.join(__dirname, 'package.json')).version;
const BASE = `https://github.com/${REPO}/releases/download/v${VERSION}`;

const PLATFORMS = { linux: 'linux', darwin: 'darwin', win32: 'windows' };
const ARCHS = { x64: 'amd64', arm64: 'arm64' };

function info(msg) {
  process.stderr.write(`[scu-cli] ${msg}\n`);
}

function fail(msg) {
  info(`错误：${msg}`);
  process.exit(1);
}

async function download(url) {
  const res = await fetch(url, { redirect: 'follow' });
  if (!res.ok) {
    fail(
      `下载失败（HTTP ${res.status}）：${url}\n` +
        `请确认 GitHub Release v${VERSION} 存在；也可改用：\n` +
        `  go install github.com/${REPO}/cmd/scu@latest`
    );
  }
  return Buffer.from(await res.arrayBuffer());
}

async function tryDownload(url) {
  try {
    const res = await fetch(url, { redirect: 'follow' });
    if (!res.ok) return null;
    return Buffer.from(await res.arrayBuffer());
  } catch {
    return null;
  }
}

function installSkill() {
  if (process.env.SCU_CLI_NO_SKILL) {
    info('SCU_CLI_NO_SKILL 已设置，跳过 Claude Skill 安装');
    return;
  }
  try {
    const src = path.join(__dirname, 'skill', 'scu-cli', 'SKILL.md');
    if (!fs.existsSync(src)) return;
    const dir = path.join(os.homedir(), '.claude', 'skills', 'scu-cli');
    fs.mkdirSync(dir, { recursive: true });
    fs.copyFileSync(src, path.join(dir, 'SKILL.md'));
    info(`Claude Skill 已安装到 ${dir}`);
  } catch (e) {
    info(`警告：Skill 安装失败（不影响 CLI 使用）：${e.message}`);
  }
}

async function main() {
  const goos = PLATFORMS[process.platform];
  const goarch = ARCHS[process.arch];
  if (!goos || !goarch) {
    fail(
      `不支持的平台 ${process.platform}/${process.arch}（支持 linux/darwin/windows × amd64/arm64）。\n` +
        `可改用：go install github.com/${REPO}/cmd/scu@latest`
    );
  }

  const asset = `scu-${goos}-${goarch}${goos === 'windows' ? '.exe' : ''}`;
  const binDir = path.join(__dirname, 'bin');
  const dest = path.join(binDir, goos === 'windows' ? 'scu.exe' : 'scu');

  info(`下载 ${asset}（v${VERSION}）…`);
  const buf = await download(`${BASE}/${asset}`);

  // checksums.txt 由 Release CI 生成，内含全部二进制的 SHA256；拿到就强制校验
  const sums = await tryDownload(`${BASE}/checksums.txt`);
  if (sums) {
    const line = sums
      .toString('utf8')
      .split(/\r?\n/)
      .find((l) => l.trimEnd().endsWith(` ${asset}`) || l.trimEnd() === asset);
    if (!line) fail(`checksums.txt 中未找到 ${asset} 的条目`);
    const expect = line.trim().split(/\s+/)[0];
    const actual = crypto.createHash('sha256').update(buf).digest('hex');
    if (actual !== expect) fail(`SHA256 校验失败：期望 ${expect}，实际 ${actual}`);
    info('SHA256 校验通过');
  } else {
    info('警告：未获取到 checksums.txt，跳过 SHA256 校验');
  }

  fs.mkdirSync(binDir, { recursive: true });
  fs.writeFileSync(dest, buf);
  fs.chmodSync(dest, 0o755);
  info(`CLI 已安装：${dest}`);

  installSkill();
}

main().catch((e) => fail(e.message));
