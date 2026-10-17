import { expect, type Locator, type Page } from '@playwright/test';

/**
 * 界面结构契约（docs/client/visual-regression.md §3 L1）。
 *
 * 这些断言不比对像素，只检查"布局与样式是否还在"：Sheet 是否贴右侧、
 * 对话框是否居中、遮罩是否存在、页面是否被撑出视口。样式整体失效
 * （如 Tailwind 工具类没生成）时，元素仍然 attached/visible，只有这类
 * 断言能发现。
 */

const EPS = 2;

async function viewport(page: Page): Promise<{ width: number; height: number }> {
  return page.evaluate(() => ({ width: window.innerWidth, height: window.innerHeight }));
}

export async function expectNoHorizontalOverflow(page: Page): Promise<void> {
  const m = await page.evaluate(() => ({
    doc: document.documentElement.scrollWidth,
    body: document.body.scrollWidth,
    width: window.innerWidth,
  }));
  expect(m.doc, '文档不应出现水平滚动').toBeLessThanOrEqual(m.width + 1);
  expect(m.body, 'body 不应被撑宽').toBeLessThanOrEqual(m.width + 1);
}

/** 遮罩层：fixed、覆盖视口、半透明黑。 */
export async function expectFixedOverlay(page: Page, slot: string, minAlpha = 0.3): Promise<void> {
  const data = await page.locator(`[data-slot="${slot}"]`).evaluate((el) => {
    const cs = el.ownerDocument.defaultView!.getComputedStyle(el);
    const r = el.getBoundingClientRect();
    return {
      position: cs.position,
      background: cs.backgroundColor,
      x: r.x,
      y: r.y,
      width: r.width,
      height: r.height,
    };
  });
  const vp = await viewport(page);
  expect(data.position, `${slot} 应为 fixed`).toBe('fixed');
  expect(data.x).toBeLessThanOrEqual(1);
  expect(data.y).toBeLessThanOrEqual(1);
  expect(data.width).toBeGreaterThanOrEqual(vp.width - EPS);
  expect(data.height).toBeGreaterThanOrEqual(vp.height - EPS);

  // Tailwind v4 用 color-mix/oklab 表达带透明度颜色，计算值不一定是 rgba()。
  const alphaMatch =
    /\/\s*([\d.]+)\s*\)/.exec(data.background) ??
    /rgba?\([^)]*,\s*([\d.]+)\s*\)/.exec(data.background);
  const alpha = alphaMatch ? Number(alphaMatch[1]) : data.background === 'transparent' ? 0 : 1;
  expect(data.background, `${slot} 应有非透明背景`).not.toBe('transparent');
  expect(alpha, `${slot} 遮罩不透明度`).toBeGreaterThanOrEqual(minAlpha);
}

/** 详情 Sheet：fixed、贴右缘、宽 320–560、高≈视口。 */
export async function expectSheetOnRight(sheet: Locator): Promise<void> {
  const page = sheet.page();
  const vp = await viewport(page);
  const position = await sheet.evaluate(
    (el) => el.ownerDocument.defaultView!.getComputedStyle(el).position,
  );
  expect(position, 'Sheet 应为 fixed').toBe('fixed');

  const box = await sheet.boundingBox();
  expect(box, 'Sheet 应可见').not.toBeNull();
  if (!box) return;
  expect(box.x + box.width, 'Sheet 右缘应贴视口右侧').toBeGreaterThanOrEqual(vp.width - EPS);
  expect(box.x, 'Sheet 应位于视口右半部').toBeGreaterThan(vp.width * 0.4);
  expect(box.width, 'Sheet 宽度').toBeGreaterThanOrEqual(320);
  expect(box.width, 'Sheet 宽度').toBeLessThanOrEqual(560);
  expect(box.height, 'Sheet 高度应铺满视口').toBeGreaterThanOrEqual(vp.height - EPS);
}

/** 对话框：fixed、居中（±2px）、宽 360–480、有圆角。 */
export async function expectDialogCentered(dialog: Locator): Promise<void> {
  const page = dialog.page();
  const vp = await viewport(page);
  const style = await dialog.evaluate((el) => ({
    position: el.ownerDocument.defaultView!.getComputedStyle(el).position,
    radius: parseFloat(el.ownerDocument.defaultView!.getComputedStyle(el).borderTopLeftRadius),
  }));
  expect(style.position, '对话框应为 fixed').toBe('fixed');
  expect(style.radius, '对话框应有圆角').toBeGreaterThan(0);

  const box = await dialog.boundingBox();
  expect(box, '对话框应可见').not.toBeNull();
  if (!box) return;
  expect(Math.abs(box.x + box.width / 2 - vp.width / 2), '水平居中').toBeLessThanOrEqual(EPS);
  expect(Math.abs(box.y + box.height / 2 - vp.height / 2), '垂直居中').toBeLessThanOrEqual(EPS);
  expect(box.width).toBeGreaterThanOrEqual(360);
  expect(box.width).toBeLessThanOrEqual(480);
}

/** 组件令牌：圆角/边框宽度，用于发现"工具类整体缺失"。 */
export async function expectRadius(
  locator: Locator,
  expected: number,
  tolerance = 1,
): Promise<void> {
  const radius = await locator.evaluate((el) =>
    parseFloat(el.ownerDocument.defaultView!.getComputedStyle(el).borderTopLeftRadius),
  );
  expect(Math.abs(radius - expected), `圆角应约为 ${expected}px`).toBeLessThanOrEqual(tolerance);
}

export async function expectPill(locator: Locator): Promise<void> {
  const radius = await locator.evaluate((el) =>
    parseFloat(el.ownerDocument.defaultView!.getComputedStyle(el).borderTopLeftRadius),
  );
  expect(radius, '胶囊按钮应为全圆角').toBeGreaterThanOrEqual(100);
}

export async function expectBorder(locator: Locator): Promise<void> {
  const width = await locator.evaluate((el) =>
    parseFloat(el.ownerDocument.defaultView!.getComputedStyle(el).borderTopWidth),
  );
  expect(width, '组件应有 1px 边框').toBeGreaterThan(0);
}

export async function expectSidebarWidth(sidebar: Locator, expected = 220): Promise<void> {
  const box = await sidebar.boundingBox();
  expect(box, '侧栏应可见').not.toBeNull();
  if (!box) return;
  expect(Math.abs(box.width - expected), '侧栏宽度').toBeLessThanOrEqual(2);
}
