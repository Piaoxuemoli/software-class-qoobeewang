/**
 * 录制数据/后端速度校准（不是录制脚本，供 S3/S5 排时长用）。
 *
 * 在容器里跑一套真实预置 + 备份 + 还原，测量：
 *   - 源树构建耗时 / dry-run 扫描耗时
 *   - Huffman+AES 备份耗时（S3 进度段总长，目标 10–14s）
 *   - 还原耗时（S5 进度段总长）
 *   - 错误口令被拒耗时（S5「还可尝试 2 次」出现时刻）
 * 结果写 course/04-视频/work/seed/calibration.json。
 *
 * 运行（容器内，HOME=/home/demo）：
 *   node e2e/record-demo/shared/calibrate.mjs
 */
import { execFile } from 'node:child_process';
import { mkdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';

import { HOME, PW, REPO, SRC, WRONG_PW, parseResult, seedHome } from './seed.mjs';

const repoDir = path.resolve(import.meta.dirname, '..', '..', '..', '..');
const WORK = path.join(repoDir, 'course', '04-视频', 'work');
const OUT = path.join(WORK, 'seed', 'calibration.json');
const backendBin = process.env.DATABACKUP_BIN ?? path.join(repoDir, 'backend', 'bin', 'databackup');
const log = (...a) => console.log('[calib]', ...a);

function cli(args, stdin) {
  return new Promise((resolve, reject) => {
    const t0 = Date.now();
    const child = execFile(
      backendBin,
      ['--json', ...args],
      {
        env: { ...process.env, HOME, XDG_CONFIG_HOME: `${HOME}/.config` },
        maxBuffer: 32 * 1024 * 1024,
      },
      (err, stdout, stderr) => {
        const ms = Date.now() - t0;
        const result = parseResult(stdout);
        if (err) {
          if (stderr.trim()) log('stderr:', stderr.trim().split('\n').slice(0, 3).join(' | '));
          reject(Object.assign(new Error(`cli exit ${err.code}`), { ms, result, stdout, stderr }));
          return;
        }
        resolve({ ms, result, stdout, stderr });
      },
    );
    child.stdin?.end(stdin === undefined ? undefined : `${stdin}\n`);
  });
}

const TARGET_CAL = `${HOME}/documents-calib-restored`;
const TARGET_WRONG = `${HOME}/documents-calib-wrong`;

const report = { startedAt: new Date().toISOString(), env: { home: HOME, backendBin } };

// 1) 预置源树（空仓库）
const seed = await seedHome('empty', { logFile: path.join(WORK, 'seed', 'calibration-seed.json') });
report.seed = { timings: seed.timings, source: seed.source };

// 2) dry-run（S2 预览行耗时）
try {
  const dry = await cli(['backup', '--source', SRC, '--dry-run']);
  report.dryRunMs = dry.ms;
  report.dryRun = dry.result;
  log('dry-run', dry.ms, 'ms', JSON.stringify(dry.result));
} catch (e) {
  report.dryRunError = String(e);
}

// 3) S3 备份：排除 node_modules + huffman + aes256
try {
  const backup = await cli(
    [
      'backup',
      '--source',
      SRC,
      '--repo',
      REPO,
      '--name',
      'documents-demo',
      '--exclude-dir',
      'node_modules',
      '--compress',
      'huffman',
      '--encrypt',
      'aes256',
      '--password-stdin',
    ],
    PW,
  );
  report.backupMs = backup.ms;
  report.backup = backup.result;
  log('backup', backup.ms, 'ms', JSON.stringify(backup.result));
} catch (e) {
  report.backupError = String(e);
  log('backup FAILED', String(e));
}

// 4) 正确口令还原（S5 进度段耗时；先整体校验再写盘）
try {
  await rm(TARGET_CAL, { recursive: true, force: true });
  const restore = await cli(
    [
      'restore',
      '--backup',
      'documents-demo',
      '--repo',
      REPO,
      '--target',
      TARGET_CAL,
      '--password-stdin',
    ],
    PW,
  );
  report.restoreMs = restore.ms;
  report.restore = restore.result;
  log('restore', restore.ms, 'ms', JSON.stringify(restore.result));
} catch (e) {
  report.restoreError = String(e);
  log('restore FAILED', String(e));
}

// 5) 错误口令（S5 错误提示出现耗时；E_AUTH 退出码 3）
try {
  await rm(TARGET_WRONG, { recursive: true, force: true });
  const restore = await cli(
    [
      'restore',
      '--backup',
      'documents-demo',
      '--repo',
      REPO,
      '--target',
      TARGET_WRONG,
      '--password-stdin',
    ],
    WRONG_PW,
  );
  report.wrongPassword = { unexpectedOk: restore.result, ms: restore.ms };
  log('wrong password UNEXPECTEDLY OK', JSON.stringify(restore.result));
} catch (e) {
  report.wrongPassword = { ms: e.ms, errno: e.code, result: e.result };
  log('wrong password rejected in', e.ms, 'ms', JSON.stringify(e.result));
}

// 6) 清理校准产物（仓库/还原目录；源树保留给后续步骤复用时可再 seedHome）
await rm(`${REPO}/documents-demo.dbk`, { force: true });
await rm(TARGET_CAL, { recursive: true, force: true });
await rm(TARGET_WRONG, { recursive: true, force: true });

report.finishedAt = new Date().toISOString();
await mkdir(path.dirname(OUT), { recursive: true });
await writeFile(OUT, `${JSON.stringify(report, null, 2)}\n`);
log('report ->', OUT);
console.log(JSON.stringify(report));
