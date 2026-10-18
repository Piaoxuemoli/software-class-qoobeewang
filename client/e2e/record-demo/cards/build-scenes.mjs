#!/usr/bin/env node
/* DataBackup 演示视频 · 卡片段场景构建
 *
 * 输入（capture-real-output.sh 在容器内产生）：
 *   course/04-视频/work/cards/raw/commands.txt   真实命令 + 原始 stdout + 退出码
 *   course/04-视频/work/cards/raw/facts.txt      环境/种子/字体等事实
 * 输出：
 *   course/04-视频/work/cards/scenes.json        卡片渲染场景（内容 + 时间轴 + 注解目标）
 *   course/04-视频/work/cards/real-commands.txt  人读证据（命令原文与输出）
 *
 * 硬约束：终端行文本只来自 commands.txt 的 stdout；本脚本只做排版/时间轴，
 * 不在这里补写任何「看起来像输出」的文本。所有出现在画面里的命令行都会校验
 * 「展示文字 == 实际执行的命令」。
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, '../../../..');
const WORK = path.join(REPO, 'course/04-视频/work');
const CARDS = path.join(WORK, 'cards');
const RAW = path.join(CARDS, 'raw');
const SCENES_OUT = path.join(CARDS, 'scenes.json');
const EVIDENCE_OUT = path.join(CARDS, 'real-commands.txt');
const REPORT_OUT = path.join(CARDS, 'build-report.txt');

function die(msg) {
  console.error(`[build-scenes] FATAL: ${msg}`);
  process.exit(1);
}
function info(msg) {
  console.log(`[build-scenes] ${msg}`);
}
function rel(file) {
  return path.relative(REPO, file).split(path.sep).join('/');
}
function readText(file) {
  if (!fs.existsSync(file))
    die(`缺少输入文件 ${path.relative(REPO, file)}（先跑 capture-real-output.sh）`);
  return fs.readFileSync(file, 'utf8');
}

// ---------------------------------------------------------------------------
// 解析 facts.txt（key<TAB>value）
// ---------------------------------------------------------------------------
const facts = {};
for (const line of readText(path.join(RAW, 'facts.txt')).split('\n')) {
  if (!line || line.startsWith('#')) continue;
  const tab = line.indexOf('\t');
  if (tab < 0) continue;
  facts[line.slice(0, tab)] = line.slice(tab + 1);
}

// ---------------------------------------------------------------------------
// 解析 commands.txt
// ---------------------------------------------------------------------------
const blocks = new Map();
{
  const lines = readText(path.join(RAW, 'commands.txt')).split('\n');
  let current = null;
  let mode = null;
  const flush = () => {
    if (!current) return;
    if (current.exec === null) die(`${current.id}: 缺少 ### EXEC 块`);
    if (current.rc === null) die(`${current.id}: 缺少 ### END 块`);
    blocks.set(current.id, current);
    current = null;
    mode = null;
  };
  for (const line of lines) {
    let m = /^### CMD (\S+)$/.exec(line);
    if (m) {
      flush();
      current = { id: m[1], display: [], exec: null, output: [], rc: null };
      mode = 'display';
      continue;
    }
    m = /^### EXEC (\S+)$/.exec(line);
    if (m) {
      if (!current || current.id !== m[1]) die(`### EXEC 与当前块不匹配: ${line}`);
      mode = 'exec';
      continue;
    }
    m = /^### OUT (\S+)$/.exec(line);
    if (m) {
      if (!current || current.id !== m[1]) die(`### OUT 与当前块不匹配: ${line}`);
      mode = 'output';
      continue;
    }
    m = /^### END (\S+) rc=(\d+)$/.exec(line);
    if (m) {
      if (!current || current.id !== m[1]) die(`### END 与当前块不匹配: ${line}`);
      current.rc = Number(m[2]);
      flush();
      continue;
    }
    if (!current) continue;
    if (mode === 'display') current.display.push(line);
    else if (mode === 'exec') current.exec = line;
    else if (mode === 'output') current.output.push(line);
  }
  flush();
}

function block(id) {
  const b = blocks.get(id);
  if (!b) die(`缺少采集块 ${id}`);
  return b;
}

// 展示命令归一化：去 "$ " 前缀、续行反斜杠、多余空白
function normalizeCommand(text) {
  return text
    .replace(/\$/g, ' $ ')
    .replace(/\\/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/^\$ /, '');
}

// 校验「画面里显示的命令 == 真实执行的命令」（续行拼接后逐字一致）
function validateDisplay(b) {
  const shown = normalizeCommand(b.display.join(' '));
  const actual = normalizeCommand(b.exec);
  if (shown !== actual) {
    die(`${b.id}: 画面命令与实际执行的命令不一致\n  shown  = ${shown}\n  actual = ${actual}`);
  }
}

// ---------------------------------------------------------------------------
// 纯文本泄漏扫描（画面/证据里只允许 /home/demo、不允许个人路径/凭据）
// ---------------------------------------------------------------------------
const LEAK_PATTERNS = [
  { name: '宿主个人路径', re: /Qoobeewang|C:\\|C:\/|Desktop/i },
  { name: '非 /home/demo 的用户目录', re: /\/home\/(?!demo(?:\/|$|\s))[A-Za-z0-9._-]+/ },
  { name: '根用户路径', re: /\/root(?:\/|\s|$)/ },
  { name: '真实口令', re: /demo-2026|wrong-demo|password/i },
  { name: '学号/凭据字段', re: /学号|token|api[_-]?key|secret/i },
];
function leakScan(text, where) {
  for (const pattern of LEAK_PATTERNS) {
    if (pattern.re.test(text)) die(`${where}: 命中敏感信息规则「${pattern.name}」`);
  }
}

// ---------------------------------------------------------------------------
// 场景 1：seg-00-title（9s）
// ---------------------------------------------------------------------------
const title = {
  kind: 'title',
  name: 'seg-00-title',
  targetSec: 9,
  recordSec: 10,
  fadeIn: { startMs: 400, durationMs: 300 },
  texts: {
    main: 'DataBackup',
    sub: 'Linux 目录树备份与还原软件',
    foot: '组长：王科瑾 · 电子科技大学《软件开发综合实验》 · v1.0.1',
  },
};

// ---------------------------------------------------------------------------
// 场景 2：seg-01-data（20s）—— 终端卡逐行打字机
// ---------------------------------------------------------------------------
// seg-01 重新配速（Lead P2 反馈：原 4.6s 打完、后面几乎全静止，与 20s 镜头不匹配）
//   0.5–8s   ls -la 的 12 行逐行出现（约 300ms/行，约 4.1s 打完，其余时间闪烁光标）
//   8–10s    find node_modules 命令 + 真实结果
//   10–12.5s ls ~/demo/DataBackup（真实空输出）
//   12.5–20s 静止 + 530ms 方波光标（画面持续有微变化，不会完全静止）
const SEG01_TIMING = {
  startMs: 500,
  lineDelayMs: 300,
  findStartMs: 8000,
  findResultMs: 8400,
  repoStartMs: 10000,
};

function pushDisplayLine(lines, displayLine, atMs) {
  const isCmd = displayLine.startsWith('$ ');
  lines.push({
    text: isCmd ? displayLine.slice(2) : displayLine,
    cls: isCmd ? 'cmd' : 'cont',
    atMs,
  });
}

function buildSeg01Lines() {
  const ls = block('seg01-lsla');
  const find = block('seg01-node-modules');
  const repo = block('seg01-ls-repo');
  for (const item of [ls, find, repo]) validateDisplay(item);
  if (repo.output.length !== 0) die(`仓库应为空（ls 无输出），实际 ${repo.output.length} 行`);
  if (find.output.length !== 1) die(`find node_modules 应输出 1 行，实际 ${find.output.length}`);

  const lines = [];
  let t = SEG01_TIMING.startMs;
  for (const displayLine of ls.display) {
    pushDisplayLine(lines, displayLine, t);
    t += SEG01_TIMING.lineDelayMs;
  }
  for (const outputLine of ls.output) {
    lines.push({ text: outputLine, cls: 'out', atMs: t });
    t += SEG01_TIMING.lineDelayMs;
  }

  t = SEG01_TIMING.findStartMs;
  for (const displayLine of find.display) {
    pushDisplayLine(lines, displayLine, t);
    t += SEG01_TIMING.lineDelayMs;
  }
  t = Math.max(t, SEG01_TIMING.findResultMs);
  for (const outputLine of find.output) {
    lines.push({ text: outputLine, cls: 'out', atMs: t });
    t += SEG01_TIMING.lineDelayMs;
  }

  t = SEG01_TIMING.repoStartMs;
  for (const displayLine of repo.display) {
    pushDisplayLine(lines, displayLine, t);
    t += SEG01_TIMING.lineDelayMs;
  }
  return lines;
}

const seg01Lines = buildSeg01Lines();
const seg01Out = (id) => block(id).output;
const seg01Lsla = seg01Out('seg01-lsla');
const seg01NodeModules = seg01Out('seg01-node-modules');
const seg01Repo = seg01Out('seg01-ls-repo');

if (seg01Lsla.length < 8) die(`seg01-lsla 输出行数异常: ${seg01Lsla.length}`);
for (const mustHave of ['docs-link -> /home/demo/shared', 'pipe', '代码', '合同', '报告', '资料']) {
  if (!seg01Lsla.some((line) => line.includes(mustHave))) die(`seg01-lsla 输出缺少「${mustHave}」`);
}
if (seg01NodeModules.length !== 1 || !seg01NodeModules[0].endsWith('/node_modules')) {
  die(`seg01-node-modules 输出异常: ${JSON.stringify(seg01NodeModules)}`);
}
if (seg01Repo.length !== 0) die(`仓库应为空（ls 无输出），实际 ${seg01Repo.length} 行`);
if (block('seg01-ls-repo').rc !== 0) die('seg01-ls-repo 退出码非 0');

function findLineIndex(lines, predicate, label) {
  const index = lines.findIndex(predicate);
  if (index < 0) die(`找不到注解目标行：${label}`);
  return index;
}

const seg01Targets = [
  {
    name: 'node_modules',
    lineIndices: [
      findLineIndex(
        seg01Lines,
        (line) => line.cls === 'out' && line.text.includes('node_modules'),
        'node_modules',
      ),
    ],
    match: 'node_modules',
    annotationWindowMs: [7000, 10000],
    visibleWindowMs: [8400, 20000],
    note: 'find 结果行 8.4s 出现，落在分镜注解窗口 7.0–10.0s 内',
  },
  {
    name: 'docs-link',
    lineIndices: [
      findLineIndex(
        seg01Lines,
        (line) => line.cls === 'out' && line.text.includes('docs-link -> /home/demo/shared'),
        'docs-link',
      ),
    ],
    match: 'docs-link',
    annotationWindowMs: [10500, 13000],
    visibleWindowMs: [2000, 20000],
    note: 'ls -la 输出行 2.0s 出现，注解窗口 10.5–13.0s 内已显示',
  },
  {
    name: 'pipe',
    lineIndices: [
      findLineIndex(
        seg01Lines,
        (line) => line.cls === 'out' && /(^|\s)pipe$/.test(line.text),
        'pipe',
      ),
    ],
    match: 'pipe',
    annotationWindowMs: [13500, 16500],
    visibleWindowMs: [2600, 20000],
    note: 'ls -la 输出行 2.6s 出现，注解窗口 13.5–16.5s 内已显示',
  },
];

const lastSeg01Ms = seg01Lines[seg01Lines.length - 1].atMs;
if (lastSeg01Ms > 20 * 1000 - 500) die(`seg-01 打字机结束太晚: ${lastSeg01Ms}ms`);

const seg01 = {
  kind: 'terminal',
  name: 'seg-01-data',
  targetSec: 20,
  recordSec: 21,
  lines: seg01Lines,
  targets: seg01Targets,
};

// ---------------------------------------------------------------------------
// 场景 3：seg-06-verify（24s）—— 终端 → 数据卡 → 谢谢 → 淡出
// ---------------------------------------------------------------------------
// 内容优先级：course/04-视频/work/seg/actual-run/ 里 app-recorder 正式录 seg-03/05 时捕获的
// 真实输出（与 S3 完成卡同一次运行 → 满足 07-…§9「完成卡数字与终端输出一致」）；
// 该目录不存在或文件不全时退回本次 capture 的自采结果，并在 provenance 里明确标注 fallback。
// actual-run 产物根目录：app-recorder 正式录制 seg-03/seg-05 后落盘到 seg/actual-run/；
// 彩排产物在 rehearsal/actual-run/（只用于识别，mode=rehearsal 时不用，避免彩排数字进成片）。
const ACTUAL_RUN_ROOTS = [
  path.join(WORK, 'seg/actual-run'),
  path.join(WORK, 'rehearsal/actual-run'),
];

const CANONICAL_SEG06 = {
  diffDisplay: [
    '$ diff -r ~/demo/documents ~/demo/documents-restored \\',
    '      --exclude node_modules --exclude pipe',
  ],
  fifoDisplay: ['$ test -p ~/demo/documents-restored/pipe && echo "FIFO 类型一致"'],
  lsDisplay: ['$ ls -l ~/demo/DataBackup/*.dbk'],
};

function readTextIfExists(file) {
  return file && fs.existsSync(file) ? fs.readFileSync(file, 'utf8') : null;
}

// actual-run 产物是带分节的 transcript，例如：
//   # ls -l ~/demo/DataBackup/*.dbk
//   # 命令: ...
//   # 退出码: 0
//   --- stdout ---
//   -rw------- 1 root root 287017980 Oct 18 08:22 /home/demo/DataBackup/documents-demo.dbk
//   --- stderr ---
//   (空)
// 这里只取 stdout（或「输出」）分节里的真实行；"(空)" 视为空输出；# 开头是注释/判读。
function parseActualRunTranscript(text) {
  const lines = text.replace(/\r\n/g, '\n').replace(/\r/g, '\n').split('\n');
  const sections = {};
  let current = null;
  for (const line of lines) {
    const marker = /^---\s*(stdout|输出|stderr)\s*---\s*$/.exec(line);
    if (marker) {
      current = marker[1] === '输出' ? 'output' : marker[1];
      sections[current] = sections[current] || [];
      continue;
    }
    if (!current) continue;
    const trimmed = line.trim();
    if (trimmed === '' || trimmed === '(空)' || line.startsWith('#')) continue;
    sections[current].push(line.trimEnd());
  }
  const output = [...(sections.stdout || []), ...(sections.output || [])];
  return { sections, output };
}

function listFilesRecursive(root, depth = 0, out = []) {
  let entries;
  try {
    entries = fs.readdirSync(root, { withFileTypes: true });
  } catch {
    return out;
  }
  for (const entry of entries) {
    const full = path.join(root, entry.name);
    if (entry.isDirectory()) {
      if (depth < 3) listFilesRecursive(full, depth + 1, out);
    } else {
      out.push(full);
    }
  }
  return out;
}

function readRunMode(jsonFiles) {
  for (const file of jsonFiles) {
    try {
      const data = JSON.parse(fs.readFileSync(file, 'utf8'));
      const mode = data && data.run && data.run.mode;
      if (mode) return { mode, file };
    } catch {
      // 不是 stats JSON，跳过
    }
  }
  return { mode: 'unknown', file: null };
}

function collectNumbers(value, out = []) {
  if (typeof value === 'number') out.push(value);
  else if (typeof value === 'string') {
    for (const match of value.matchAll(/\d[\d,_ ]*\d|\d/g)) {
      const parsed = Number(match[0].replace(/[_, ]/g, ''));
      if (Number.isFinite(parsed)) out.push(parsed);
    }
  } else if (value && typeof value === 'object') {
    for (const item of Object.values(value)) collectNumbers(item, out);
  }
  return out;
}

function lsDbkBytes(line) {
  const match = /^[^ ]+\s+\d+\s+\S+\s+\S+\s+(\d+)\s+/.exec(line || '');
  return match ? Number(match[1]) : null;
}

function loadActualRunSeg06() {
  const candidates = [];
  for (const root of ACTUAL_RUN_ROOTS) {
    if (!fs.existsSync(root)) continue;
    const files = listFilesRecursive(root);
    // 按 patterns 的先后顺序挑文件（越靠前优先级越高），而不是按文件系统顺序
    const pick = (patterns) => {
      for (const pattern of patterns) {
        const hit = files.find((file) => pattern.test(path.basename(file)));
        if (hit) return hit;
      }
      return null;
    };
    const diffFile = pick([/seg05.*diff/i, /diff/i]);
    const fifoFile = pick([/fifo/i]);
    // Lead 定稿：S6 用 seg-05 的 4 行真实仓库列表（S5 之后仓库就是 documents-demo + 3 个历史备份）；
    // 没有 seg05 时退回 seg03（只有本次 documents-demo.dbk，与分镜 §S6 单行示例一致）。
    const lsFile = pick([/seg05.*ls.*dbk/i, /seg03.*ls.*dbk/i, /ls.*dbk/i]);
    if (!diffFile || !fifoFile || !lsFile) continue;
    const jsonFiles = files.filter((file) => file.endsWith('.json'));
    candidates.push({
      root,
      diffFile,
      fifoFile,
      lsFile,
      jsonFiles,
      runMode: readRunMode(jsonFiles),
    });
  }
  if (!candidates.length) {
    return {
      source: 'own-capture-fallback',
      reason: 'seg/actual-run/ 还没有完整的 diff/fifo/ls 产物',
    };
  }
  // 优先正式（非 rehearsal）产物；只有彩排产物时不采用
  candidates.sort(
    (a, b) => (a.runMode.mode === 'rehearsal' ? 1 : 0) - (b.runMode.mode === 'rehearsal' ? 1 : 0),
  );
  const chosen = candidates[0];
  if (chosen.runMode.mode === 'rehearsal') {
    return {
      source: 'own-capture-fallback',
      reason: `actual-run 目前只有彩排产物（${rel(chosen.root)}），等正式录制产物`,
    };
  }

  const diff = parseActualRunTranscript(readTextIfExists(chosen.diffFile));
  const fifo = parseActualRunTranscript(readTextIfExists(chosen.fifoFile));
  const ls = parseActualRunTranscript(readTextIfExists(chosen.lsFile));
  if (diff.output.length !== 0) {
    die(
      `actual-run 的 diff 输出不是空（${diff.output.length} 行）——请确认 app-recorder 用的是 ` +
        `「--exclude node_modules --exclude pipe」且 rc=0：\n${diff.output.join('\n')}`,
    );
  }
  const fifoLine = fifo.output.find((line) => line.includes('FIFO'));
  if (!fifoLine) die(`actual-run 的 FIFO 校验输出异常: ${JSON.stringify(fifo.output)}`);
  const dbkRows = ls.output.filter((line) => /\.dbk\s*$/.test(line.trim()));
  if (!dbkRows.length) die(`actual-run 的 ls -l *.dbk 输出异常: ${JSON.stringify(ls.output)}`);

  // 数字一致性：本次备份行（documents-demo）的字节数应出现在 stats JSON 里（允许 0.1% 漂移）
  const docRow =
    dbkRows.find((line) => line.includes('documents-demo')) || dbkRows[dbkRows.length - 1];
  const lsBytes = lsDbkBytes(docRow);
  let statsCheck = 'stats-json-missing';
  if (chosen.jsonFiles.length) {
    const numbers = chosen.jsonFiles.flatMap((file) => {
      try {
        return collectNumbers(JSON.parse(fs.readFileSync(file, 'utf8')));
      } catch {
        return [];
      }
    });
    const hit =
      lsBytes !== null &&
      numbers.some((n) => n === lsBytes || (n > 0 && Math.abs(n - lsBytes) / lsBytes < 0.001));
    statsCheck = hit
      ? `ok(${chosen.jsonFiles.map((file) => path.basename(file)).join(', ')})`
      : `未在 stats JSON 里找到与 ls 行一致的字节数 ${lsBytes}`;
    if (!hit) console.warn(`[build-scenes] WARN: ${statsCheck}`);
  }
  return {
    source: 'actual-run',
    reason: null,
    dir: rel(chosen.root),
    runMode: chosen.runMode.mode,
    files: {
      diff: rel(chosen.diffFile),
      fifo: rel(chosen.fifoFile),
      ls: rel(chosen.lsFile),
      stats: chosen.jsonFiles.map(rel),
    },
    diffDisplay: CANONICAL_SEG06.diffDisplay,
    fifoDisplay: CANONICAL_SEG06.fifoDisplay,
    lsDisplay: CANONICAL_SEG06.lsDisplay,
    diffOutput: diff.output,
    fifoOutput: [fifoLine],
    lsOutput: dbkRows,
    lsBytes,
    statsCheck,
  };
}

function loadOwnCaptureSeg06() {
  const diffBlock = block('seg06-diff');
  const fifoBlock = block('seg06-fifo-check');
  const lsBlock = block('seg06-ls-dbk');
  validateDisplay(diffBlock);
  validateDisplay(fifoBlock);
  validateDisplay(lsBlock);
  if (diffBlock.output.length !== 0)
    die(`seg06-diff 应为真实无输出，实际 ${diffBlock.output.length} 行`);
  if (diffBlock.rc !== 0) die(`seg06-diff 退出码应为 0，实际 ${diffBlock.rc}`);
  if (!fifoBlock.output.some((line) => line.includes('FIFO'))) {
    die(`seg06-fifo-check 输出异常: ${JSON.stringify(fifoBlock.output)}`);
  }
  if (lsBlock.output.length !== 1)
    die(`seg06-ls-dbk 应输出 1 行，实际 ${lsBlock.output.length} 行`);
  if (!/\.dbk$/.test(lsBlock.output[0].trim()))
    die(`seg06-ls-dbk 输出不是 .dbk 文件: ${lsBlock.output[0]}`);
  return {
    source: 'own-capture-fallback',
    reason: 'seg/actual-run/ 暂不可用',
    dir: rel(RAW),
    files: {
      diff: 'raw/commands.txt#seg06-diff',
      fifo: 'raw/commands.txt#seg06-fifo-check',
      ls: 'raw/commands.txt#seg06-ls-dbk',
      stats: null,
    },
    diffDisplay: diffBlock.display,
    fifoDisplay: fifoBlock.display,
    lsDisplay: lsBlock.display,
    diffOutput: diffBlock.output,
    fifoOutput: fifoBlock.output,
    lsOutput: lsBlock.output,
    lsBytes: lsDbkBytes(lsBlock.output[0]),
    statsCheck: 'fallback(own-capture)',
  };
}

let seg06Src = loadOwnCaptureSeg06();
seg06Src = { ...seg06Src, ...loadActualRunSeg06() };
if (seg06Src.source !== 'actual-run') info(`seg-06 使用 fallback 自采数据：${seg06Src.reason}`);

const seg06Lines = [];
{
  let t = 300;
  for (const displayLine of seg06Src.diffDisplay) {
    const isCmd = displayLine.startsWith('$ ');
    seg06Lines.push({
      text: isCmd ? displayLine.slice(2) : displayLine,
      cls: isCmd ? 'cmd' : 'cont',
      atMs: t,
    });
    t += 120;
  }
  if (seg06Src.diffOutput.length) die('seg-06 diff 必须无输出（真实 diff 结果）');
  t = 2000;
  for (const displayLine of seg06Src.fifoDisplay) {
    const isCmd = displayLine.startsWith('$ ');
    seg06Lines.push({
      text: isCmd ? displayLine.slice(2) : displayLine,
      cls: isCmd ? 'cmd' : 'cont',
      atMs: t,
    });
    t += 120;
  }
  for (const outputLine of seg06Src.fifoOutput) {
    seg06Lines.push({ text: outputLine, cls: 'out', atMs: t });
    t += 120;
  }
  t = 3400;
  for (const displayLine of seg06Src.lsDisplay) {
    const isCmd = displayLine.startsWith('$ ');
    seg06Lines.push({
      text: isCmd ? displayLine.slice(2) : displayLine,
      cls: isCmd ? 'cmd' : 'cont',
      atMs: t,
    });
    t += 120;
  }
  for (const outputLine of seg06Src.lsOutput) {
    seg06Lines.push({ text: outputLine, cls: 'out', atMs: t });
    t += 120;
  }
}

// 终端块行索引：diff 命令 2 行 → FIFO 校验（命令+输出）→ ls（命令 + 全部 .dbk 输出行）
const diffLineIndices = [0, 1];
const fifoLineIndices = [2, 3];
const lsOutIndexes = seg06Lines
  .map((line, index) => ({ line, index }))
  .filter(({ line }) => line.cls === 'out' && /\.dbk\s*$/.test(line.text.trim()))
  .map(({ index }) => index);
if (!lsOutIndexes.length) die('seg-06 找不到 ls *.dbk 的输出行');
const lsLineIndices = [Math.min(...lsOutIndexes) - 1, ...lsOutIndexes];
if (diffLineIndices.length !== 2 || fifoLineIndices.length !== 2 || lsLineIndices.length < 2) {
  die('seg-06 终端块行索引异常');
}
const docRowIndex = seg06Lines.findIndex(
  (line) => line.cls === 'out' && line.text.includes('documents-demo'),
);

// 注解目标（P3 用 ASS 画框）：box 由彩排实测；visibleWindowMs 是该块在卡片上真实可见的时间窗，
// annotationWindowMs 来自 video-storyboard.json（P3 已修正 S6 注解窗口到终端相位内）。
const seg06Targets = [
  {
    name: 'terminal-diff',
    lineIndices: diffLineIndices,
    annotationWindowMs: [2000, 5200],
    visibleWindowMs: [300, 8000],
    note: 'diff 两条命令行（P3 修正后注解 2.0–5.2s）；终端卡 0–8s 显示',
  },
  {
    name: 'fifo-check',
    lineIndices: fifoLineIndices,
    annotationWindowMs: null,
    visibleWindowMs: [2000, 8000],
    note: 'FIFO 类型一致性校验（Lead P2 更正 #1 加入；分镜暂无此注解，可选）',
  },
  {
    name: 'terminal-ls',
    lineIndices: lsLineIndices,
    annotationWindowMs: [5400, 8000],
    visibleWindowMs: [3400, 8000],
    note: 'ls -l *.dbk 命令+全部输出行（P3 修正后注解 5.4–8.0s）；最后一行 3.88s 已出现，远早于 5.3s',
  },
  ...(seg06Src.lsOutput.length > 1 && docRowIndex >= 0
    ? [
        {
          name: 'dbk-documents-demo',
          lineIndices: [docRowIndex],
          annotationWindowMs: [5400, 8000],
          visibleWindowMs: [3760, 8000],
          note: '本次运行的单个备份产物 documents-demo.dbk（其余 .dbk 行是 seg-04 预置的历史备份）；P3 要强调「单个备份文件」时可框这一行',
        },
      ]
    : []),
];

const seg06 = {
  kind: 'mixed',
  name: 'seg-06-verify',
  targetSec: 24,
  recordSec: 25,
  timeline: {
    terminalStartMs: 0,
    terminalEndMs: 8000,
    statsStartMs: 8000,
    statsEndMs: 16000,
    thanksStartMs: 16000,
    thanksEndMs: 22000,
    fadeStartMs: 22000,
    fadeEndMs: 24000,
  },
  terminal: { lines: seg06Lines },
  targets: seg06Targets,
  stats: {
    items: [
      { value: 'FR-01~18', label: '功能需求' },
      { value: '32', label: '过滤 / 压缩 / 加密组合' },
      { value: '400+', label: '自动化测试用例' },
    ],
  },
  thanks: { main: '谢谢', byline: '组长：王科瑾' },
};

// ---------------------------------------------------------------------------
// 泄漏扫描（画面里会出现的全部文本）
// ---------------------------------------------------------------------------
for (const line of [...seg01Lines, ...seg06Lines]) {
  leakScan(line.text, `终端行「${line.text.slice(0, 40)}」`);
}
leakScan(title.texts.main + title.texts.sub + title.texts.foot, 'seg-00 文本');
leakScan(seg06.stats.items.map((i) => i.value + i.label).join(''), 'seg-06 数据卡');
leakScan(seg06.thanks.main + seg06.thanks.byline, 'seg-06 片尾');

const scenes = {
  version: 1,
  generatedAt: new Date().toISOString(),
  generator: 'client/e2e/record-demo/cards/build-scenes.mjs',
  video: { width: 1920, height: 1080, fps: 30 },
  fonts: {
    sans: 'Noto Sans CJK SC',
    mono: 'DejaVu Sans Mono',
    monoCjkFallback: 'Noto Sans Mono CJK SC',
  },
  seed: {
    demoUser: `demo(uid ${facts.demo_uid || '1000'})`,
    homeForTilde: facts.home_for_tilde || '/home',
    dataRoot: '/home/demo',
    source: facts.source || '/home/demo/documents',
    repo: '/home/demo/DataBackup',
    restore: '/home/demo/documents-restored',
    excludedDir: facts.excluded_dir || '代码/site/node_modules',
    backupName: facts.backup_name || 'documents-demo.dbk',
    compress: facts.backup_compress || 'huffman',
    encrypt: facts.backup_encrypt || 'aes256',
  },
  provenance: {
    capturedAt: facts.captured_at || null,
    backendVersion: facts.backend_version || null,
    sourceHuman: facts.source_human || null,
    sourceFiles: facts.source_files || null,
    dbkHuman: facts.dbk_human || null,
    backupSeconds: facts.backup_seconds || null,
    restoreSeconds: facts.restore_seconds || null,
    evidence: 'course/04-视频/work/cards/real-commands.txt',
    seg06Source: {
      source: seg06Src.source,
      reason: seg06Src.reason || null,
      dir: seg06Src.dir,
      runMode: seg06Src.runMode || null,
      files: seg06Src.files,
      statsCheck: seg06Src.statsCheck,
      lsBytes: seg06Src.lsBytes,
      rows: seg06Src.lsOutput ? seg06Src.lsOutput.length : null,
      note:
        seg06Src.source === 'actual-run'
          ? 'seg-06 终端内容取自 app-recorder 正式录制 seg-03/05 时捕获的真实产物（与 S3 完成卡同一次运行；ls -l 显示 root root 是因为容器内客户端以 root 运行，未改写）'
          : 'seg-06 终端内容退回本脚本自采数据（未拿到 actual-run）；与客户端 S3 完成卡的数字可能不同，需 Lead 确认',
    },
  },
  segments: [title, seg01, seg06],
};

fs.mkdirSync(CARDS, { recursive: true });
fs.writeFileSync(SCENES_OUT, `${JSON.stringify(scenes, null, 2)}\n`);

// ---------------------------------------------------------------------------
// 人读证据 real-commands.txt
// ---------------------------------------------------------------------------
const evidence = [];
evidence.push('DataBackup 演示视频 · 卡片段终端内容真实采集证据');
evidence.push(`生成时间: ${new Date().toISOString()}`);
evidence.push(
  `生成脚本: client/e2e/record-demo/cards/build-scenes.mjs（数据来自 capture-real-output.sh 的 raw/commands.txt）`,
);
evidence.push('');
evidence.push('== 环境与种子（raw/facts.txt） ==');
for (const [key, value] of Object.entries(facts)) evidence.push(`${key}\t= ${value}`);
evidence.push('');
evidence.push('== 真实命令与原始输出（画面里的每一行终端文本都来自这里） ==');
for (const id of [
  'seg01-lsla',
  'seg01-node-modules',
  'seg01-ls-repo',
  'seg06-diff-strict',
  'seg06-diff',
  'seg06-fifo-check',
  'seg06-ls-dbk',
  'backup',
  'restore',
]) {
  const b = blocks.get(id);
  if (!b) continue;
  evidence.push('');
  evidence.push(`--- ${id} (exit ${b.rc}) ---`);
  evidence.push('展示用命令行:');
  for (const line of b.display) evidence.push(line);
  evidence.push('实际执行:');
  evidence.push(b.exec);
  evidence.push('stdout:');
  if (b.output.length === 0) evidence.push('(无输出)');
  else for (const line of b.output) evidence.push(line);
}
evidence.push('');
evidence.push('== 说明 ==');
evidence.push(
  '- GNU diffutils 3.8 对 FIFO 恒输出 "File ... is a fifo while file ... is a fifo"（即使两侧都是 FIFO），',
);
evidence.push(
  '  所以视频里的 diff 在分镜的 --exclude node_modules 之外再加 --exclude pipe，才是真实「无输出」。',
);
evidence.push(
  '- FIFO 还原正确性用 `test -p .../pipe && echo "FIFO 类型一致"` 单独证明（seg06-fifo-check 块）。',
);
evidence.push(
  '- diff 的源目录不含 node_modules（备份时被排除），因此 --exclude node_modules 必需。',
);
evidence.push(
  '- seg-06 最终画面优先使用 course/04-视频/work/seg/actual-run/ 里客户端段正式录制的真实产物；',
);
evidence.push(
  `  本次构建来源: ${seg06Src.source}${seg06Src.reason ? `（${seg06Src.reason}）` : ''}。`,
);
evidence.push(
  '- 容器内运行用户 demo 的 HOME 记为 /home，使画面命令里的 ~/demo/... 展开为 /home/demo/...，与分镜命令逐字一致。',
);
fs.writeFileSync(EVIDENCE_OUT, `${evidence.join('\n')}\n`);

// ---------------------------------------------------------------------------
// 构建报告
// ---------------------------------------------------------------------------
const report = [];
report.push(`generatedAt: ${new Date().toISOString()}`);
report.push(
  `segments: ${scenes.segments.map((s) => `${s.name}=${s.targetSec}s(+1s 余量)`).join(', ')}`,
);
report.push(
  `seg-01: ${seg01Lines.length} 行，最后一行 ${lastSeg01Ms}ms（打字机约 ${(lastSeg01Ms / 1000).toFixed(1)}s，其余为静态展示）`,
);
report.push(
  `seg-01 targets: ${seg01Targets.map((t) => `${t.name}@line${t.lineIndices[0]}`).join(', ')}`,
);
report.push(
  `seg-06: ${seg06Lines.length} 行终端；phases ` +
    Object.entries(seg06.timeline)
      .map(([k, v]) => `${k}=${v}`)
      .join(' '),
);
report.push(
  `seg-06 targets: ${seg06Targets.map((t) => `${t.name}@lines${t.lineIndices.join('+')}`).join(', ')}`,
);
report.push(
  `seg-06 终端: diff 无输出 + FIFO 校验（${JSON.stringify(seg06Src.fifoOutput)}）+ ls -l *.dbk（${seg06Src.lsOutput.length} 行）`,
);
report.push(
  `seg-06 数据来源: ${seg06Src.source}${seg06Src.runMode ? `/${seg06Src.runMode}` : ''}（${seg06Src.reason || '与客户端段同一次运行'}），ls 字节数=${seg06Src.lsBytes}，stats 核对=${seg06Src.statsCheck}`,
);
if (seg06Src.lsOutput)
  report.push(
    `seg-06 ls -l 输出 ${seg06Src.lsOutput.length} 行（含本次 documents-demo 与历史备份），FIFO 行=${JSON.stringify(seg06Src.fifoOutput)}`,
  );
if (seg06Src.files) report.push(`seg-06 源文件: ${JSON.stringify(seg06Src.files)}`);
report.push(
  `字体: sans=${scenes.fonts.sans} mono=${scenes.fonts.mono}（容器 fc-list 实测，见 facts.mono_fonts）`,
);
report.push(`scenes.json: ${path.relative(REPO, SCENES_OUT)}`);
report.push(`evidence: ${path.relative(REPO, EVIDENCE_OUT)}`);
fs.writeFileSync(REPORT_OUT, `${report.join('\n')}\n`);

// 终端摘要
info('scenes.json 已生成');
for (const line of report) info(line);
info(`seg-01 行预览:`);
for (const line of seg01Lines)
  info(`  +${String(line.atMs).padStart(5)}ms [${line.cls}] ${line.text}`);
info(`seg-06 行预览:`);
for (const line of seg06Lines)
  info(`  +${String(line.atMs).padStart(5)}ms [${line.cls}] ${line.text}`);
