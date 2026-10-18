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
const TOTAL = 14;

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
    x: 1.1, y: 1.95, w: 10, h: 1.1, fontSize: 52, bold: true, fontFace: F, color: C.ink, margin: 0,
  });
  s.addText('Linux 目录树备份与还原软件', {
    x: 1.12, y: 3.05, w: 10, h: 0.5, fontSize: 22, fontFace: F, color: C.accent, margin: 0,
  });
  s.addShape(p.shapes.LINE, {
    x: 1.12, y: 3.75, w: 5.2, h: 0, line: { color: C.rule, width: 1 },
  });
  s.addText([
    { text: '《软件开发综合实验》课程项目', options: { breakLine: true } },
    { text: '组长：王科瑾（单人组）', options: { breakLine: true } },
    { text: '电子科技大学 · 2026 年 11 月', options: {} },
  ], {
    x: 1.12, y: 4.0, w: 7, h: 1.1, fontSize: 13, fontFace: F, color: C.muted,
    margin: 0, lineSpacingMultiple: 1.5,
  });
  s.addText('v1.0.1', {
    x: 1.12, y: 5.75, w: 3, h: 0.35, fontSize: 11, fontFace: MONO, color: C.faint, margin: 0,
  });
}

// ── P2 需求与范围 ──────────────────────────────────────────────────────────
{
  const s = page('需求范围：基础功能与全部扩展项', '需求');
  text(s, '课程要求分为基础 40 分与扩展项；本项目 18 条功能需求全部实现，未做实时备份与网络备份。',
    M, 1.35, CW, 0.5, 12.5);
  table(s,
    ['需求', '内容', '计分'],
    [
      ['FR-01 / 02', '把目录树备份为单个文件；还原到指定目录', '基础 40'],
      ['FR-03 / 04', '8 种文件类型（含硬链接、设备、套接字）；权限、属主、纳秒时间', '扩展 20'],
      ['FR-05 ~ 10', '路径、类型、名字、时间、尺寸、用户组六个过滤维度，可组合', '扩展 25'],
      ['FR-11', '自研 MyPack 归档格式（512B 头记录，仿 tar）', '扩展 10'],
      ['FR-12', '自研 Huffman 压缩（1 MiB 分块流式）', '扩展 10'],
      ['FR-13', '自研 AES-256 加密：PBKDF2 派生 + CBC + HMAC', '扩展 10'],
      ['FR-14 / 15', '备份列表、详情、删除；仓库路径设置', '支撑'],
      ['FR-16 ~ 18', 'Electron 客户端；错误口令与路径穿越防护；JSONL 进度', '扩展 10'],
    ],
    M, 1.95, 8.3, [1.45, 5.9, 0.95],
  );
  s.addShape(p.shapes.RECTANGLE, {
    x: 9.45, y: 1.95, w: 3.1, h: 3.35, fill: { color: C.tint },
  });
  label(s, '明确不做', 9.7, 2.15, 2.6);
  text(s, '实时备份、网络备份。\n\n两项都需要常驻服务或服务器，与"单机、单文件、可离线"的课程场景不匹配。',
    9.7, 2.5, 2.55, 1.4, 10.5, { color: C.body });
  label(s, '可选项（已完成）', 9.7, 3.8, 2.7);
  text(s, 'Windows 便携包：同一套后端交叉编译，平台不支持的元数据静默跳过，仅用于课堂演示兜底。',
    9.7, 4.15, 2.55, 1.1, 10.5, { color: C.body });
  text(s, '计分口径见 docs/project/course-requirements.md；扩展项按课程 85 分上限核算。',
    M, 5.55, 8.3, 0.4, 9.5, { color: C.faint });
  number(s, 2);
}

