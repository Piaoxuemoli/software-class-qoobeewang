/**
 * 演示视频录制 · 数据预置（与录制分离）。
 *
 * 每个段独立重建 /home/demo，互不影响：
 *   /home/demo/documents        演示源目录 ≈ 500 MB（中文目录、两级子目录、软链接、
 *                               FIFO、待排除的 node_modules、可压缩文本 + 随机二进制）
 *   /home/demo/shared           软链接 docs-link 指向的真实目录
 *   /home/demo/DataBackup       备份仓库（空 / 预置 4 个备份）
 *
 * 所有数据确定性生成：同一份种子每次得到相同的文件数/字节数/输出大小，
 * 便于彩排与正式录制对比、也避免字幕里的数字漂移。
 *
 * 运行环境：databackup-dev 容器内（HOME 已设置为 /home/demo），由
 * run-in-docker.sh / record-all.sh 调用；宿主/独立环境请勿运行（会 rm -rf /home/demo）。
 */
import { execFile, execFileSync } from 'node:child_process';
import { mkdir, rm, stat, symlink, writeFile } from 'node:fs/promises';
import path from 'node:path';

export const HOME = '/home/demo';
export const SRC = `${HOME}/documents`;
export const REPO = `${HOME}/DataBackup`;
export const RESTORE = `${HOME}/documents-restored`;
export const SHARED = `${HOME}/shared`;
/** 演示口令（只在本地演示用；不写进任何受控文档/仓库）。 */
export const PW = 'demo-2026';
export const WRONG_PW = 'wrong-demo';

const backendBin = process.env.DATABACKUP_BIN ?? '/workspace/backend/bin/databackup';

/** 仓库预置的 4 个备份（S4/S5 用）；名字与 07-…§4 S4 一致。 */
export const HISTORY_NAMES = ['daily', 'photos', 'archive', 'documents-demo'];

/** 演示源目录里"待排除"的 node_modules 文件数（体积计入 ~500MB，但被过滤不计入备份）。 */
const NM_FILES = 120;

const log = (...a) => console.log('[seed]', ...a);

/** 确定性伪随机字节（xorshift32）；同一 seed 每次内容完全一致。 */
function pseudoBytes(n, seed = 0x9e3779b9) {
  const buf = Buffer.alloc(n);
  let s = seed >>> 0;
  for (let i = 0; i < n; i++) {
    s ^= s << 13;
    s >>>= 0;
    s ^= s >>> 17;
    s ^= s << 5;
    s >>>= 0;
    buf[i] = s & 0xff;
  }
  return buf;
}

async function writeRel(rel, data) {
  const p = path.join(SRC, rel);
  await mkdir(path.dirname(p), { recursive: true });
  await writeFile(p, data);
}

/** 批量写文件：并发度受控，避免 300 个 1.5MB 文件同时占内存。 */
async function writeMany(items, batch = 40) {
  for (let i = 0; i < items.length; i += batch) {
    await Promise.all(items.slice(i, i + batch).map(([rel, data]) => writeRel(rel, data)));
  }
}

/**
 * 造演示源目录树（确定性，总量 ≈ 500MB）：
 *  - 说明书 + 报告（小文本）
 *  - 合同 2 个 PDF（随机二进制）
 *  - 资料/归档/part-001..300.txt：300 个 ~1.5MB 的可压缩文本分片（备份主体）
 *  - 资料/数据集/原始数据.bin：16MB 随机二进制（压不动，体现压缩率）
 *  - 代码/site/src/*.ts：小文本
 *  - node_modules/：120 个文件（待排除，过滤演示）
 *  - docs-link -> /home/demo/shared（软链接）
 *  - pipe：命名管道（FIFO）
 */
