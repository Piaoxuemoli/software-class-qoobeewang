#!/usr/bin/env bash
# DataBackup 演示视频 · 卡片段「真实数据采集」脚本（容器内运行）
#
# 做什么：
#   1) 在容器内把 /home/demo 预置成 07-…§2 的演示数据（约 500 MB：中文目录/软链接/FIFO/
#      待排除的 node_modules/可压缩文本 + 随机二进制）；
#   2) 用真实后端二进制跑一次真实备份（huffman + aes256）与真实还原；
#   3) 跑 seg-01 / seg-06 终端卡要显示的真实命令，把「展示用命令行 + 实际执行命令 +
#      原始 stdout + 退出码」原样采集到 course/04-视频/work/cards/raw/commands.txt。
#
# 硬约束（P2-3 任务）：终端卡里的每一个字符都必须来自这里采到的真实命令输出，
# 不允许在卡片 HTML/JS 里手写终端输出。
#
# 用法（databackup-dev 容器内，宿主入口见 run-in-docker.sh）：
#   bash /workspace/client/e2e/record-demo/cards/capture-real-output.sh
set -uo pipefail

REPO=/workspace
CARDS_OUT="$REPO/course/04-视频/work/cards"
RAW="$CARDS_OUT/raw"
BIN="$REPO/backend/bin/databackup"
DEMO_HOME=/home/demo
DEMO_PW=demo-2026
# 容器镜像没有 tzdata（TZ=Asia/Shanghai 会被忽略），用 POSIX TZ 直接表达 UTC+8，让 ls 显示本地时间
export TZ='UTC-8'

say() { printf '[capture] %s\n' "$*"; }
mkdir -p "$RAW"
: > "$RAW/commands.txt"
rm -f "$RAW"/rc-*.txt "$RAW"/stderr-*.txt

# ---------------------------------------------------------------------------
# 0. demo 用户
#    分镜脚本里的命令写作 `~/demo/documents`（07-…§4 S1/S6），所以把 demo 的 HOME
#    记为 /home：`~/demo/...` 展开为 /home/demo/...，与分镜命令文字逐字一致；
#    数据本身仍全部落在 /home/demo（任务要求的预置根目录）。
# ---------------------------------------------------------------------------
if ! id demo >/dev/null 2>&1; then
  useradd -u 1000 -d /home -M -s /bin/bash demo
fi
run_demo() {
  runuser -u demo -- env HOME=/home TZ="$TZ" LC_ALL=C \
    PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    bash -c "$1"
}

# ---------------------------------------------------------------------------
# 1. 预置 /home/demo（约 500 MB）
# ---------------------------------------------------------------------------
say 'seed /home/demo/documents'
rm -rf "$DEMO_HOME/documents" "$DEMO_HOME/documents-restored" "$DEMO_HOME/shared" "$DEMO_HOME/DataBackup"
mkdir -p "$DEMO_HOME/documents/报告" \
         "$DEMO_HOME/documents/合同" \
         "$DEMO_HOME/documents/资料/归档" \
         "$DEMO_HOME/documents/代码/site/node_modules/fake-lib" \
         "$DEMO_HOME/shared" \
         "$DEMO_HOME/DataBackup"

# 普通文件（顶层）
cat > "$DEMO_HOME/documents/README.md" <<'EOF'
# 演示资料目录

DataBackup 演示视频用的源目录：普通文件、两级子目录、软链接、命名管道，
以及一个待排除的 node_modules。
EOF
cat > "$DEMO_HOME/documents/notes.txt" <<'EOF'
备份前检查清单：源目录、排除规则、备份名称、压缩与加密选项。
EOF

# 报告/：2 个普通文件
for name in 周报 月报; do
  printf '# %s\n\n已完成：过滤、打包、压缩、加密流水线联调。\n待办：补全组合测试与文档。\n' "$name" \
    > "$DEMO_HOME/documents/报告/$name.md"
done

