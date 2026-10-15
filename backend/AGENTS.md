# backend/AGENTS.md

Go 后端的补充规则（全局规则见根目录 `AGENTS.md`）。

## 结构

- `cmd/databackup/`：入口，只调用 `internal/cli.Run`。
- `internal/<包>/`：每个包一个概念，划分与依赖方向见 `docs/architecture/overview.md` §2。原语包（`pack`、`huffman`、`crypto/*`）不得依赖 `engine`、`cli`。
- `internal/testutil/`：测试专用的声明式目录树与比较器，所有往返测试都用它。
- `e2e/`：`//go:build e2e`，构建真实二进制后黑盒测试。

## 必须遵守

- 生产代码只用标准库；禁用 import 由 `scripts/check-imports.sh` 检查（见根 `AGENTS.md` §3）。
- 流式处理：用户文件一律 `io.Reader`/`io.Writer` 串联，不整体读入内存。
- 归档/容器格式字段长度、魔数、块大小定义为具名常量并注明来源；格式变化必须同步 `docs/architecture/mypack-format.md` 与 golden 文件。
- 面向用户的错误用包级哨兵错误（如 `container.ErrAuth`），由 `cli` 统一映射为退出码与 `error.code`（`docs/architecture/cli-contract.md` §4）。
- `--json` 模式下 stdout 只能输出 JSONL；任何人类可读输出走 stderr。
- 需要 root 的测试用 `t.Skip` 守护；测试只在 `t.TempDir()` 中读写。
- `internal/engine/` 只属于 roadmap W3 集成泳道。

## 自检

```bash
make lint backend-test          # 容器内
bash scripts/docker.sh make lint backend-test   # Windows 主机
```