// ── P3 总体架构 ────────────────────────────────────────────────────────────
{
  const s = page('架构：进程边界清晰，客户端不含业务逻辑', '系统设计');
  image(s, IMG + 'component-top.png', 8.85, 1.35, 3.9, 5.35);
  caption(s, '图：顶层构件（docs/deliverables/diagrams/component-top.puml）', 8.85, 6.74, 3.9);
  points(s, [
    ['编译型后端', 'Go 1.27，生产代码零第三方模块；自研约束由 scripts/check-imports.sh 强制。'],
    ['唯一接口', '客户端 spawn CLI，stdout 输出 JSON Lines；协议冻结在 cli-contract.md。'],
    ['无数据库', '.dbk 头尾记录元信息，list / info 只读文件首尾，不需要口令。'],
    ['口令隔离', '口令经 stdin 传入，不进参数、环境变量与日志。'],
  ], M, 1.55, 7.6, 0.86, 12);
  s.addShape(p.shapes.RECTANGLE, { x: M, y: 5.35, w: 7.6, h: 0.62, fill: { color: C.accentTint } });
  text(s, '契约双端一致：Go 类型与 TypeScript 类型由测试比对同一组 JSON 样例，字段漂移会直接失败。',
    M + 0.2, 5.45, 7.2, 0.45, 11, { color: C.accent });
  number(s, 3);
}

// ── P4 备份流水线 ──────────────────────────────────────────────────────────
{
  const s = page('备份流水线：写临时文件，校验通过后才发布', '系统设计');
  const steps = [
    ['scan', '遍历目录树\n套用过滤规则'],
    ['pack', 'MyPack\n逐条写头与数据'],
    ['compress', 'Huffman\n1 MiB 分块'],
    ['encrypt', 'AES-256-CBC\nHMAC 尾部'],
    ['publish', 'fsync + link\n.partial → .dbk'],
  ];
  steps.forEach(([h, d], i) => {
    const x = M + i * 2.42;
    s.addShape(p.shapes.RECTANGLE, {
      x, y: 1.65, w: 2.12, h: 0.95,
      fill: { color: i === 4 ? C.accent : C.tint },
      line: { color: i === 4 ? C.accent : C.rule, width: 1 },
    });
    s.addText(h, {
      x: x + 0.16, y: 1.72, w: 1.8, h: 0.3, fontSize: 13, bold: true, fontFace: MONO,
      color: i === 4 ? 'FFFFFF' : C.ink, margin: 0,
    });
    s.addText(d, {
      x: x + 0.16, y: 2.02, w: 1.8, h: 0.5, fontSize: 9.5, fontFace: F,
      color: i === 4 ? 'EAF1EE' : C.muted, margin: 0, lineSpacingMultiple: 1.1,
    });
    if (i < 4) {
      s.addShape(p.shapes.LINE, {
        x: x + 2.12, y: 2.12, w: 0.3, h: 0, line: { color: C.faint, width: 1 },
      });
    }
  });
  table(s, ['环节', '关键做法', '失败时的行为'], [
    ['scan', 'lstat 遍历、稳定排序、硬链接按 inode 去重', '不可读项跳过并计数，不中断任务'],
    ['pack', '头记录定长 512 B，长路径溢出到扩展块', '路径不合规返回 ErrUnsafePath'],
    ['compress', '每块独立码表，内存与输入大小无关', '随机数据不膨胀，最差 1.0'],
    ['encrypt', 'encrypt-then-MAC，尾部记录统计与校验', '篡改或口令错误返回认证失败（退出码 3）'],
    ['publish', '先写 .dbk.partial，fsync 后硬链接发布', '同名已存在返回 E_EXISTS；取消删除半成品'],
  ], M, 3.25, CW, [1.5, 7.2, 5.1], { rowH: 0.44, size: 10.5 });
  text(s, '还原方向相反：先对整文件做 MAC/SHA 校验，通过后才开始写目标目录，避免"还原到一半发现备份已损坏"。',
    M, 6.25, CW, 0.4, 11, { color: C.muted });
  number(s, 4);
}

