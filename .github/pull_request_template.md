## 关联任务

任务 ID: <!-- 例如 M2-3，见 docs/roadmap.md -->

## 变更说明

<!-- 做了什么、为什么这么做；接口/行为变化请点名 -->

## 测试

- 新增/修改用例: <!-- 列出测试文件与用例 ID -->
- `make check` 结果: <!-- 粘贴通过输出；如跑了 make check-all 请注明 -->

## 界面基线（自动门禁）

- [ ] `make visual-audit` 通过（尺寸 / 深色背景 / 内容占比 / 唯一色数 / 清单覆盖 / 盖章哈希）
- [ ] `make e2e` 通过（结构契约 + 像素比对）
- [ ] 基线有变更时：`client/e2e/baseline-stamp.json` 已由 `make e2e-update BASELINE_REASON="..."` 更新
  - 变更原因: <!-- 一句话；不需要逐张贴图 -->
- [ ] 本次无界面改动、无基线变更（与上一项二选一）

## 文档同步

- [ ] 需求说明书 (`docs/deliverables/01-需求分析说明书.md`) 无需更新 / 已更新
- [ ] 系统设计文档 (`docs/deliverables/02-系统设计文档.md`) 无需更新 / 已更新
- [ ] 测试报告 (`docs/deliverables/03-软件测试报告.md`) 无需更新 / 已更新
- [ ] `CHANGELOG.md` `[Unreleased]` 已补充
- [ ] 未生成或提交任何 PDF（PDF 只在 M9-2 统一生成）
- [ ] 只修改了本任务"触碰范围"内的文件（见 `docs/roadmap.md`）

## 风险与回滚

<!-- 兼容性影响、回滚方式、遗留问题 -->