# 合同/：2 个 PDF（演示文件；只要求是真实文件，内容不参与画面）
make_pdf() { # <path> <rand-bytes>
  local out=$1 pad=$2
  {
    printf '%%PDF-1.4\n'
    printf '1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n'
    printf '2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n'
    printf '3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 595 842]>>endobj\n'
    printf 'trailer<</Size 4/Root 1 0 R>>\nstartxref\n0\n%%%%EOF\n'
    head -c "$pad" /dev/urandom
  } > "$out"
}
make_pdf "$DEMO_HOME/documents/合同/合同-A.pdf" 1048576
make_pdf "$DEMO_HOME/documents/合同/合同-B.pdf" 1048576

# 资料/归档/：300 个文本分片（每个约 536 KiB，可压缩）
TEMPLATE=/tmp/card-shard-template.txt
{
  for i in $(seq 1 4000); do
    printf '2026-10-18 08:%02d:%02d shard-line %05d 备份任务日志：扫描、打包、压缩、加密流水线正常，校验通过。\n' \
      $(( (i / 60) % 60 )) $(( i % 60 )) "$i"
  done
} > "$TEMPLATE"
for i in $(seq 0 299); do
  cp "$TEMPLATE" "$DEMO_HOME/documents/资料/归档/part-$(printf '%03d' "$i").txt"
done
# 随机二进制（不可压缩，约 340 MiB；加文本分片后总大小约 500 MB，与 07-…§2 一致）
dd if=/dev/urandom of="$DEMO_HOME/documents/资料/blob.bin" bs=1M count=340 status=none

# 代码/site/node_modules/：待排除目录（--exclude-dir 代码/site/node_modules）
printf 'console.log("excluded");\n' > "$DEMO_HOME/documents/代码/site/node_modules/fake-lib/index.js"
printf '{ "name": "fake-lib", "version": "0.0.0" }\n' > "$DEMO_HOME/documents/代码/site/node_modules/fake-lib/package.json"
printf '<!doctype html><title>site</title>\n' > "$DEMO_HOME/documents/代码/site/index.html"

# 软链接 → /home/demo/shared；FIFO
ln -s /home/demo/shared "$DEMO_HOME/documents/docs-link"
mkfifo "$DEMO_HOME/documents/pipe"
printf '共享目录说明：软链接目标在源目录之外，备份时按链接原样保存。\n' > "$DEMO_HOME/shared/共享说明.txt"

chown -R demo:demo "$DEMO_HOME"
SOURCE_HUMAN=$(du -sh "$DEMO_HOME/documents" | cut -f1)
SOURCE_BYTES=$(du -sb "$DEMO_HOME/documents" | cut -f1)
SOURCE_FILES=$(find "$DEMO_HOME/documents" -type f | wc -l)
say "seed done: $SOURCE_HUMAN / $SOURCE_FILES files"

# ---------------------------------------------------------------------------
# 2. 采集辅助：块格式
#   ### CMD <id>  展示用命令行（可多行，第二行起为 shell 续行）
#   ### EXEC <id> 实际执行的一条命令
#   ### OUT <id>  原始 stdout（空输出时该块无内容）
#   ### END <id> rc=<退出码>
# ---------------------------------------------------------------------------
CMDS="$RAW/commands.txt"
capture() { # <id> <display-cmd> <actual-cmd>
  local id=$1 display=$2 actual=$3
  printf '### CMD %s\n%s\n### EXEC %s\n%s\n### OUT %s\n' "$id" "$display" "$id" "$actual" "$id" >> "$CMDS"
  local out rc
  out=$(run_demo "$actual" 2>"$RAW/stderr-$id.txt")
  rc=$?
  [ -n "$out" ] && printf '%s\n' "$out" >> "$CMDS"
  printf '### END %s rc=%s\n' "$id" "$rc" >> "$CMDS"
  printf '%s' "$rc" > "$RAW/rc-$id.txt"
  say "captured $id (exit $rc, $(printf '%s' "$out" | wc -l) output lines)"
}

# ---------------------------------------------------------------------------
# 3. seg-01 终端卡：真实 ls / find / 空仓库
# ---------------------------------------------------------------------------
capture seg01-lsla \
  '$ ls -la ~/demo/documents' \
  'ls -la ~/demo/documents'
capture seg01-node-modules \
  '$ find ~/demo/documents -maxdepth 4 -type d -name node_modules' \
  'find ~/demo/documents -maxdepth 4 -type d -name node_modules'
