/**
 * 答辩 PPT 生成脚本（M9-3）：docs/deliverables/05-答辩PPT.md → docs/ppt/答辩PPT.pptx
 *
 * 设计约定（与 05-答辩PPT.md 一致）：
 * - 白底黑字、细线分隔、单一深绿强调色；不使用大号数字卖点、渐变或装饰性图形。
 * - 每页一个论点：标题陈述结论，正文给事实与出处（文档 §、测试编号）。
 * - 图只用项目自有产物：架构图、真实客户端截图。
 *
 * 依赖：pptxgenjs 只用于构建期，不进入代码仓库。用 `scripts/docs/build-ppt.sh`
 * 调用即可（它把依赖装到 gitignored 的 build/ppt-deps 并设置 NODE_PATH）；
 * 手工运行等价于：
 *   npm i --prefix build/ppt-deps pptxgenjs@4
 *   NODE_PATH=build/ppt-deps/node_modules node scripts/docs/pptx-build.mjs
 * 在仓库根执行（图片路径相对仓库根）。
 */
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const pptxgen = require('pptxgenjs');

const p = new pptxgen();
p.layout = 'LAYOUT_WIDE';
p.author = '王科瑾';
p.title = 'DataBackup 答辩';
p.subject = 'Linux 目录树备份与还原软件';

const W = 13.333;
const H = 7.5;
const M = 0.75;
const CW = W - 2 * M;
const F = '微软雅黑';
const MONO = 'Consolas';
const C = {
  ink: '1A1D1F',
  body: '33383D',
  muted: '6B7280',
  faint: '9CA3AF',
  rule: 'E3E6E8',
  tint: 'F6F7F7',
  accent: '14584B',
  accentTint: 'EAF1EE',
  danger: 'B42318',
};
const IMG = 'docs/deliverables/diagrams/';
const SHOT = 'docs/images/client/';
const TOTAL = 8;

/** 页眉：标题 + 可选眉标 + 细线 + 页脚。 */
function page(title, kicker) {
  const s = p.addSlide();
  s.background = { color: 'FFFFFF' };
  s.addText(title, {
    x: M, y: 0.5, w: CW - 1.2, h: 0.52, fontSize: 25, bold: true, fontFace: F,
    color: C.ink, margin: 0, valign: 'middle',
  });
  if (kicker) {
    s.addText(kicker, {
      x: W - M - 2.0, y: 0.5, w: 2.0, h: 0.52, fontSize: 10, fontFace: F,
      color: C.faint, align: 'right', valign: 'middle', margin: 0,
    });
  }
  s.addShape(p.shapes.LINE, {
    x: M, y: 1.16, w: CW, h: 0, line: { color: C.rule, width: 1 },
  });
  s.addText('DataBackup · 软件开发综合实验', {
    x: M, y: H - 0.46, w: 6, h: 0.26, fontSize: 9, fontFace: F, color: C.faint, margin: 0,
  });
  return s;
}

/** 页脚页码（在 addSlide 之后统一补，避免 _slideNum 未初始化）。 */
function number(s, n) {
  s.addText(`${n} / ${TOTAL}`, {
    x: W - M - 1.0, y: H - 0.46, w: 1.0, h: 0.26, fontSize: 9, fontFace: F,
    color: C.faint, align: 'right', margin: 0,
  });
}

const text = (s, str, x, y, w, h, size = 12, opts = {}) =>
  s.addText(str, {
    x, y, w, h, fontSize: size, fontFace: F, color: C.body, margin: 0,
    valign: 'top', lineSpacingMultiple: 1.25, ...opts,
  });

/** 小标题（正文内的分组标签）。 */
function label(s, str, x, y, w = 3) {
  s.addText(str, {
    x, y, w, h: 0.28, fontSize: 10.5, bold: true, fontFace: F, color: C.accent, margin: 0,
  });
}

/** 逐条要点：短横线 + 正文，避免项目符号带来的模板感。 */
function points(s, items, x, y, w, gap = 0.52, size = 12) {
  items.forEach((it, i) => {
    const [head, rest] = Array.isArray(it) ? it : [null, it];
    s.addShape(p.shapes.LINE, {
      x, y: y + i * gap + 0.11, w: 0.16, h: 0, line: { color: C.accent, width: 1.25 },
    });
    s.addText(
      head
        ? [
            { text: `${head}  `, options: { bold: true, color: C.ink } },
            { text: rest, options: { color: C.body } },
          ]
        : rest,
      {
        x: x + 0.26, y: y + i * gap, w: w - 0.26, h: gap, fontSize: size, fontFace: F,
        margin: 0, valign: 'top', lineSpacingMultiple: 1.2,
      },
    );
  });
}