// ── P5 MyPack ─────────────────────────────────────────────────────────────
{
  const s = page('MyPack：512 字节定长头记录 + 全类型支持', '自研格式');
  // 头记录布局示意
  const segs = [
    ['魔数/版本/类型/标志', 1.5, C.accent],
    ['CRC-32', 1.1, C.accent],
    ['长度与元数据字段', 3.3, C.accent],
    ['时间戳（秒 + 纳秒）', 2.7, C.accent],
    ['名字区 408 B', 2.9, C.accentTint],
  ];
  let x = M;
  segs.forEach(([t, w, col], i) => {
    s.addShape(p.shapes.RECTANGLE, {
      x, y: 1.55, w, h: 0.62, fill: { color: col },
      line: { color: i === 4 ? C.accent : 'FFFFFF', width: i === 4 ? 1 : 0.5 },
    });
    s.addText(t, {
      x: x + 0.05, y: 1.55, w: w - 0.1, h: 0.62, fontSize: 8.5, fontFace: F,
      color: i === 4 ? C.accent : 'FFFFFF', align: 'center', valign: 'middle', margin: 0,
    });
    x += w;
  });
  text(s, '小端二进制；字段定长，读取时逐项校验；名字放不下时用扩展块续写（每块 512 B），归档以两个全零块结尾。',
    M, 2.3, CW, 0.4, 11, { color: C.muted });

  table(s, ['类型码', '对象', '数据'], [
    ['0', '普通文件', 'size 字节，补齐到 512 的倍数'],
    ['1 / 2', '硬链接 / 软链接', '硬链接存目标路径；软链接原样保存'],
    ['3 / 4', '字符设备 / 块设备', '设备主次号写入头记录'],
    ['5 / 6 / 7', '目录 / 命名管道 / 套接字', '无数据，仅元信息'],
  ], M, 2.9, 6.6, [1.0, 2.6, 3.0], { rowH: 0.42, size: 10 });

  s.addShape(p.shapes.RECTANGLE, { x: M, y: 5.35, w: 6.6, h: 1.05, fill: { color: C.tint } });
  text(s, '往返一致性由 golden 文件与 testdata 目录树断言：类型、权限、uid/gid、atime/mtime（含纳秒）逐项比对。',
    M + 0.2, 5.55, 6.2, 0.9, 10.5, { color: C.body });

  label(s, '读取端的安全校验', 7.85, 2.9, 4.7);
  points(s, [
    '拒绝绝对路径、空段、`.` 与 `..` 段、NUL 字节。',
    '拒绝以已出现软链接为祖先的后续路径，堵住借软链接写出还原目录的攻击。',
    'CRC-32、保留位、纳秒范围、扩展块个数与结尾零块逐项校验；出错后持续报同一错误。',
  ], 7.85, 3.3, 4.6, 0.82, 10.5);

  s.addShape(p.shapes.LINE, { x: 7.85, y: 5.95, w: 4.6, h: 0, line: { color: C.rule, width: 1 } });
  s.addText('4,881,324', {
    x: 7.85, y: 6.08, w: 2.4, h: 0.35, fontSize: 20, bold: true, fontFace: F, color: C.ink, margin: 0,
  });
  text(s, '次 FuzzReader 执行（30 s），无 panic、无死循环。', 10.15, 6.12, 2.55, 0.5, 9, { color: C.muted });
  number(s, 5);
}

