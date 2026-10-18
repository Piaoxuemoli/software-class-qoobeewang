# 界面回归：截图基线的范围与评审规则

> 适用：`client/e2e/` 里的 Playwright 截图用例与基线（`*-snapshots/*.png`）。
> 本文替代 `ui-design.md` §7 里"每页各状态一张基准图"的旧说法；改界面、改基线前先读本文。

## 1. 为什么要改

v1.0.0 发布后复核截图时发现：备份详情 Sheet 与口令对话框的样式整体丢失（Tailwind 只扫描了 `src/renderer`，`src/components/ui` 独占的工具类没生成），页面把 Sheet 排进文档流、对话框落到左下角。**这 7 张有问题的截图本身就是基线**，而用例只断言 `toBeVisible()`，于是 e2e 一直是绿的（`pitfalls.md` §2.8 记录了根因）。

问题不在"截图对比"本身，而在三件事：

1. 只有整页截图，没有"结构契约"——样式整体失效时，元素仍然 attached/visible；
2. 基线可以随手 `--update-snapshots` 全量覆盖，没有人逐张核对"这张图应该是这样吗"；
3. 基线没有清单，没人说得清每张图保护的是什么。

## 2. 调研结论

- Playwright 官方文档明确：截图像素受操作系统、字体、硬件影响，必须在生成基线的同一环境跑，并且**提交到版本库的基线变更需要 review**（[Visual comparisons](https://playwright.dev/docs/test-snapshots)）。
- TestChimp 的 E2E 反模式清单指出：整页快照极其脆弱，PR 里一出现几十行 diff，`-u` 就会变成默认操作，reviewer 直接点接受，回归被掩盖；推荐**优先用角色/测试 ID 做结构断言，快照只留给稳定的组件**（[Snapshot testing is too brittle for E2E](https://docs.testchimp.io/guides/gotchas/snapshot-testing-too-brittle/)）。

两条结论合起来：**结构断言负责"不能坏"，范围快照负责"看着没变"**，两者缺一不可。

## 3. 四层方案

### L0 环境锁定（不变）

- 只在 Docker 开发镜像（`scripts/docker.sh`）里生成/比对基线：固定字体（Noto CJK）、固定 Xvfb `1280x800x24`、窗口 `1120x720`。
- 截图默认 `animations: 'disabled'`、`caret: 'hide'`；动态数值用 `mask` 遮罩，不用"看起来差不多"的阈值糊过去。

### L1 结构契约（新增，强制）

用计算样式与几何断言覆盖"样式整体失效"这类故障，不依赖像素：

| 契约 | 断言 | 保护什么 |
|---|---|---|
| 无水平溢出 | `documentElement.scrollWidth ≤ innerWidth + 1` | 元素被撑出视口、Sheet 掉进文档流 |
| Sheet 几何 | `position: fixed`；右边缘贴视口右缘（±2px）；宽 320–560；高≈视口 | 右侧抽屉失效 |
| 遮罩层 | `position: fixed`、覆盖视口、`rgba(0,0,0,α)` 且 α ≥ 0.3 | 遮罩类丢失（点击穿透、层级错乱） |
| 对话框 | 中心与视口中心偏差 ≤2px；宽 360–480；圆角 >0 | 对话框变成普通块级元素 |
| 组件令牌 | 主按钮胶囊圆角、输入框 8px、卡片 12px、边框宽度 >0 | Tailwind 工具类整体缺失 |

实现：`client/e2e/layout.ts` 提供断言函数，`client/e2e/layout.spec.ts` 逐项跑（TC-E2E-10 ~ 12）。这类用例不产生图片，故障信息直接给坐标/计算值。

### L2 范围快照（收敛）

快照分三类，命名固定：

| 前缀 | 范围 | 数量 | 用途 |
|---|---|---|---|
| `page-*.png` | 整页空闲态（备份 / 管理 / 设置） | 3 | 页面级布局与视觉总览 |
| `state-*.png` | 关键状态（进行中、完成、取消失败） | 2–3 | 任务状态机的可见差异 |
| `sheet-* / dialog-* / popover-*` | 浮层元素截图（只截元素本身） | 4 | 浮层样式与内容；失败时一眼定位到组件 |

原则：**能用元素截图就不用整页**；整页只保留"这一屏长什么样"的总览，不承担细节断言。

### L3 业务断言（保留）

文本、数量、格式、文件系统结果仍由现有断言负责（例如输出大小用正则、还原后用目录树比较）。截图不是业务正确性的证据。

## 4. 自动化门禁（取代人工逐张看图）

基线相关的人工步骤尽量压缩成两条命令能完成的事，CI 用脚本判定：

| 门禁 | 脚本 | 检查什么 | 能拦住的问题 |
|---|---|---|---|
| 结构契约 | `client/e2e/layout.spec.ts` | 元素几何、`position`、遮罩透明度、组件圆角/边框、页面是否水平溢出 | 样式整体失效、Sheet 掉进文档流、对话框错位 |
| 视觉审计 | `scripts/visual-audit.mjs` | 尺寸、深色背景占比、内容像素占比、唯一色数、平均亮度、清单覆盖、基线哈希 | 空白图、白底、截图被手工替换、新增截图漏登记 |
| 盖章 | `scripts/baseline-stamp.mjs` | 记录更新原因、时间与每张图 sha256 | 绕过 `make e2e-update` 直接覆盖 PNG |
| 像素比对 | Playwright `toHaveScreenshot` | 与基线逐像素比较（动态值已 mask） | 未预期的视觉变化 |

`scripts/visual-audit.mjs` 自己解 PNG（`node:zlib` + 反滤波），不引入第三方依赖；阈值在
`client/e2e/baselines.json` 里按图配置（整页 1120×720、浮层元素按实际尺寸；深色界面
背景占比 ≥0.45、内容占比有下限也有上限、唯一色数 ≥8）。阈值是"这个应用至少长这样"的
下限，不是审美判断。

自动化覆盖不到的部分（文案改错、业务含义变化）由 PR 的"变更原因"一行说明，不再要求
逐张贴 before/after 截图——需要看时，CI 的 `test-results/` 里有 actual/expected/diff。

## 5. 更新流程

```bash
# 容器内（Windows 主机用 bash scripts/docker.sh make e2e-update）
make e2e-update BASELINE_REASON="一句话说明这次为什么变"
```

脚本按顺序执行：`--update-snapshots` → 写入盖章文件 → 跑视觉审计 → 列出变化的文件。
审计不通过就直接失败，不会留下"已经改好但没人发现"的基线。随后只需在 PR 里写一句
变更原因；新增/删除基线时同步更新 `client/e2e/baselines.json` 与 `baselines.md`
（两边都登记，审计会校验一一对应）。

## 6. 验收（全部自动）

- `make check`：包含 `visual-audit`（清单、尺寸、内容比例、盖章哈希）。
- `make e2e`：结构契约 + 像素比对；失败时上传 `test-results/`。
- CI 的 client job 跑 `visual-audit`，e2e job 跑完整截图对比。
- PR 模板只要求勾"自动门禁通过"与一句变更原因，不再要求逐张人工核对。

## 7. 与旧规范的差异

| 旧 | 新 |
|---|---|
| 每页各状态一张整页图 | 整页只留 3 张空闲态；状态与浮层用元素截图 |
| 只断言 `toBeVisible()` | 增加计算样式/几何契约（样式失效也能发现） |
| 基线可随时全量更新 | 只能经 `make e2e-update` 更新，自动盖章并校验哈希 |
| PR 要求逐张人工看图 | 自动审计 + 像素比对作为门禁；人工只需一句变更原因 |