/** 表格：无底色、细横线、表头浅灰。 */
function table(s, head, rows, x, y, w, colW, opts = {}) {
  const body = [
    head.map((h) => ({
      text: h,
      options: { bold: false, color: C.muted, fontSize: 9.5, fill: { color: C.tint } },
    })),
    ...rows.map((r) =>
      r.map((c, i) => ({
        text: String(c),
        options: { color: i === 0 ? C.ink : C.body, fontSize: opts.size ?? 10.5 },
      })),
    ),
  ];
  s.addTable(body, {
    x, y, w, colW,
    fontFace: F, border: { type: 'solid', color: C.rule, pt: 0.5 },
    rowH: opts.rowH ?? 0.36, valign: 'middle', margin: 0.06,
    autoPage: false,
  });
}

/** 图片：等比放进给定框内（只用于项目自有图）。 */
function image(s, file, x, y, w, h) {
  s.addImage({ path: file, x, y, w, h, sizing: { type: 'contain', w, h } });
}

function caption(s, str, x, y, w) {
  s.addText(str, { x, y, w, h: 0.26, fontSize: 9, fontFace: F, color: C.faint, margin: 0 });
}

// ── P1 封面 ────────────────────────────────────────────────────────────────
{
  const s = p.addSlide();
  s.background = { color: 'FFFFFF' };
  s.addShape(p.shapes.RECTANGLE, { x: 0, y: 0, w: 0.16, h: H, fill: { color: C.accent } });
  s.addText('DataBackup', {
    x: 1.1, y: 1.85, w: 10, h: 0.9, fontSize: 48, bold: true, fontFace: F, color: C.ink, margin: 0,
  });
  s.addText('Linux 目录树备份与还原软件', {
    x: 1.12, y: 2.85, w: 10, h: 0.45, fontSize: 22, fontFace: F, color: C.accent, margin: 0,
  });
  s.addShape(p.shapes.LINE, {
    x: 1.12, y: 3.5, w: 5.2, h: 0, line: { color: C.rule, width: 1 },
  });
  s.addText([
    { text: '《软件开发综合实验》  ·  组长：王科瑾（单人组）  ·  v1.0.1', options: { breakLine: true } },
    { text: '答辩 2026-11-18 上午', options: { breakLine: true } },
    { text: '本片 3 分钟。随后 7 分钟，请老师完成一次备份和还原。', options: {} },
  ], { x: 1.12, y: 3.75, w: 10, h: 1.3, fontSize: 16, fontFace: F, color: C.body, margin: 0 });
}

// ── P2 开发环境 ────────────────────────────────────────────────────────────
{
  const s = page('开发环境锁在 Ubuntu 镜像里', '环境');
  table(s,
    ['项', '事实'],
    [
      ['操作系统', '目标 Ubuntu 22.04 / 24.04。Windows 只编辑；构建和测试在 Docker 或 WSL2'],
      ['编译环境', 'Go 1.27.1，Node.js 22，GNU Make'],
      ['环境锁定', 'docker/dev.Dockerfile；版本由 scripts/install-deps.sh 锁定'],
      ['门禁', 'bash scripts/docker.sh check；CI 为 GitHub ubuntu-22.04'],
    ],
    M, 1.55, CW, [2.2, CW - 2.2], { rowH: 0.55, size: 14 });
  number(s, 2);
}

// ── P3 扩展功能 ────────────────────────────────────────────────────────────
{
  const s = page('扩展功能自己实现，不靠第三方库', '扩展');
  table(s,
    ['扩展', '实现', '第三方库'],
    [
      ['类型与元数据', 'lstat 采集，按类型重建', '不用'],
      ['六维过滤', '自研 filter', '不用'],
      ['打包', '自研 MyPack，512 字节头', '不用'],
      ['压缩', '自研 Huffman，1 MiB 分块', '不用'],
      ['加密', 'AES-256-CBC + PBKDF2 + HMAC', '不用'],
      ['图形界面', 'Electron 只 spawn CLI', '不实现扩展算法'],
    ],
    M, 1.4, CW, [2.3, 6.4, 2.9], { rowH: 0.42, size: 13 });
  text(s, 'scripts/check-imports.sh 拦住 archive、compress、crypto/aes 等导入。实时备份和网络备份没有做。',
    M, 6.35, CW, 0.4, 13, { color: C.muted });
  number(s, 3);
}