// ── P6 Huffman ────────────────────────────────────────────────────────────
{
  const s = page('Huffman：分块流式编码，内存不随文件增长', '自研压缩');
  // 块结构
  const blocks = [
    ['块头：码长表 + 原始长度', 5.0, C.accent],
    ['编码数据', 3.6, C.accentTint],
  ];
  let x = M;
  blocks.forEach(([t, w, col], i) => {
    s.addShape(p.shapes.RECTANGLE, {
      x, y: 1.55, w, h: 0.6, fill: { color: col },
      line: { color: i === 1 ? C.accent : 'FFFFFF', width: i === 1 ? 1 : 0.5 },
    });
    s.addText(t, {
      x: x + 0.08, y: 1.55, w: w - 0.16, h: 0.6, fontSize: 9.5, fontFace: F,
      color: i === 1 ? C.accent : 'FFFFFF', align: 'center', valign: 'middle', margin: 0,
    });
    x += w;
  });
  text(s, '每个 1 MiB 块独立建树，码长表写进块头，解码端按规范码重建码表——所以可以逐块流式读写，不需要把整个文件读进内存。',
    M, 2.3, CW, 0.5, 11, { color: C.muted });

  label(s, '实测（512 MiB 文本，WSL2 + NVMe）', M, 2.95);
  table(s, ['场景', '耗时', '吞吐', '输出大小', '压缩率'], [
    ['备份，无压缩', '3.22 s', '≈167 MB/s', '536.9 MB', '1.000'],
    ['备份，Huffman', '5.51 s', '≈97 MB/s', '330.8 MB', '0.616'],
    ['还原，Huffman', '5.67 s', '≈95 MB/s', '—', '—'],
    ['备份，随机数据', '7.76 s', '≈69 MB/s', '537.0 MB', '1.000'],
  ], M, 3.35, 7.4, [2.5, 1.2, 1.4, 1.3, 1.0], { rowH: 0.4, size: 10.5 });

  label(s, '边界条件', 8.55, 2.95);
  points(s, [
    '空输入、单字节、全同字节、跨块大文件都有用例。',
    '随机数据不膨胀：压缩率不会超过 1.0。',
    '码长表非法或码表不完整时解码直接报错，不输出半个文件。',
    '单线程实现是吞吐上限的主要原因，见第 12 页。',
  ], 8.55, 3.35, 3.9, 0.82, 10.5);

  s.addShape(p.shapes.RECTANGLE, { x: M, y: 5.55, w: 7.4, h: 0.9, fill: { color: C.tint } });
  text(s, '10,000 个小文件：备份 0.16 s、还原 0.36 s；每文件摊薄开销约 16 µs（含 lstat、头记录与元数据）。',
    M + 0.2, 5.7, 7.0, 0.6, 10.5);
  number(s, 6);
}

// ── P7 加密与容器 ─────────────────────────────────────────────────────────
{
  const s = page('加密：口令派生双密钥，先加密后认证', '自研加密');
  const parts = [
    ['明文头\n32 B + 盐 16 + IV 16 + 源路径', 4.2, C.accent],
    ['密文负载\nAES-256-CBC + PKCS#7', 3.6, C.accentTint],
    ['明文尾\n40 B 统计', 2.2, C.tint],
    ['校验\n32 B HMAC/SHA-256', 1.8, C.tint],
  ];
  let x = M;
  parts.forEach(([t, w, col], i) => {
    s.addShape(p.shapes.RECTANGLE, {
      x, y: 1.6, w, h: 0.78, fill: { color: col },
      line: { color: i > 1 ? C.rule : 'FFFFFF', width: 1 },
    });
    s.addText(t, {
      x: x + 0.06, y: 1.6, w: w - 0.12, h: 0.78, fontSize: 8.5, fontFace: F,
      color: i === 0 ? 'FFFFFF' : C.body, align: 'center', valign: 'middle', margin: 0,
      lineSpacingMultiple: 1.1,
    });
    x += w;
  });

  label(s, '密钥派生（RFC 8018 §5.2）', M, 2.75, 5);
  s.addText('K = PBKDF2-HMAC-SHA256(口令, salt, 100 000 轮, 64 B)', {
    x: M, y: 3.08, w: 7.5, h: 0.3, fontSize: 10.5, fontFace: MONO, color: C.ink, margin: 0,
  });
  s.addText('K[0:32] → AES-256 密钥        K[32:64] → HMAC 密钥', {
    x: M, y: 3.42, w: 7.5, h: 0.3, fontSize: 10.5, fontFace: MONO, color: C.muted, margin: 0,
  });

  points(s, [
    ['先加密后认证', 'HMAC 覆盖头部、负载与尾部；改算法标识、迭代次数或统计值同样会被发现。'],
    ['常量时间比较', '校验值比较不做短路，避免时序侧信道。'],
    ['参数有上限', '迭代次数限制在 1 ~ 1 000 000，防止伪造文件让还原卡在密钥派生。'],
    ['错误可区分', '口令错误或内容被篡改统一返回 E_AUTH（退出码 3）；客户端重试 3 次后回到编辑态。'],
  ], M, 3.95, 7.4, 0.62, 10.5);

  s.addShape(p.shapes.RECTANGLE, { x: 8.6, y: 2.75, w: 3.98, h: 3.2, fill: { color: C.tint } });
  label(s, '公开向量对照', 8.85, 2.95, 3.4);
  text(s, 'FIPS-197（AES 分组）\nFIPS 180-4（SHA-256）\nRFC 2104（HMAC）\nRFC 7914（PBKDF2）\nNIST SP 800-38A（CBC）',
    8.85, 3.35, 3.4, 1.5, 10.5, { color: C.body, lineSpacingMultiple: 1.35 });
  text(s, '同一组用例在 _test.go 中与标准库输出逐字节对照；生产代码不引用标准库实现。',
    8.85, 4.95, 3.45, 0.85, 9.5, { color: C.muted });
  number(s, 7);
}

