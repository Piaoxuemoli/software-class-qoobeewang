# 截图基线清单

> 规则见 [docs/client/visual-regression.md](../../docs/client/visual-regression.md)。
> 机器可读的期望值在同目录 `baselines.json`；`make check` 的 `visual-audit` 会校验
> 两边与磁盘文件一一对应，并检查尺寸/背景/内容占比/唯一色数/盖章哈希。
> 更新基线只走 `make e2e-update BASELINE_REASON="..."`（自动盖章 + 审计），不需要逐张贴图。

| 文件                                                 | 范围 | 保护什么                                                   | 遮罩 / 动态项                                                       | 评审要点                                                  |
| ---------------------------------------------------- | ---- | ---------------------------------------------------------- | ------------------------------------------------------------------- | --------------------------------------------------------- |
| `app.spec.ts-snapshots/page-backup-linux.png`        | 整页 | 备份页空闲态布局：编排卡、六个筛选胶囊、开关、主按钮位置   | 无（mock 数据固定）                                                 | 卡片是否居中、胶囊是否换行错位、主按钮是否被裁切          |
| `app.spec.ts-snapshots/page-manage-linux.png`        | 整页 | 管理页空闲态：搜索行、表头、三行 mock 备份、完整性标记     | 无                                                                  | 列宽是否错位、状态色是否正常、空态是否误现                |
| `app.spec.ts-snapshots/page-settings-linux.png`      | 整页 | 设置页：仓库路径、动效开关、版本信息                       | 无                                                                  | 分区间距、输入框与按钮对齐；版本号变更时属预期            |
| `app.spec.ts-snapshots/backup-progress-linux.png`    | 整页 | 备份进行中：进度卡、计数、当前路径、取消按钮、侧栏迷你进度 | 状态图标、文件/大小计数、已用时间、百分比、进度条、当前路径、日志区 | 进度卡是否变形为 Sheet/块级元素；遮罩区域外不应有布局变化 |
| `app.spec.ts-snapshots/backup-done-linux.png`        | 整页 | 备份完成态：摘要六项、文件路径、两个操作按钮               | 无（mock 固定）                                                     | 摘要网格是否对齐、按钮是否重叠、焦点环是否异常            |
| `app.spec.ts-snapshots/restore-sheet-linux.png`      | 整页 | 管理页 + 详情 Sheet：Sheet 贴右、背景变暗、详情行与操作    | 无                                                                  | Sheet 是否脱离右侧、遮罩是否覆盖整页、是否出现水平滚动    |
| `app.spec.ts-snapshots/password-dialog-linux.png`    | 整页 | 加密备份还原时的口令对话框：居中、标题、说明、输入框、按钮 | 无                                                                  | 对话框是否居中、是否被 Sheet 遮挡、遮罩透明度             |
| `real.spec.ts-snapshots/real-backup-done-linux.png`  | 整页 | 真实后端完成卡：输出文件、压缩率、仓库路径的排版           | 耗时、输出文件、压缩率、仓库路径、临时路径                          | 数字格式与列宽；数值本身允许环境漂移，不作像素断言        |
| `layout.spec.ts-snapshots/sheet-detail-linux.png`    | 元素 | 详情 Sheet 本体：标题、徽标、八行元信息、删除/还原按钮     | 无                                                                  | 内边距、行距、按钮是否被裁切；几何由 L1 断言兜底          |
| `layout.spec.ts-snapshots/dialog-password-linux.png` | 元素 | 口令对话框本体：标题、说明、输入框、取消/开始还原          | 无                                                                  | 圆角、边框、按钮间距；几何由 L1 断言兜底                  |
| `layout.spec.ts-snapshots/popover-filter-linux.png`  | 元素 | 筛选 Popover 本体：标签、输入框、说明文字                  | 无                                                                  | 内边距、宽度（288px）、是否超出视口                       |
