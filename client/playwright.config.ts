import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  workers: 1,
  retries: 0,
  reporter: [['list']],
  expect: {
    // 基线生成与比对固定在 Docker 开发镜像里（字体、Xvfb、窗口尺寸固定）；
    // 动态数值由调用方 mask，不用阈值糊过去。见 docs/client/visual-regression.md。
    toHaveScreenshot: {
      animations: 'disabled',
      caret: 'hide',
      scale: 'css',
    },
  },
});