// ── P8 客户端 ─────────────────────────────────────────────────────────────
{
  const s = page('客户端：三个页面，备份与还原都在一屏内完成', '图形界面');
  image(s, SHOT + '01-backup.png', M, 1.4, 5.75, 3.7);
  caption(s, '备份：源目录、六个筛选维度、压缩与加密、口令与预览', M, 5.12, 5.75);
  image(s, SHOT + '04-manage.png', M + 5.95, 1.4, 5.75, 3.7);
  caption(s, '备份管理：列表、完整性标记、详情 Sheet 与还原入口', M + 5.95, 5.12, 5.75);
  points(s, [
    'Electron 44 + React 19，三页面（备份 / 管理 / 设置）；侧栏折叠、减少动效等偏好可设置。',
    '备份任务状态机 9 个状态全部有单元测试覆盖；进度来自后端 JSON Lines，界面只做展示。',
    '端到端用 Playwright 驱动打包产物 + 真实后端二进制，覆盖"过滤 + 压缩 + 加密 → 还原一致"与 E_AUTH。',
  ], M, 5.5, CW, 0.42, 10.5);
  number(s, 8);
}

// ── P9 工程化 ─────────────────────────────────────────────────────────────
{
  const s = page('工程化：仓库、门禁与协作规则都写进仓库', '工程实践');
  label(s, '仓库结构', M, 1.4, 4);
  s.addText([
    { text: 'backend/', options: { bold: true, color: C.ink } },
    { text: '  Go CLI 与核心库，生产代码零第三方依赖', options: { color: C.body, breakLine: true } },
    { text: 'client/', options: { bold: true, color: C.ink } },
    { text: '  Electron + React，只做界面与 CLI 调用', options: { color: C.body, breakLine: true } },
    { text: 'docs/', options: { bold: true, color: C.ink } },
    { text: '  需求、设计、测试、规范与交付源文件', options: { color: C.body, breakLine: true } },
    { text: 'scripts/ · docker/', options: { bold: true, color: C.ink } },
    { text: '  门禁、文档构建与开发镜像', options: { color: C.body } },
  ], { x: M, y: 1.75, w: 5.4, h: 1.9, fontSize: 11, fontFace: F, margin: 0, lineSpacingMultiple: 1.4 });

  label(s, '质量门禁', 6.4, 1.4, 6);
  table(s, ['检查', '覆盖内容', '执行时机'], [
    ['make check', '格式、lint、禁用 import、构建与单测', '每次提交前'],
    ['make check-all', 'check + race + 后端 e2e + 客户端 e2e', '合并前 / 核心改动'],
    ['CI', 'PR 四 job 全绿；打 tag 自动构建与发布', 'PR 与 Release'],
  ], 6.4, 1.75, 6.18, [1.35, 3.45, 1.35], { rowH: 0.55, size: 9.5 });

  label(s, '协作与约束', 6.4, 4.35, 6);
  points(s, [
    'AGENTS.md 是硬约束的唯一来源：零第三方依赖、无学号与凭据、PDF 只在发布节点生成。',
    '任务在 docs/roadmap.md 里按波次与依赖排期；每个 PR 对照任务 ID 与验收标准。',
    '行为变更必须同 PR 改文档；提交信息与 Release Notes 有固定写法（docs/conventions/）。',
  ], 6.4, 4.7, 6.15, 0.62, 10);

  s.addShape(p.shapes.RECTANGLE, { x: M, y: 4.9, w: 5.4, h: 1.25, fill: { color: C.tint } });
  text(s, '开发方式：编码 agent 按上述规则执行实现与测试，组长负责需求、设计决策、验收与答辩。',
    M + 0.2, 5.05, 5.0, 0.9, 10.5);
  number(s, 9);
}

