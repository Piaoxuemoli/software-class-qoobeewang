/**
 * 正式录制后的「真实产物捕获」（app.close 之后、容器退出之前执行）。
 *
 * 目的：S6 卡片（终端 diff / ls 输出）与 S3 完成卡、S5 完成卡的数字必须来自
 * **产生该视频的那一次真实运行**，否则画面与字幕/卡片对不上。
 *
 * 产物目录：course/04-视频/work/seg/actual-run/（正式录制）
 *          course/04-视频/work/rehearsal/actual-run/<id>-pass<N>/（彩排）
 */
import { execFile } from 'node:child_process';
import { mkdir, stat, writeFile } from 'node:fs/promises';
import path from 'node:path';

import { WORK } from './record.mjs';
import { REPO, RESTORE, SRC } from './seed.mjs';

/** 本次运行的产物目录（正式 / 彩排分开，避免彩排覆盖正式证据）。 */
export function actualRunDir(mode, id, pass) {
  return mode === 'formal'
    ? path.join(WORK, 'seg', 'actual-run')
    : path.join(WORK, 'rehearsal', 'actual-run', `${id}-pass${pass}`);
}

/** 跑命令并完整接住 stdout/stderr/rc（非 0 不抛错——diff 的 rc=1 是预期证据）。 */
function execCapture(cmd, args, opts = {}) {
  return new Promise((resolve) => {
    execFile(cmd, args, { maxBuffer: 32 * 1024 * 1024, ...opts }, (err, stdout, stderr) => {
      resolve({
        cmd: [cmd, ...args].join(' '),
        code: err ? (typeof err.code === 'number' ? err.code : 1) : 0,
        stdout: String(stdout ?? ''),
        stderr: String(stderr ?? ''),
      });
    });
  });
}

/** 把一条命令的结果渲染进文本文件（含命令、退出码、stdout/stderr 原文）。 */
function renderCommand(r, title) {
  return [
    `# ${title}`,
    `# 命令: ${r.cmd}`,
    `# 退出码: ${r.code}`,
    '--- stdout ---',
    r.stdout === '' ? '(空)' : r.stdout.replace(/\n$/, ''),
    '--- stderr ---',
    r.stderr === '' ? '(空)' : r.stderr.replace(/\n$/, ''),
    '',
  ].join('\n');
}

async function fileBytes(p) {
  const st = await stat(p).catch(() => null);
  return st?.isFile() ? st.size : null;
}

/**
 * seg-03 录制后：仓库里的 documents-demo.dbk + 完成卡真实数字。
 * @param {object} o
 * @param {object} o.card 完成卡六项统计（UI innerText 读到的文本，如 "458 MB"）
 * @param {number} o.backupMs 点击「开始备份」→「备份完成」的实测耗时（含 UI 往返）
 */
export async function captureBackupActualRun(o) {
  const dir = actualRunDir(o.mode, 'seg-03-backup', o.pass);
  await mkdir(dir, { recursive: true });
  const dbk = path.join(REPO, 'documents-demo.dbk');
  const ls = await execCapture('ls', ['-l', dbk]);
  const statCmd = await execCapture('stat', ['-c', '%s %Y %n', dbk]);
  await writeFile(
    path.join(dir, 'seg03-ls-dbk.txt'),
    renderCommand(ls, 'ls -l ~/demo/DataBackup/documents-demo.dbk'),
  );
  await writeFile(
    path.join(dir, 'seg03-stat-dbk.txt'),
    renderCommand(statCmd, "stat -c '%s %Y %n' ~/demo/DataBackup/documents-demo.dbk"),
  );

  const statsJson = {
    capturedAt: new Date().toISOString(),
    run: {
      mode: o.mode,
      pass: o.pass,
      video: o.video?.file ?? null,
      videoDurationSec: o.video?.durationSec ?? null,
    },
    source: SRC,
    repo: REPO,
    backupFile: dbk,
    backupFileBytes: await fileBytes(dbk),
    backupFileStat: { code: statCmd.code, stdout: statCmd.stdout.trim() },
    completionCard: o.card,
    backupMsWallClock: o.backupMs ?? null,
    ls: { code: ls.code, stdout: ls.stdout.trim() },
    how: 'captureBackupActualRun()：app.close 之后在录制容器内执行 ls/stat + 读取完成卡 innerText',
  };
  await writeFile(path.join(dir, 'seg03-stats.json'), `${JSON.stringify(statsJson, null, 2)}\n`);
  return { dir, files: ['seg03-ls-dbk.txt', 'seg03-stat-dbk.txt', 'seg03-stats.json'] };
}

/**
 * seg-05 录制后：diff 无输出、FIFO/软链接类型、仓库列表 + 还原完成卡真实数字。
 * @param {object} o
 * @param {object} o.card 完成卡统计（文件/目录/数据量/耗时 的文本）
 */
