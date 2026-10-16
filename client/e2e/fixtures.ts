import { execFile } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { mkdir, mkdtemp, readdir, readlink, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { readFileSync } from 'node:fs';
import {
  _electron as electron,
  expect,
  test as base,
  type ElectronApplication,
  type Page,
} from '@playwright/test';

/**
 * 真实后端 e2e fixture（docs/plans/C3-real-backend-e2e.md）：
 * 每条用例在独立临时目录中运行真实 databackup 二进制——HOME/XDG_CONFIG_HOME
 * 注入使后端配置、默认仓库与 Electron userData 全部落在临时根下，测试密闭
 * （testing.md §2）。app.close 走正常 quit 流程，main 的 will-quit 钩子会
 * 先取消仍在运行的后端进程（index.ts），不留孤儿进程与 .partial。
 */

const clientDir = path.resolve(__dirname, '..');
export const backendBin =
  process.env.DATABACKUP_BIN ?? path.resolve(clientDir, '..', 'backend', 'bin', 'databackup');

// Chromium refuses its sandbox when running as root (CI container).
const extraArgs =
  typeof process.getuid === 'function' && process.getuid() === 0 ? ['--no-sandbox'] : [];

/** 一条用例的独立文件系统环境，全部在临时根目录下。 */
export interface E2eEnv {
  /** 临时根，teardown 时整体删除。 */
  root: string;
  /** HOME；Electron userData 与默认仓库都落在这里。 */
  home: string;
  /** XDG_CONFIG_HOME；后端 config.json 落在这里。 */
  xdg: string;
  /** 默认仓库目录：$HOME/DataBackup。 */
  repo: string;
  /** 预置的备份源目录（含待排除的 node_modules）。 */
  src: string;
  /** 预置的空还原目标目录（满足 E_TARGET_NOT_EMPTY 校验）。 */
  dst: string;
}

/** 真实后端的就绪信号：状态栏出现非 mock 版本号。 */
const READY = /后端版本: (?!0\.0\.0-mock)\S+/;

/**
 * 确定性伪随机字节（xorshift32）。截图断言覆盖完成卡的"输出文件/压缩率"
 * 两位统计，它们由 Huffman(outputBytes) 推出——源数据用 crypto.randomBytes
 * 时输出大小逐次漂移几十字节，基线会随机翻车（PR #31 CI 实测 71px）。
 */
function pseudoBytes(n: number): Buffer {
  const buf = Buffer.alloc(n);
  let s = 0x9e3779b9;
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

type E2eFixtures = {
  env: E2eEnv;
  app: ElectronApplication;
  win: Page;
};

export const test = base.extend<E2eFixtures>({
  // eslint-disable-next-line no-empty-pattern -- Playwright fixture 签名要求首参为对象解构
  env: async ({}, use) => {
    const root = await mkdtemp(path.join(tmpdir(), 'databackup-e2e-'));
    const env: E2eEnv = {
      root,
      home: path.join(root, 'home'),
      xdg: path.join(root, 'xdg'),
      repo: path.join(root, 'home', 'DataBackup'),
      src: path.join(root, 'src'),
      dst: path.join(root, 'dst'),
    };
    await mkdir(env.home, { recursive: true });
    await mkdir(env.xdg, { recursive: true });
    await mkdir(path.join(env.src, 'sub'), { recursive: true });
    await mkdir(path.join(env.src, 'node_modules', 'dep'), { recursive: true });
    await mkdir(env.dst, { recursive: true });
    // 源目录预置内容：普通文件 + 子目录 + 应被 ^node_modules$ 排除的目录。
    await writeFile(path.join(env.src, 'hello.txt'), 'hello e2e\n');
    await writeFile(path.join(env.src, 'sub', 'data.bin'), pseudoBytes(4096));
    await writeFile(path.join(env.src, 'node_modules', 'dep', 'index.js'), 'ignored();\n');

    await use(env);

    await rm(root, { recursive: true, force: true });
  },

  app: async ({ env }, use, testInfo) => {
    const launchEnv = { ...process.env };
    delete launchEnv.DATABACKUP_MOCK;
    const app = await electron.launch({
      args: [...extraArgs, '.'],
      cwd: clientDir,
      env: {
        ...launchEnv,
        HOME: env.home,
        XDG_CONFIG_HOME: env.xdg,
        DATABACKUP_BIN: backendBin,
      },
      artifactsDir: path.join(env.root, 'artifacts'),
      timeout: 60_000,
    });
    // Electron 的 trace 需手动挂在 BrowserContext 上；失败时落盘供 M8 报告/排障引用。
    await app.context().tracing.start({ screenshots: true, snapshots: true });
    await use(app);
    const tracing = app.context().tracing;
    if (testInfo.status !== testInfo.expectedStatus) {
      await tracing.stop({ path: testInfo.outputPath('trace.zip') });
    } else {
      await tracing.stop();
    }
    // 正常 quit：will-quit 钩子先 cancelAll() 正在运行的后端（SIGTERM→3s→SIGKILL）。
    await app.close();
  },

  win: async ({ app }, use, testInfo) => {
    // CI/Docker 慢环境下 firstWindow 与首个 IPC 往返可比本地慢数倍，放宽到 60s。
    const win = await app.firstWindow({ timeout: 60_000 });
    await expect(win.getByTestId('statusbar')).toContainText(READY, { timeout: 60_000 });
    await use(win);
    if (testInfo.status !== testInfo.expectedStatus) {
      await win.screenshot({ path: testInfo.outputPath('failure.png') }).catch(() => {});
    }
  },
});

export { expect };

/**
 * stub 原生目录对话框：Playwright 无法拦截 Electron 的 dialog API
 * （走 OS 原生），只能在主进程用 evaluate 替换。paths 队列按序消费，
 * 耗尽后返回 canceled。必须在点击"浏览…"之前调用。
 */
export async function stubOpenDialog(app: ElectronApplication, paths: string[]): Promise<void> {
  await app.evaluate(({ dialog }, picks) => {
    const g = globalThis as unknown as { __dbkPicks: string[] };
    g.__dbkPicks = [...picks];
    const d = dialog as unknown as {
      showOpenDialog: (...args: unknown[]) => Promise<{ canceled: boolean; filePaths: string[] }>;
    };
    d.showOpenDialog = async () => {
      const p = g.__dbkPicks.shift();
      return p === undefined
        ? { canceled: true, filePaths: [] }
        : { canceled: false, filePaths: [p] };
    };
  }, paths);
}

/** 直接调真实 CLI（用于前置造数据，如 TC-E2E-09 需要一个已存在的加密备份）。 */
export function runCli(args: string[], stdin?: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = execFile(backendBin, ['--json', ...args], (err, stdout, stderr) => {
      if (err) {
        reject(
          new Error(`databackup ${args.join(' ')} 失败 (code=${err.code}): ${stderr}${stdout}`),
        );
      } else {
        resolve(stdout);
      }
    });
    child.stdin?.end(stdin === undefined ? undefined : `${stdin}\n`);
  });
}