// ── P10 测试设计 ──────────────────────────────────────────────────────────
{
  const s = page('测试：四层用例，需求与用例编号双向可查', '软件测试');
  table(s, ['层次', '范围', '手段', '数量级'], [
    ['单元测试', '各内部包：格式、压缩、加密、过滤、遍历、仓库', 'go test，表驱动 + golden 文件', '300+ 用例编号'],
    ['集成测试', 'engine 全流水线：七种文件类型、元数据、过滤组合', '临时目录树 + 五要素比较器', '32 组合矩阵'],
    ['CLI 黑盒', '子命令、参数校验、JSONL 事件、退出码', 'testscript（唯一测试依赖）', '110 ~ 400 系列'],
    ['端到端', '客户端 + 真实后端二进制', 'Playwright（含截图回归）', 'TC-E2E-01 ~ 09'],
  ], M, 1.4, CW, [1.5, 5.3, 4.0, 1.3], { rowH: 0.58, size: 10 });

  label(s, '用例编号约定', M, 4.45, 6);
  text(s, '测试代码里直接写编号（如 // TC-PACK-042），需求文档的 FR 编号与测试报告的 TC 编号互相对照，评审时可按编号检索实现与证据。',
    M, 4.8, 7.4, 0.8, 10.5, { color: C.body });

  label(s, '证据位置', 8.4, 4.45, 4.2);
  text(s, '测试报告 §3 覆盖率\n§3.3 race 与 fuzz\n§4 性能与剖析\n§5 健壮性用例表',
    8.4, 4.8, 4.2, 1.4, 10.5, { color: C.body, lineSpacingMultiple: 1.3 });

  s.addShape(p.shapes.RECTANGLE, { x: M, y: 5.95, w: CW, h: 0.75, fill: { color: C.accentTint } });
  text(s, '健壮性单独成组：篡改任一字节、截断、错误口令、取消任务、路径穿越、损坏备份出现在列表中，都有对应用例。',
    M + 0.2, 6.12, CW - 0.4, 0.45, 10.5, { color: C.accent });
  number(s, 10);
}