// ── P4 配置管理 ────────────────────────────────────────────────────────────
{
  const s = page('代码库、分支和规范都在仓库里', '配置管理');
  points(s, [
    ['代码库', 'GitHub Piaoxuemoli/software-class-qoobeewang。功能分支、Pull Request、GitHub Actions。'],
    ['编程规范', 'docs/conventions/coding-style.md。后端 gofmt，客户端 ESLint。'],
    ['硬约束', 'AGENTS.md：后端生产代码零第三方模块；学号和凭据不进仓库。'],
    ['任务', 'docs/roadmap.md 写依赖、触碰范围和验收标准。'],
  ], M, 1.55, CW, 0.85, 16);
  number(s, 4);
}

// ── P5 测试 ────────────────────────────────────────────────────────────────
{
  const s = page('有单元测试，工具和用例数量都超过课程下限', '测试');
  table(s,
    ['层次', '工具', '规模'],
    [
      ['单元', 'go test', '用例编号 300+'],
      ['集成', '目录树五要素比较', '过滤 × 压缩 × 加密，32 组'],
      ['CLI', 'testscript', '编号到 400 段'],
      ['客户端', 'Vitest、Playwright', '状态机单测；端到端含真实后端'],
    ],
    M, 1.45, CW, [2.2, 4.2, 5.2], { rowH: 0.48, size: 14 });
  text(s, '核心包语句覆盖率 87%～100%（2026-10-17）。go test -race 通过。FuzzReader 约 488 万次无 panic。课程要求至少 2 种测试、15 个用例。',
    M, 6.2, CW, 0.55, 13, { color: C.muted });
  number(s, 5);
}

// ── P6 进展 ────────────────────────────────────────────────────────────────
{
  const s = page('开发在 10 月 18 日冻结，答辩改到 11 月 18 日', '进展');
  text(s, '甘特图源文件 docs/project/diagrams/schedule-gantt.puml。完成日按仓库记录，不再沿用 10 月 15 日那版把答辩放在 11 月 16 日的计划。',
    M, 1.35, CW, 0.55, 13, { color: C.muted });
  table(s,
    ['日期', '仓库记录'],
    [
      ['10-15～10-16', 'M0，以及 W0 到 W3 合入'],
      ['10-17', 'W4；发布 v1.0.0'],
      ['10-18', '文档、PDF、PPT、演示成片；发布 v1.0.1'],
      ['10-19～11-17', '按 10 月 28 日的新口径改材料'],
      ['11-18 上午 / 23:59', '答辩与现场演示 / 材料截止'],
    ],
    M, 2.05, CW, [3.2, CW - 3.2], { rowH: 0.48, size: 14 });
  number(s, 6);
}

// ── P7 人员 ────────────────────────────────────────────────────────────────
{
  const s = page('单人组：需求到答辩都由组长承担', '人员');
  points(s, [
    ['分工', '组长王科瑾承担需求、设计、编码、测试和答辩。编码和文档起草使用 AI，验收在组长。'],
    ['已完成', 'FR-01～18，v1.0.1。开发在 2026-10-18 冻结。'],
    ['还没对齐新要求', '这套 3 分钟 PPT 还没重新导出；视频仍是 2 分 58 秒；提交包仍是平台加邮箱两包；7 分钟用的小目录还没准备。'],
  ], M, 1.6, CW, 1.15, 16);
  number(s, 7);
}

// ── P8 吐槽与演示 ──────────────────────────────────────────────────────────
{
  const s = page('口径改晚了；接下来请老师操作', '演示');
  points(s, [
    ['提交口径', '10 月 28 日才改为：视频 2 分钟以内的文件，四项打成一个包交成电慕课。10 月 18 日的成片和两个包不能直接交。'],
    ['当堂不做设备节点', '字符设备和块设备要 root 才能建。'],
    ['这 7 分钟', '请用一份小目录：普通文件、软链接或 FIFO、一条排除规则，并打开 Huffman 和 AES-256。录视频的约 500 MB 种子不适合当堂。'],
  ], M, 1.55, CW, 1.2, 16);
  number(s, 8);
}

await p.writeFile({ fileName: 'docs/ppt/答辩PPT.pptx' });
console.log('wrote docs/ppt/答辩PPT.pptx');