export async function captureRestoreActualRun(o) {
  const dir = actualRunDir(o.mode, 'seg-05-restore', o.pass);
  await mkdir(dir, { recursive: true });

  // 1) diff -r 源目录 vs 还原目录（node_modules 被过滤、pipe 为 FIFO 需 diff 排除）
  const diff = await execCapture('diff', [
    '-r',
    SRC,
    RESTORE,
    '--exclude',
    'node_modules',
    '--exclude',
    'pipe',
  ]);
  await writeFile(
    path.join(dir, 'seg05-diff.txt'),
    [
      renderCommand(
        diff,
        'diff -r ~/demo/documents ~/demo/documents-restored --exclude node_modules --exclude pipe',
      ),
      '# 判读：退出码 0 且 stdout/stderr 均为空 = 目录树逐字节一致（S6 卡片用）',
      '',
    ].join('\n'),
  );

  // 2) FIFO / 软链接类型校验（diff -r 对 FIFO 会报 "is a fifo while..." 并 rc=1，
  //    所以特殊文件类型单独用 test -p / test -L 证明）
  const fifo = await execCapture('bash', [
    '-c',
    `test -p ${RESTORE}/pipe && echo "FIFO 类型一致"; test -L ${RESTORE}/docs-link && echo "软链接类型一致"`,
  ]);
  const fifoOut = [fifo.stdout.trim(), fifo.stderr.trim()].filter(Boolean).join('\n');
  await writeFile(
    path.join(dir, 'seg05-fifo-check.txt'),
    [
      '# 特殊文件类型校验',
      `# 命令: test -p ${RESTORE}/pipe && echo "FIFO 类型一致"; test -L ${RESTORE}/docs-link && echo "软链接类型一致"`,
      `# 退出码: ${fifo.code}`,
      '--- 输出 ---',
      fifoOut === '' ? '(空)' : fifoOut,
      '',
    ].join('\n'),
  );

  // 3) 仓库里的 .dbk 列表（S6 卡片 ls -l 用）
  const lsDbk = await execCapture('bash', ['-c', `ls -l ${REPO}/*.dbk`]);
  await writeFile(
    path.join(dir, 'seg05-ls-dbk.txt'),
    renderCommand(lsDbk, 'ls -l ~/demo/DataBackup/*.dbk'),
  );

  // 4) 还原完成卡真实数字
  const statsJson = {
    capturedAt: new Date().toISOString(),
    run: {
      mode: o.mode,
      pass: o.pass,
      video: o.video?.file ?? null,
      videoDurationSec: o.video?.durationSec ?? null,
    },
    source: SRC,
    restored: RESTORE,
    completionCard: o.card,
    restoreMsWallClock: o.restoreMs ?? null,
    diff: { code: diff.code, stdout: diff.stdout, stderr: diff.stderr },
    specialTypes: { code: fifo.code, output: fifoOut },
    how: 'captureRestoreActualRun()：app.close 之后在录制容器内执行 diff/test/ls + 读取完成卡 innerText',
  };
  await writeFile(path.join(dir, 'seg05-stats.json'), `${JSON.stringify(statsJson, null, 2)}\n`);
  return {
    dir,
    files: ['seg05-diff.txt', 'seg05-fifo-check.txt', 'seg05-ls-dbk.txt', 'seg05-stats.json'],
  };
}

/** 写 actual-run/README.md（说明来源、时间点、命令与"视频/数字必须同源"）。 */
export async function writeActualRunReadme({ mode, pass, segments }) {
  const dir =
    mode === 'formal'
      ? path.join(WORK, 'seg', 'actual-run')
      : path.join(WORK, 'rehearsal', 'actual-run');
  await mkdir(dir, { recursive: true });
  const body = [
    '# 录制后真实产物（actual-run）',
    '',
    `- 采集时间：${new Date().toISOString()}`,
    `- 采集模式：${mode}（pass ${pass}）`,
    '- 采集脚本：`client/e2e/record-demo/shared/actual-run.mjs`（由 seg-03 / seg-05 的 finalize 钩子调用）',
    '- 采集时机：`app.close()` 之后、录制容器退出之前 —— **与视频同一次运行**，未做二次运行或手工补录。',
    '- 容器：`databackup-dev`（Ubuntu 22.04，diffutils 3.8）；HOME=/home/demo；后端 `databackup 1.0.1`。',
    '',
    '## 文件',
    '',
    '| 文件 | 内容 |',
    '|---|---|',
    '| `seg03-ls-dbk.txt` | seg-03 完成后 `ls -l ~/demo/DataBackup/documents-demo.dbk` 的真实输出 |',
    '| `seg03-stats.json` | seg-03 完成卡六项统计（UI innerText）+ 备份耗时 + .dbk 实际字节数 |',
    '| `seg05-diff.txt` | seg-05 完成后 `diff -r ~/demo/documents ~/demo/documents-restored --exclude node_modules --exclude pipe` 的 rc/stdout/stderr |',
    '| `seg05-fifo-check.txt` | `test -p .../pipe` 与 `test -L .../docs-link` 的真实结果（FIFO/软链接类型一致的证据） |',
    '| `seg05-ls-dbk.txt` | seg-05 完成后 `ls -l ~/demo/DataBackup/*.dbk` |',
    '| `seg05-stats.json` | seg-05 还原完成卡统计 + 还原耗时 + diff/特殊类型结果 |',
    '',
    '## 给 S6 卡片/字幕的用法',
    '',
    '- `diff` 无输出（rc=0）才可以说"目录树逐字节一致"；FIFO 的"类型一致"用 `seg05-fifo-check.txt` 证明。',
    '- S3/S5 完成卡上的数字以 `seg03-stats.json` / `seg05-stats.json` 的 `completionCard` 为准（不要用别的运行的结果）。',
    '- `.dbk` 文件名/大小以 `seg03-ls-dbk.txt`、`seg05-ls-dbk.txt` 为准。',
    '',
    '## 本次段清单',
    '',
    ...segments.map((s) => `- ${s}`),
    '',
  ].join('\n');
  await writeFile(path.join(dir, 'README.md'), body);
  return { dir, readme: path.join(dir, 'README.md') };
}
