# 踩坑手册（全程问题沉淀）

> 目的：把整个开发过程中真实踩过的坑按"现象 → 根因 → 对策 → 预防"沉淀下来，**同类问题不再花第二次排查时间**。新 agent / 未来自己开工前先浏览一遍（已挂入 AGENTS.md 必读表）。
> 记录规则：每解决一个新坑就追加一节；同一根因多处表现合并记录。

## 1. 环境与工具链

### 1.1 本机工具层会"吃掉"一层反斜杠

- **现象**：heredoc 里写的 `\\n` 到 python 变成 `\n`（甚至 `\n` 变成 `n`）；sed 的 `\\n` 模式匹配不到文件里的字面 `\n`；puml/testscript 里带反斜杠的字符串替换屡屡断言失败。
- **根因**：命令经工具调用层传递时转义被剥一层（Git Bash heredoc 本身不带，是外层干的）。
- **对策**：python 里用 `chr(92)` 构造反斜杠；**含反斜杠/复杂转义的内容一律用 Write/Edit 工具写文件**，不走 shell 内联。
- **预防**：凡是替换串里有 `\` 的脚本，先跑一次"模式存在性自检"再动手；失败先怀疑转义而不是逻辑。

### 1.2 python `io.open(path, 'w')` 的截断陷阱

- **现象**：一次带 bug 的批量子替换（`newline` 参数写错）把目标文件截断成空，且异常发生在截断之后。
- **根因**：`open(path,'w')` 打开即清空文件，参数校验错误在其后才抛出；循环中途抛异常 = 半途而废 + 已损坏。
- **对策**：批量改文件先 `read` 全部、内存中替换、**全部 assert 通过后**再统一写回；或写临时文件成功后 `os.replace`。
- **预防**：凡"循环里 open('w')"的写法都改成两阶段；改完 `git diff` 必看一眼再提交。

### 1.3 WSL 间歇性 `Wsl/Service/CreateInstance/E_FAIL`

- **现象**：`wsl.exe` 调用突然全部 E_FAIL，几分钟后自愈。
- **根因**：WSL 服务繁忙/重启中。**千万不要 `wsl --shutdown`**——会把 docker-desktop 发行版一起杀掉，Docker Desktop 门户全断。
- **对策**：等几分钟重试；期间把可迁移的活转 Docker 容器跑（基准/分析类任务等价）。
- **预防**：WSL 任务设计成可重试、幂等；不依赖 WSL 处于常驻状态。

### 1.4 WSL `/tmp` 跨调用不保命

- **现象**：上一次 `wsl.exe` 生成的 /tmp 中间产物，下一次调用神秘消失（备份"不存在"、还原失败）。
- **根因**：WSL 空闲自动停机 → 下次调用重新启动 → systemd 开机清理 /tmp。
- **对策**：跨调用的中间产物放 `~` 下；**多步骤验证合并进同一次 wsl.exe 调用**。
- **预防**：凡写 WSL 操作文档/脚本，默认路径用 `~`。

### 1.5 WSL 读不了 Windows worktree 的 .git

- **现象**：`git fetch <windows路径>` 报 "not a git repository: .../C:/Users/..."。
- **根因**：worktree 的 `.git` 是指向主仓的**文件**，内容是 Windows 风格绝对路径，WSL 的 git 无法解析。
- **对策**：不搞跨界 git——直接 `cp` 需要的文件进 WSL 工作副本，或从 GitHub 拉取。

### 1.6 Git Bash 的 MSYS 路径改写

- **现象**：`docker run -v /workspace ...` 之类参数被改写成 Windows 路径报错；中文路径在 docker 挂载里出问题。
- **对策**：前缀 `MSYS_NO_PATHCONV=1`；挂载源路径用 `cygpath -w` 转换；docker.sh 内部已处理，优先走 docker.sh。
- **预防**：裸 docker 命令清单先在脑内过一遍"哪些参数像 POSIX 路径"。

### 1.7 新 worktree / 损坏的 node 卷

- **现象**：新 worktree 跑 e2e 报 `electron-vite: not found`；某 worktree 跑 e2e 报 `Electron failed to install correctly` + `fetch failed`。
- **根因**：每个 worktree 独立 node 卷（M0-5 设计），新卷没装依赖；旧卷里 electron 二进制（dist/）损坏或缺失。
- **对策**：新 worktree 首跑 `bash scripts/docker.sh make client-install e2e`；坏卷 `bash scripts/docker.sh clean-volumes` 后重装（npm 缓存热，约 1 分钟）。
- **预防**：见过的两种报错直接对应这两条处置，不必再排查。

## 2. CI 与发布

### 2.1 GitHub Release 资产名剥中文（发布三连坑之一）

- **现象**：`gh release create` 携带中文名文件稳定 422 `ReleaseAsset.name already exists`；取证发现已上传资产名被剥掉中文（`01-需求分析说明书.pdf` → `01-.pdf`，全中文名塌缩成 `default.pdf` 互相碰撞）。
- **根因**：GitHub 上传通道对非 ASCII 资产名做剥离/替换，归一化后同名冲突。
- **对策**（已固化进 release.yml）：组装产物时把 PDF 复制为 ASCII 名；`gh release upload --clobber` 幂等补传。
- **预防**：**一切对外上传的文件名用 ASCII**；仓库内路径/文件名可继续中文。

### 2.2 docs 容器 root 属主污染（发布三连坑之二）

- **现象**：发布流水线 Assemble 步骤写 `build/release-notes.md` Permission denied。
- **根因**：docs 步骤的 `docker run` 以 root 创建 `build/`，后续 runner 用户不可写。
- **对策**（已固化）：release notes 写 `$RUNNER_TEMP`。
- **预防**：工作流里 docker 步骤与宿主步骤共享目录时，想清楚属主；输出走 `$RUNNER_TEMP` 最稳。

### 2.3 失败的 `gh release create` 会自回滚（发布三连坑之三）

- **现象**：失败后 `gh release view` 说 release not found，但错误日志分明在往某个 release 传资产。
- **根因**：gh CLI 在资产上传失败时回滚自己刚创建的 Release。
- **预防**：排障时先 `gh release list` 看真实状态再决定删/重跑；**修完工作流必须移动 tag 重推**（rerun 用的是旧工作流文件）。

### 2.4 e2e 截图基线是"环境锁定"的

- **现象**：本地容器两遍全绿的截图用例，CI 恒差 71px；修一轮"随机数"后 CI 仍差**恰好同样** 71px。
- **根因链**（本项目最重要的一课）：完成卡"输出文件/压缩率" = f(Huffman(打包流))，**打包流内嵌 mtime 纳秒与 uid/gid 等环境元数据**——同一份确定性源数据在 root 容器 ≈5508B、uid1000 ≈5519B、CI runner ≈5591B：逐次稳定、跨环境恒定漂移，像素断言天生不可行。
- **对策**（已固化）：环境熵源统计改 mask + 正则格式断言，精确值由后端矩阵用例保证；真随机夹具改确定性伪随机（xorshift32）消除逐次漂移。
- **预防**：**截图断言前先问"这个值依赖哪些环境输入"**；排查方法论：失败次数恒定 ⇒ 系统性差异；先取 CI 产物（error-context.md 直接可读失败时页面文本）+ 像素级 diff 坐标取证，再动手改。
- **配套**：ci.yml 已有 e2e 失败自动上传 test-results——排障时善用。

### 2.5 手动 Xvfb 必须复刻 xvfb-run 默认参数

- **现象**：绕开 xvfb-run 挂起竞态自管 Xvfb 时，app.spec 截图整批尺寸不符。
- **根因**：给了 `-screen 0 640x480x8`；Ubuntu 的 xvfb-run 默认是 `-screen 0 1280x1024x24`，全部基线在该环境生成。
- **预防**：任何替代默认执行方式的操作，先查默认值（`grep XVFBARGS /usr/bin/xvfb-run`）。

### 2.6 版本号 bump 是全链路连锁

- **现象**：0.1.0→1.0.0 后 CI e2e 红。
- **根因**：版本号印在 UI 上（侧栏客户端版本、状态栏后端版本）→ **截图基线含版本**；另有表驱动断言、e2e 断言、testscript（注意是 `.txt` 不是 `.txtar`，且期望里是正则转义的 `0\.1\.0`）、package-lock 两处。
- **对策**（清单化）：version.go / package.json / package-lock×2 / internal-cli 与 backend-e2e 断言 / TC-CLI-274 / fakeApi 与 App.test / 截图基线（用 --update-snapshots 重建）。
- **预防**：bump 版本照此清单过一遍；grep 时别带 `--include` 过滤器直接搜全目录。

### 2.7 其它 CI/构建坑

- **install-deps 双坑**（#33/#34）：`apt-cache show` 对纯虚拟包也返回 0（noble 的 libasound2）→ 用 `apt-cache policy` 查候选；且 `set -o pipefail` 下 `grep -q` 命中即退会 SIGPIPE 前置命令、整管道判负 → **先捕获输出再 herestring 匹配**。
- **PlantUML `skinparam padding` 已弃用**：警告会被渲染进图片 → 用 `<style>` CSS 语法。
- **emoji（🔴🟡🟢）在 LibreOffice 渲染乱码**：PPT/文档正文用文字（强/中/轻）。
- **soffice 加载中文名文件失败**：容器内先 `cp` 成 ASCII 名再转换。
- **make coverage** 的 html 步骤曾跑错目录（已在 #36 修）；electron-builder 26 无 `linux.arch` 键、deb 目标强制 author/maintainer 元数据（#35）。

## 3. 协作与流程

### 3.1 判断合并状态先 `git fetch`

- 本地 master 常落后远端；"分支未合并"的误判会引发多余 rebase。先 fetch 再看 origin/master。
### 3.2 commit 前看清当前分支

- 一次修复 commit 直接落在了本地 master（推送被分支保护拦下，未外泄）。`git branch --show-current` 应成为肌肉记忆；分支保护是最后兜底不是借口。
### 3.3 变异性实验要控制变量

- 排查输出大小漂移时首轮实验用了真随机数据，把"内容熵"与"元数据熵"混在一起，结论差点走偏。改用固定内容后才分离出 mtime/uid 元数据这一真因。
### 3.4 二进制文件的 grep 命中不是渲染文本

- PNG 里 grep 到字符串（如警告文案）≠ 图里有该文字，判断渲染内容必须看图/产物。

---
*维护：本文件由踩坑人即时追加；合并 PR 前自问"这次有没有新坑值得记录"。*