interface TreeEntry {
  type: 'f' | 'd' | 'l';
  content?: Buffer;
  target?: string;
}

async function collectTree(
  root: string,
  excludeDirNames: RegExp[],
  rel = '',
  out = new Map<string, TreeEntry>(),
): Promise<Map<string, TreeEntry>> {
  for (const e of await readdir(path.join(root, rel), { withFileTypes: true })) {
    const r = rel === '' ? e.name : `${rel}/${e.name}`;
    if (e.isDirectory()) {
      if (excludeDirNames.some((x) => x.test(e.name))) continue;
      out.set(r, { type: 'd' });
      await collectTree(root, excludeDirNames, r, out);
    } else if (e.isSymbolicLink()) {
      out.set(r, { type: 'l', target: await readlink(path.join(root, r)) });
    } else {
      out.set(r, { type: 'f', content: readFileSync(path.join(root, r)) });
    }
  }
  return out;
}

/**
 * e2e 粒度的目录树断言：相对路径集合一致、文件内容逐字节一致、
 * 软链接目标一致（元数据五要素由后端 testutil 覆盖，这里只证界面链路的结果）。
 */
export async function assertTreeEqual(
  src: string,
  dst: string,
  opts: { excludeDirs?: RegExp[] } = {},
): Promise<void> {
  const ex = opts.excludeDirs ?? [];
  const a = await collectTree(src, ex);
  const b = await collectTree(dst, []);
  expect([...b.keys()].sort(), '还原结果的相对路径集合应与源一致').toEqual([...a.keys()].sort());
  for (const [r, ea] of a) {
    const eb = b.get(r);
    if (ea.type === 'f') expect(eb?.content?.equals(ea.content ?? Buffer.alloc(0))).toBe(true);
    if (ea.type === 'l') expect(eb?.target).toBe(ea.target);
  }
}

/**
 * 造一个"足够慢"的源树供取消用例使用：大量小文件拉长扫描/打包阶段，
 * 若干 MB 随机数据拉长写入阶段。取消窗口不能以 sleep 凑，只能靠真实工作量。
 */
export async function makeBigTree(dir: string): Promise<void> {
  for (let i = 0; i < 100; i++) {
    await mkdir(path.join(dir, `d${i}`), { recursive: true });
  }
  const small = Buffer.from('payload-'.repeat(16));
  for (let i = 0; i < 8000; i += 400) {
    await Promise.all(
      Array.from({ length: 400 }, (_, j) =>
        writeFile(path.join(dir, `d${(i + j) % 100}`, `f${i + j}.txt`), small),
      ),
    );
  }
  const big = randomBytes(4 * 1024 * 1024);
  for (let i = 0; i < 8; i++) {
    await writeFile(path.join(dir, `big-${i}.bin`), big);
  }
}