export async function buildSourceTree() {
  const t0 = Date.now();
  log('build source tree ->', SRC);

  // 300 × 1.5MB 的可压缩文本（每行等长，Huffman 压缩率高且逐字节确定性）
  const line = '2026-10-17 09:12:33 INFO  sync chunk ok  size=4096 sha=9f2c1a7e\n';
  const textBlock = Buffer.from(line.repeat(24_000));
  const shards = Array.from({ length: 300 }, (_, i) => [
    `资料/归档/part-${String(i + 1).padStart(3, '0')}.txt`,
    textBlock,
  ]);
  await writeMany(shards, 40);

  // 随机二进制：16MB 数据集 + 两个合同 PDF + 8 张照片（不可压缩部分）
  const bin4m = pseudoBytes(4 * 1024 * 1024, 21);
  const bin1m = pseudoBytes(1024 * 1024, 11);
  const bin07m = pseudoBytes(700_000, 12);
  await writeRel('资料/数据集/原始数据.bin', Buffer.concat([bin4m, bin4m, bin4m, bin4m]));
  await writeRel(
    '合同/2026-服务协议.pdf',
    Buffer.concat([bin1m, bin07m, pseudoBytes(100_000, 13)]),
  );
  await writeRel('合同/2025-保密协议.pdf', Buffer.concat([bin07m, pseudoBytes(200_000, 14)]));

  // 小文本：说明书 / 报告 / 代码
  const text = (s) => Buffer.from(`${s}\n`.repeat(40));
  await writeRel('说明.txt', text('归档说明：本目录用于演示 DataBackup 的备份与还原。'));
  await writeRel(
    '报告/开题报告.md',
    text('# 开题报告\n\n- 选题：Linux 目录树备份\n- 组长：王科瑾'),
  );
  await writeRel('报告/中期检查.md', text('# 中期检查\n\n后端流水线与客户端三页面已完成。'));
  for (let i = 1; i <= 3; i += 1) {
    await writeRel(`代码/site/src/module-${i}.ts`, text(`export const module${i} = ${i};`));
  }

  // 待排除的 node_modules：120 个文件，共约 30MB
  const nmBlock = pseudoBytes(250_000, 31);
  const nmItems = [];
  for (let i = 0; i < NM_FILES; i += 1) {
    nmItems.push([
      `node_modules/pkg-${String(i).padStart(3, '0')}/index.js`,
      Buffer.concat([Buffer.from(`module.exports = ${i};\n`), nmBlock]),
    ]);
  }
  await writeMany(nmItems, 40);

  // 软链接 → /home/demo/shared；真实 shared 目录里放两个文件
  await mkdir(SHARED, { recursive: true });
  await writeFile(
    path.join(SHARED, '共享说明.txt'),
    text('这个目录是 documents/docs-link 指向的位置。'),
  );
  await writeFile(path.join(SHARED, '团队名单.md'), text('# 团队名单\n\n- 组长：王科瑾'));
  await symlink(SHARED, path.join(SRC, 'docs-link'));

  // 命名管道（FIFO）：不写入数据，只作为特殊文件类型被备份/还原
  execFileSync('mkfifo', [path.join(SRC, 'pipe')]);

  log(`source tree ready in ${((Date.now() - t0) / 1000).toFixed(2)}s`);
}

/** 直接调真实 CLI（种子造备份用；stdout JSONL 原样返回）。 */
export function runCli(args, stdin) {
  return new Promise((resolve, reject) => {
    const t0 = Date.now();
    const child = execFile(
      backendBin,
      ['--json', ...args],
      {
        env: { ...process.env, HOME, XDG_CONFIG_HOME: `${HOME}/.config` },
        maxBuffer: 16 * 1024 * 1024,
      },
      (err, stdout, stderr) => {
        if (err) {
          reject(
            new Error(`databackup ${args.join(' ')} 失败 (code=${err.code}): ${stderr}${stdout}`),
          );
          return;
        }
        resolve({ stdout, ms: Date.now() - t0 });
      },
    );
    child.stdin?.end(stdin === undefined ? undefined : `${stdin}\n`);
  });
}

/** 解析最后一行 result 事件。 */
export function parseResult(stdout) {
  const lines = stdout.trim().split('\n').filter(Boolean);
  for (let i = lines.length - 1; i >= 0; i -= 1) {
    try {
      const e = JSON.parse(lines[i]);
      if (e.type === 'result' || e.type === 'error') return e;
    } catch {
      // 忽略非 JSON 行
    }
  }
  return null;
}

/** 造历史备份用的素材目录（造完即删，让 HOME 只留 documents 与仓库）。 */
async function buildHistorySources() {
  const base = `${HOME}/.seed-src`;
  await rm(base, { recursive: true, force: true });
  const write = async (rel, data) => {
    const p = path.join(base, rel);
    await mkdir(path.dirname(p), { recursive: true });
    await writeFile(p, data);
  };
  const text = (s) => Buffer.from(`${s}\n`.repeat(30));
  // daily：周报/日志类小文本
  await write('daily/week-41.md', text('# 第 41 周\n\n- 联调完成'));
  await write('daily/week-42.md', text('# 第 42 周\n\n- 文档定稿'));
  await write('daily/notes.txt', text('每日构建记录。'));
  // photos：随机二进制"照片"
  for (let i = 1; i <= 3; i += 1) {
    await write(
      `photos/2026-09/IMG_${String(i).padStart(4, '0')}.jpg`,
      pseudoBytes(1_800_000, 40 + i),
    );
  }
  // archive：合同扫描件（加密备份）
  await write('archive/2026/service.pdf', pseudoBytes(700_000, 51));
  await write('archive/2026/nda.pdf', pseudoBytes(420_000, 52));
  await write('archive/readme.txt', text('合同扫描件，按年份归档。'));
  return base;
}

/**
 * 重建 HOME 并预置数据。
 * @param {'empty'|'history'} preset empty=空仓库（S2/S3）；history=4 个备份（S4/S5）
 * @param {object} [opts]
 * @returns {Promise<object>} 预置报告（写进段日志）
 */