// ── P11 测试结果 ──────────────────────────────────────────────────────────
{
  const s = page('测试结果：核心包覆盖率 87% ~ 100%，竞态与模糊测试干净', '软件测试');
  table(s, ['包', '覆盖率', '包', '覆盖率'], [
    ['crypto/sha256', '100.0%', 'engine', '87.2%'],
    ['crypto/hmac', '100.0%', 'walk', '93.2%'],
    ['jsonl', '100.0%', 'repo', '87.3%'],
    ['crypto/aes', '99.2%', 'huffman', '94.4%'],
    ['filter', '99.0%', 'pack', '95.3%'],
    ['crypto/pbkdf2', '97.1%', 'container', '92.2%'],
  ], M, 1.4, 7.4, [2.0, 1.3, 2.0, 1.3], { rowH: 0.42, size: 10.5 });
  caption(s, '语句覆盖率，2026-10-17 实测；cli 包逻辑经 testscript 子进程执行，不计入包内统计。', M, 4.55, 7.4);

  label(s, '其它检查', 8.5, 1.4, 4);
  table(s, ['项目', '结果'], [
    ['race 检测', '全部通过'],
    ['FuzzReader', '488 万次，无 panic'],
    ['FuzzDecode', '4.06 万次，无 panic'],
    ['staticcheck / vet', '零告警'],
    ['ESLint / prettier', '零告警'],
  ], 8.5, 1.75, 4.08, [2.2, 1.88], { rowH: 0.4, size: 10 });

  s.addShape(p.shapes.RECTANGLE, { x: M, y: 5.0, w: 7.4, h: 1.15, fill: { color: C.tint } });
  label(s, '过滤 × 压缩 × 加密 组合矩阵', M + 0.2, 5.12, 6.8);
  text(s, '六维过滤各取"开 / 关"与压缩、加密组合，共 32 组；每组断言备份退出码、容器头标志位与还原后目录树一致。',
    M + 0.2, 5.45, 6.9, 0.6, 10.5);
  number(s, 11);
}

// ── P12 性能与限制 ────────────────────────────────────────────────────────
{
  const s = page('性能与边界：实测数据与已知限制', '软件测试');
  table(s, ['场景', '数据量', '耗时', '吞吐'], [
    ['备份（无压缩）', '512 MiB', '3.22 s', '≈167 MB/s'],
    ['备份（Huffman）', '512 MiB', '5.51 s', '≈97 MB/s'],
    ['还原（Huffman）', '512 MiB', '5.67 s', '≈95 MB/s'],
    ['加密容器写入', '基准', '27.8 ms/op', '≈37.8 MB/s'],
    ['10,000 个小文件', '260 KB', '0.16 s', '—'],
  ], M, 1.4, 7.6, [2.4, 1.2, 1.4, 1.7], { rowH: 0.4, size: 10.5 });
  caption(s, 'WSL2 Ubuntu 24.04 + NVMe，真实二进制；微基准为 go test -bench，benchtime=2s。', M, 3.9, 7.6);

  label(s, 'pprof 热点（加密容器写入）', 8.9, 1.4, 3.7);
  table(s, ['函数', '占比'], [
    ['aes.addRoundKey', '18.6%'],
    ['aes.shiftRows', '18.3%'],
    ['aes.mixColumns', '15.6%'],
    ['sha256.block', '15.6%'],
    ['aes.subBytes', '12.2%'],
  ], 8.9, 1.75, 3.68, [2.6, 1.08], { rowH: 0.36, size: 9.5 });
  caption(s, '采样 3 s；WSL2 不提供 perf_event，用 Go pprof 替代。', 8.9, 3.95, 3.68);

  label(s, '已知限制', M, 4.55, 8);
  points(s, [
    '超过 10 万个条目的目录树没有做过端到端实测；测试报告 §6.3 对此有说明。',
    'Huffman 为单线程实现，压缩吞吐低于不压缩路径；加密写入约 38 MB/s，是当前最慢环节。',
    '仓库是普通目录：没有增量备份、去重与并发写保护，同一仓库并发备份只能靠 E_EXISTS 兜底。',
    'Windows 便携包会静默跳过 uid/gid、权限位、设备节点与硬链接身份，仅作演示用途。',
  ], M, 4.9, 11.8, 0.55, 10.5);
  number(s, 12);
}