capture seg01-ls-repo \
  '$ ls ~/demo/DataBackup' \
  'ls ~/demo/DataBackup'

# ---------------------------------------------------------------------------
# 4. 真实备份 + 真实还原（huffman + aes256；与 S3/S5 分镜一致）
# ---------------------------------------------------------------------------
BACKUP_CMD="echo $DEMO_PW | $BIN backup --source /home/demo/documents --repo /home/demo/DataBackup --name documents-demo --exclude-dir 代码/site/node_modules --compress huffman --encrypt aes256 --password-stdin"
RESTORE_CMD="echo $DEMO_PW | $BIN restore --backup documents-demo --repo /home/demo/DataBackup --target /home/demo/documents-restored --password-stdin"

say 'real backup (huffman + aes256) as demo ...'
T0=$(date +%s)
capture backup \
  '$ databackup backup --source ~/demo/documents --repo ~/demo/DataBackup --name documents-demo --exclude-dir 代码/site/node_modules --compress huffman --encrypt aes256 --password-stdin' \
  "$BACKUP_CMD"
BACKUP_SECONDS=$(( $(date +%s) - T0 ))
say "backup done in ${BACKUP_SECONDS}s"

say 'real restore as demo ...'
T0=$(date +%s)
capture restore \
  '$ databackup restore --backup documents-demo --repo ~/demo/DataBackup --target ~/demo/documents-restored --password-stdin' \
  "$RESTORE_CMD"
RESTORE_SECONDS=$(( $(date +%s) - T0 ))
say "restore done in ${RESTORE_SECONDS}s"

# ---------------------------------------------------------------------------
# 5. seg-06 终端卡：真实 diff（无输出）+ 真实 .dbk 列表
#    注意：GNU diffutils 3.8 对 FIFO 恒输出 "File ... is a fifo while file ..."，
#    即便两侧都是 FIFO（见 seg06-diff-strict 证据）。视频里要「真实无输出」，
#    因此严格版命令额外 --exclude pipe（分镜里只写了 --exclude node_modules）。
# ---------------------------------------------------------------------------
capture seg06-diff-strict \
  '$ diff -r ~/demo/documents ~/demo/documents-restored --exclude node_modules' \
  'diff -r ~/demo/documents ~/demo/documents-restored --exclude node_modules'

printf '### CMD seg06-diff\n$ diff -r ~/demo/documents ~/demo/documents-restored \\\n      --exclude node_modules --exclude pipe\n### EXEC seg06-diff\ndiff -r ~/demo/documents ~/demo/documents-restored --exclude node_modules --exclude pipe\n### OUT seg06-diff\n' >> "$CMDS"
DIFF_OUT=$(run_demo 'diff -r ~/demo/documents ~/demo/documents-restored --exclude node_modules --exclude pipe' 2>"$RAW/stderr-seg06-diff.txt")
DIFF_RC=$?
[ -n "$DIFF_OUT" ] && printf '%s\n' "$DIFF_OUT" >> "$CMDS"
printf '### END seg06-diff rc=%s\n' "$DIFF_RC" >> "$CMDS"
printf '%s' "$DIFF_RC" > "$RAW/rc-seg06-diff.txt"
say "captured seg06-diff (exit $DIFF_RC, $(printf '%s' "$DIFF_OUT" | wc -l) output lines)"

capture seg06-ls-dbk \
  '$ ls -l ~/demo/DataBackup/*.dbk' \
  'ls -l ~/demo/DataBackup/*.dbk'

# FIFO 类型一致性（Lead P2 更正 #1：diff 需 --exclude pipe；FIFO 用 test -p 单独证明还原正确）
capture seg06-fifo-check \
  '$ test -p ~/demo/documents-restored/pipe && echo "FIFO 类型一致"' \
  'test -p ~/demo/documents-restored/pipe && echo "FIFO 类型一致"'