export async function seedHome(preset, opts = {}) {
  const t0 = Date.now();
  log(`seed HOME preset=${preset}`);
  await rm(HOME, { recursive: true, force: true });
  await mkdir(HOME, { recursive: true });
  await mkdir(`${HOME}/.config`, { recursive: true });

  const report = {
    preset,
    home: HOME,
    src: SRC,
    repo: REPO,
    timings: {},
    source: {},
    repo_preset: {},
  };

  const tTree = Date.now();
  await buildSourceTree();
  report.timings.buildSourceTreeMs = Date.now() - tTree;

  if (preset === 'history') {
    const tRepo = Date.now();
    const seedSrc = await buildHistorySources();
    const backups = [];
    backups.push({
      name: 'daily',
      r: await runCli([
        'backup',
        '--source',
        `${seedSrc}/daily`,
        '--repo',
        REPO,
        '--name',
        'daily',
        '--compress',
        'huffman',
      ]),
    });
    backups.push({
      name: 'photos',
      r: await runCli([
        'backup',
        '--source',
        `${seedSrc}/photos`,
        '--repo',
        REPO,
        '--name',
        'photos',
        '--compress',
        'huffman',
      ]),
    });
    backups.push({
      name: 'archive',
      r: await runCli(
        [
          'backup',
          '--source',
          `${seedSrc}/archive`,
          '--repo',
          REPO,
          '--name',
          'archive',
          '--compress',
          'huffman',
          '--encrypt',
          'aes256',
          '--password-stdin',
        ],
        PW,
      ),
    });
    // documents-demo：就是 S3 的产物（同一源目录、同一过滤/压缩/加密配置）
    const tDemo = Date.now();
    backups.push({
      name: 'documents-demo',
      r: await runCli(
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
      ),
    });
    report.repo_preset.backupMs = backups.map((b) => ({
      name: b.name,
      ms: b.r.ms,
      result: parseResult(b.r.stdout),
    }));
    report.repo_preset.documentsDemoMs = Date.now() - tDemo;
    await rm(seedSrc, { recursive: true, force: true });
    report.timings.seedRepoMs = Date.now() - tRepo;
  } else {
    // 空仓库：目录都不存在（客户端显示"还没有备份"；列表接口返回空）
    await rm(REPO, { recursive: true, force: true });
    report.repo_preset = { empty: true };
  }

  // 源目录清单统计（文件数 / 字节数 / 目录数），用于报告与校验目标体积
  const tStat = Date.now();
  const stats = await summarizeTree(SRC);
  report.source = stats;
  report.timings.summarizeMs = Date.now() - tStat;
  report.timings.totalMs = Date.now() - t0;

  if (opts.logFile) {
    await mkdir(path.dirname(opts.logFile), { recursive: true });
    await writeFile(
      opts.logFile,
      JSON.stringify({ seededAt: new Date().toISOString(), ...report }, null, 2),
    );
  }
  log(
    `seed done in ${(report.timings.totalMs / 1000).toFixed(1)}s: ` +
      `source files=${stats.files} dirs=${stats.dirs} bytes=${(stats.bytes / 1e6).toFixed(1)}MB ` +
      `(node_modules files=${stats.nodeModulesFiles})`,
  );
  return report;
}

/** 递归统计源目录（用 node 自身，避免依赖 du/find 的差异）。 */
export async function summarizeTree(root) {
  const out = {
    files: 0,
    dirs: 0,
    bytes: 0,
    symlinks: 0,
    fifos: 0,
    nodeModulesFiles: 0,
    entries: 0,
  };
  const walk = async (rel) => {
    const dir = rel === '' ? root : path.join(root, rel);
    let ents;
    try {
      ents = await import('node:fs/promises').then((fs) =>
        fs.readdir(dir, { withFileTypes: true }),
      );
    } catch {
      return;
    }
    for (const e of ents) {
      const r = rel === '' ? e.name : `${rel}/${e.name}`;
      out.entries += 1;
      if (e.isDirectory()) {
        out.dirs += 1;
        await walk(r);
      } else if (e.isSymbolicLink()) {
        out.symlinks += 1;
      } else {
        try {
          const st = await stat(path.join(root, r));
          if (st.isFIFO()) out.fifos += 1;
          else {
            out.files += 1;
            out.bytes += st.size;
            if (r.startsWith('node_modules/')) out.nodeModulesFiles += 1;
          }
        } catch {
          // 忽略
        }
      }
    }
  };
  await walk('');
  return out;
}

/** 该段预置的 HOME 里，备份源目录是否已存在（录制前自检用）。 */
export async function assertSeeded() {
  const st = await stat(SRC).catch(() => null);
  if (!st?.isDirectory()) throw new Error(`源目录不存在：${SRC}`);
}