// ── P13 交付与运行 ────────────────────────────────────────────────────────
{
  const s = page('交付物：一次构建产出命令行、两种安装包与全部文档', '交付');
  table(s, ['产物', '大小', '说明'], [
    ['DataBackup-1.0.1.AppImage', '139.8 MB', '单文件免安装，内置后端二进制'],
    ['databackup-client_1.0.1_amd64.deb', '107.6 MB', 'dpkg 安装，菜单入口 /opt/DataBackup'],
    ['databackup-1.0.1-linux-x86_64.tar.gz', '1.6 MB', '独立 CLI，可脚本化调用'],
    ['databackup-1.0.1-source.tar.gz', '9.4 MB', '源码包，含构建脚本'],
    ['DataBackup-1.0.1-portable.exe', '116 MB', 'Windows 便携包（可选加分项）'],
    ['文档 PDF × 6', '6.1 MB', '需求、设计、测试、实验报告、PPT、进度'],
  ], M, 1.4, CW, [4.9, 1.5, 5.3], { rowH: 0.4, size: 10.5 });
  caption(s, 'v1.0.1 Release 资产，同时提供 sha256sums.txt；Windows 便携包为宿主机构建后追加。', M, 4.3, CW);

  label(s, '运行', M, 4.65, 4);
  s.addText([
    { text: 'sudo dpkg -i databackup-client_1.0.1_amd64.deb', options: { breakLine: true } },
    { text: 'databackup backup --source ~/docs --repo ~/backups \\', options: { breakLine: true } },
    { text: '           --compress huffman --encrypt aes256 --password-stdin', options: {} },
  ], { x: M, y: 4.98, w: 7.6, h: 0.9, fontFace: MONO, fontSize: 10, color: C.ink, margin: 0, lineSpacingMultiple: 1.25 });

  s.addShape(p.shapes.RECTANGLE, { x: 8.55, y: 4.65, w: 4.03, h: 1.0, fill: { color: C.tint } });
  text(s, '目标平台 Ubuntu 22.04 / 24.04。WSL2 + WSLg 的安装、依赖与已知问题见 docs/client/wsl-guide.md。',
    8.75, 4.8, 3.65, 0.75, 10);
  text(s, '演示视频 ≤3 分钟，按 docs/deliverables/06-演示视频脚本.md 录制。', M, 6.15, CW, 0.4, 10.5, { color: C.muted });
  number(s, 13);
}

// ── P14 总结 ──────────────────────────────────────────────────────────────
{
  const s = page('小结：课程要求的功能全部落地，边界清楚', '总结');
  label(s, '已完成', M, 1.45, 5);
  points(s, [
    '基础备份/还原 + 全部扩展项：七类文件对象、完整元数据、六维过滤、自研格式/压缩/加密、Electron 客户端。',
    '自研实现有公开向量与 golden 文件对照；客户端走真实后端端到端验证。',
    '从需求编号到测试编号、从 Makefile 到 CI 与 Release，工程链路完整可复现。',
  ], M, 1.8, 5.9, 0.78, 11);

  label(s, '未做与后续', 6.95, 1.45, 5.5);
  points(s, [
    '增量备份、去重、并发写保护：都属于仓库层设计，需要先定新的格式版本。',
    '更大规模目录树的实测（10 万条目以上）与多线程压缩。',
    'Windows 版的元数据能力受平台限制，短期只作为演示兜底。',
  ], 6.95, 1.8, 5.55, 0.78, 11);

  s.addShape(p.shapes.LINE, { x: M, y: 3.85, w: CW, h: 0, line: { color: C.rule, width: 1 } });
  text(s, '代码规模：后端 Go 生产 6,755 行、测试 7,157 行；客户端 TypeScript/TSX 4,749 行、测试 2,325 行；文档 Markdown 3,600 余行（不含图）。',
    M, 4.0, CW, 0.5, 10.5, { color: C.muted });

  s.addShape(p.shapes.LINE, { x: M, y: 4.75, w: CW, h: 0, line: { color: C.rule, width: 1 } });
  s.addText('谢谢', {
    x: M, y: 4.95, w: 4, h: 0.7, fontSize: 30, bold: true, fontFace: F, color: C.ink, margin: 0,
  });
  text(s, '演示见视频或现场：过滤 + 压缩 + 加密备份 → 口令还原 → 目录树比对。',
    M, 5.7, CW, 0.4, 11.5, { color: C.muted });
  number(s, 14);
}

await p.writeFile({ fileName: 'docs/ppt/答辩PPT.pptx' });
console.log('wrote docs/ppt/答辩PPT.pptx');