# ---------------------------------------------------------------------------
# 6. 证据：环境事实 + 字体 + 采集结果索引（供 build-scenes.mjs 与人工复核）
# ---------------------------------------------------------------------------
DBK="$DEMO_HOME/DataBackup/documents-demo.dbk"
FACTS="$RAW/facts.txt"
{
  printf '# key<TAB>value（capture-real-output.sh 生成）\n'
  printf 'captured_at\t%s\n' "$(date -Is)"
  printf 'container_hostname\t%s\n' "$(hostname)"
  printf 'backend_version\t%s\n' "$("$BIN" --version)"
  printf 'backend_sha256\t%s\n' "$(sha256sum "$BIN" | cut -d' ' -f1)"
  printf 'demo_uid\t%s\n' "$(id -u demo)"
  printf 'demo_group\t%s\n' "$(id -gn demo)"
  printf 'demo_home\t/home/demo\n'
  printf 'home_for_tilde\t/home\n'
  printf 'tz\t%s\n' "$TZ"
  printf 'source\t/home/demo/documents\n'
  printf 'source_human\t%s\n' "$SOURCE_HUMAN"
  printf 'source_bytes\t%s\n' "$SOURCE_BYTES"
  printf 'source_files\t%s\n' "$SOURCE_FILES"
  printf 'source_dirs\t%s\n' "$(find "$DEMO_HOME/documents" -type d | wc -l)"
  printf 'excluded_dir\t代码/site/node_modules\n'
  printf 'excluded_regular_files\t%s\n' "$(find "$DEMO_HOME/documents/代码/site/node_modules" -type f | wc -l)"
  printf 'restored_regular_files\t%s\n' "$(find "$DEMO_HOME/documents-restored" -type f | wc -l)"
  printf 'restored_node_modules_entries\t%s\n' "$(find "$DEMO_HOME/documents-restored" -name node_modules | wc -l)"
  printf 'restored_symlink\t%s\n' "$(readlink "$DEMO_HOME/documents-restored/docs-link" 2>/dev/null || echo missing)"
  printf 'restored_fifo\t%s\n' "$([ -p "$DEMO_HOME/documents-restored/pipe" ] && echo yes || echo no)"
  printf 'restored_missing_vs_source\t%s\n' "$(find "$DEMO_HOME/documents" -type f | wc -l)-(excluded $(find "$DEMO_HOME/documents/代码/site/node_modules" -type f | wc -l))"
  printf 'backup_name\tdocuments-demo.dbk\n'
  printf 'backup_compress\thuffman\n'
  printf 'backup_encrypt\taes256\n'
  printf 'backup_seconds\t%s\n' "$BACKUP_SECONDS"
  printf 'restore_seconds\t%s\n' "$RESTORE_SECONDS"
  printf 'dbk_bytes\t%s\n' "$(stat -c %s "$DBK")"
  printf 'dbk_human\t%s\n' "$(du -h "$DBK" | cut -f1)"
  printf 'diff_strict_note\tGNU diffutils 3.8 对 FIFO 恒报 "is a fifo while ..."，故视频里的 diff 额外 --exclude pipe 才是真实无输出\n'
  for id in seg01-lsla seg01-node-modules seg01-ls-repo seg06-diff-strict seg06-diff seg06-fifo-check seg06-ls-dbk backup restore; do
    printf 'rc_%s\t%s\n' "$id" "$(cat "$RAW/rc-$id.txt" 2>/dev/null || echo NA)"
  done
  printf 'mono_fonts\t%s\n' "$(fc-list :spacing=mono family 2>/dev/null | sort -u | tr '\n' ';')"
  printf 'zh_fonts\t%s\n' "$(fc-list :lang=zh family 2>/dev/null | sort -u | head -8 | tr '\n' ';')"
  printf 'noto_cjk_path\t%s\n' "$(fc-list :lang=zh file 2>/dev/null | grep -i 'NotoSansCJK' | head -2 | tr '\n' ';')"
  printf 'playwright_chromium\t%s\n' "$(ls -d "$REPO"/course/04-视频/tools/ms-playwright/chromium-* 2>/dev/null | head -1)"
  printf 'git_head\t%s\n' "$(cd "$REPO" && git rev-parse --short HEAD 2>/dev/null || echo NA)"
} > "$FACTS"

say "raw evidence: $RAW/commands.txt ($(wc -l < "$CMDS") lines), facts: $FACTS"
say 'capture-real-output.sh done'
