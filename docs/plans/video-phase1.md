# 演示视频 P1：AI 配音方案与试听（IndexTTS-2.5 零样本）

> 对应 [视频任务交接 §5](../project/video-handoff.md) 的 P1 门禁：**选定模型 + 部署环境 + 3 句试听 + 语速/音色参数**。
> 关联：[分镜与操作脚本 §5–§7](../deliverables/07-演示视频分镜与操作脚本.md)、[AI 配音选型 §5.2/§7.1/§8](../research/2026-10-18-AI配音选型.md)、[分镜 JSON](../deliverables/video-storyboard.json)。
> 日期：2026-10-18。**状态：待用户试听确认（P1 门禁未过，不进入 P2）**。

## 1. 结论摘要

| 项 | 结论 |
|---|---|
| 方案 | **IndexTTS-2.5 零样本音色克隆**（0.8B，2026-08-10） |
| 代码版本 | `index-tts` commit `d9e41aa`（2026-09-30，fix: 末尾加淡出，避免停止符采样偏早时留下咔哒声 #806） |
| 权重 | `IndexTeam/IndexTTS-2.5`（ModelScope 镜像），主权重 5.2 GB + 首次推理自动下载辅助模型 5.2 GB |
| 推理硬件 | **AutoDL 租用 GPU：RTX 4090 24 GB**，Ubuntu 22.04.4，驱动 595.71.05，CUDA 13.2（208 核 / 1 TB 内存） |
| 显存占用 | **峰值 6.6 GB**（bf16，见 §9；8 GB 卡理论上够，但租用 4090 余量充足） |
| 参考音色 | **AISHELL-1 男声 `S0101`**，Apache-2.0，10.29 s prompt，F0 中位数 ≈116–122 Hz |
| 语速参数 | 基线 **`duration_factor=1.10`**；实测 4.53 字/秒（目标 4.2–4.6），详见 §8 |
| 试听结论 | 客观指标通过（3 句完整、数字读法正确、单句时长可测）；**音色与语速主观项待用户确认** |
| 输出规格 | 22050 Hz / 单声道 / 16-bit PCM，逐句一个 WAV（后期统一转 48k/24bit + loudnorm） |

用户已确认的决策（2026-10-18）：TTS 用 IndexTTS-2.5；**不使用本人录音**，改用开源语料男声；GPU 用用户租用机、SSH 访问；**不在本机部署**。

## 2. 选型依据

- 调研文档 §6.3 的第一选择：~6 GB 显存、`uv sync` 一条命令、`duration_factor` 是同类里唯一数值化语速控制、拼音注音治多音字、长文自动分段；bilibili Model Use License 允许课程公开展示（仅"月活 >1 亿或年营收 >10 亿元"需单独授权）。
- 本项目 P1/P2 的核心痛点是**"按字幕卡时长"**，这正好是 `duration_factor` 的强项——实测确认它能直接改变输出时长（§8）。
- 备选未采用的代价：GPT-SoVITS 需要 30–60 分钟微调流程（但能把 100 s 样本用满，见 §10 的兜底）；CosyVoice3 装 pynini/sox 折腾且语速只能指令式。

## 3. 部署环境与实测命令

机器：AutoDL 容器 `/root/autodl-tmp`（数据盘 50 GB）。工作目录：

| 路径 | 内容 |
|---|---|
| `/root/autodl-tmp/index-tts` | 代码（commit `d9e41aa`）+ `.venv` + `checkpoints/` |
| `/root/autodl-tmp/py311` | conda 建的 Python 3.11.17（供 uv 建 venv 用） |
| `/root/autodl-tmp/uv-cache` | uv 缓存（实测 22 GB，空间紧张时可 `uv cache clean`） |
| `/root/autodl-tmp/tools/` | `tts_batch_indextts.py`、`tts-test-lines.txt`、`ref/{main,backup}-ref.wav` |
| `/root/autodl-tmp/p1test`、`p1test-df110` | 3 句试听（duration_factor=1.0 / 1.10） |

实测版本：Python **3.11.17**（conda）、uv **0.8.16**、torch **2.8.0+cu128**、torchaudio **2.8.0+cu128**、transformers **4.52.1**、modelscope **1.29.2**。

一条命令复现：`bash scripts/video/setup_indextts_remote.sh`（脚本内已固化下面所有坑的规避方式）。

### 3.1 实测步骤与坑（按踩坑顺序）

1. **GitHub 直连超时**（`github.com` 直连 000）：用 `source /etc/network_turbo`（AutoDL 学术加速）或 `https://gh-proxy.com/<github-url>` 前缀 clone。
2. **`curl astral.sh/uv/install.sh` 卡在拉 GitHub 的 uv 二进制**：直接 `/root/miniconda3/bin/pip install uv -i https://mirrors.aliyun.com/pypi/simple`（装到 miniconda bin，PATH 自带）。
3. **`uv python install 3.11` 长时间卡在 `python-build-standalone` 下载（GitHub）**：改用 conda（镜像已配 TUNA）`conda create -y -p /root/autodl-tmp/py311 python=3.11`，再 `uv sync --python /root/autodl-tmp/py311/bin/python`。
4. **modelscope CLI 参数是 `--local_dir`（下划线）**：写 `--local-dir` 会直接 `unrecognized arguments`，下载不启动（这个坑卡过约 3 分钟）。
5. **`uv sync --all-extras` 会在 `accel` extra 里从源码编译 `flash-attn`**（极慢，本次直接杀掉）：**推理不需要 flash-attn**，用 `uv sync`（不带 extras）即可；`deepspeed` 同理只在训练/加速时才需要。
6. **BigVGAN 自定义 CUDA kernel 需要 ninja**：首次运行报 `Ninja is required...` 自动回退 torch 实现（RTF 0.41）；第二次运行编译成功后 RTF 降到 0.356。**不影响正确性**。
7. **首次推理会自动下载辅助模型 5.2 GB**（w2v-bert-2.0 / campplus / semantic_codec / bigvgan）到 `checkpoints/hf_cache`；国内需 `HF_ENDPOINT=https://hf-mirror.com`（实测约 17 MB/s）。
8. **磁盘**：数据盘 50 GB，装完后 `checkpoints 11 GB + .venv 8.3 GB + uv-cache 22 GB ≈ 41 GB`（剩 9.5 GB）。**批量合成前先 `uv cache clean` 或删 uv-cache**，否则再下辅助模型会满。

### 3.2 已验证的关键命令

```bash
# 假设在 GPU 主机上，ROOT=/root/autodl-tmp
nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader
/root/miniconda3/bin/pip install -q uv modelscope -i https://mirrors.aliyun.com/pypi/simple
source /etc/network_turbo
git clone --depth 1 https://gh-proxy.com/https://github.com/index-tts/index-tts.git /root/autodl-tmp/index-tts
unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY
/root/miniconda3/bin/conda create -y -p /root/autodl-tmp/py311 python=3.11
cd /root/autodl-tmp/index-tts
export UV_CACHE_DIR=/root/autodl-tmp/uv-cache
uv sync --python /root/autodl-tmp/py311/bin/python --default-index https://mirrors.aliyun.com/pypi/simple
/root/miniconda3/bin/modelscope download --model IndexTeam/IndexTTS-2.5 \
  --local_dir /root/autodl-tmp/index-tts/checkpoints
./.venv/bin/python tools/gpu_check.py
```

## 4. 模型、权重与许可

- 主权重（`checkpoints/`，5.2 GB）：`gpt.pth` 3.04 GB、`qwen0.6bemo4-merge/` 1.11 GB、`codec.pth` 579 MB、`s2mel.pth` 396 MB、`config.yaml`、`pinyin.vocab`、`multilingual_zh_ja_yue_char_del.tiktoken`、`wav2vec2bert_stats.pt`、`feat1/2.pt`。
- 辅助模型（`checkpoints/hf_cache/`，5.2 GB）：`w2v-bert-2.0`、`campplus_cn_common.bin`、`semantic_codec*`、`bigvgan`。
- 许可：代码/权重 **bilibili Model Use License Agreement**（免费、允许课程公开展示；>1 亿 MAU 或年营收 >10 亿元需单独授权；禁止用输出去改进其他商用 AI 模型）。课程场景合规。
- 复现记录：`git log -1` = `d9e41aac89fd00b3d71497fddb287b7f24613712`。

## 5. 参考音频（克隆源）

| 项 | 内容 |
|---|---|
| 来源 | **AISHELL-1**（OpenSLR SLR33，北京壳壳科技），Apache License 2.0 |
| 说话人 | `train/S0101`，**男**（官方 `speaker.info` 标注 M；F0 中位数 115.9 Hz，p10–p90 = 100–140 Hz） |
| 主 prompt | `course/04-视频/voice/ref/main-ref.wav`：10.29 s，16 kHz/16-bit/mono，峰值 −4.00 dBFS，RMS −21.05 dBFS，估算 SNR 31.9 dB，0 削波 |
| 备用 prompt | `ref/backup-ref.wav`：8.09 s（同说话人不同句子） |
| 兜底长样本 | `raw/aishell1-S0101-full.wav`：82.51 s（17 句拼接），供将来 GPT-SoVITS 微调兜底 |
| 处理 | 去首尾静音 + 句间 0.30 s 静音 + 常数增益到 −4 dBFS + 10 ms 淡入出；**未降噪/EQ/压缩/变速** |
| 明细 | `course/04-视频/voice/SOURCE.md`（URL、SHA256、切分时间戳、候选对比、失败记录） |

**署名建议**（Apache-2.0 义务很轻，课程视频不重分发语料本体）：片尾或简介加一行
`语音参考：AISHELL-1 (Apache-2.0, OpenSLR SLR33) — 说话人 S0101`。

已知局限：16 kHz 源（带宽上限 8 kHz）；AISHELL 是新闻稿朗读，克隆旁白可能带一点播报感——**音色是否可用由用户试听决定**，不满意先换 `backup-ref.wav`。

## 6. P1 三句试听（门禁证据）

三句文本 `scripts/video/tts-test-lines.txt`（覆盖数字 256/18/32、英文 `DataBackup/Linux/Huffman/AES`、`.dbk` 写法）：

1. `DataBackup 把 Linux 目录树备份成一个文件，并原样还原。`
2. `开始备份。扫描、打包、Huffman 压缩、AES-256 加密依次执行。`
3. `目录树逐字节一致，备份产物是单个 .dbk 文件。18 条功能需求、32 组组合全部通过。`

合成命令（GPU 主机）：

```bash
cd /root/autodl-tmp/index-tts
export HF_ENDPOINT=https://hf-mirror.com
unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY
./.venv/bin/python /root/autodl-tmp/tools/tts_batch_indextts.py \
  --ref /root/autodl-tmp/tools/ref/main-ref.wav \
  --text-file /root/autodl-tmp/tools/tts-test-lines.txt \
  --out-dir /root/autodl-tmp/p1test --duration-factor 1.0
# 同样参数再跑一次 --out-dir /root/autodl-tmp/p1test-df110 --duration-factor 1.10
```

真实 API 签名（`--print-api` 实测，2.5 与调研文档一致）：

```
IndexTTS2.__init__(cfg_path='checkpoints/config.yaml', model_dir='checkpoints', use_bf16=False,
                   device=None, use_cuda_kernel=None, use_deepspeed=False, use_accel=False,
                   use_torch_compile=False, use_qwen_emo=False)
IndexTTS2.infer(spk_audio_prompt, text, output_path, lang, emo_audio_prompt=None, emo_alpha=1.0,
                emo_vector=None, use_emo_text=False, emo_text=None, use_random=False,
                interval_silence=200, verbose=False, max_text_tokens_per_segment=120,
                stream_return=False, more_segment_before=0, duration_factor=1.0,
                text_normalization=True, generation_kwargs)
```

批量脚本按签名过滤参数（`scripts/video/tts_batch_indextts.py`），因此小版本参数差异不会直接炸；
本次实际生效参数：`use_bf16=True`、`lang="ZH"`、`text_normalization=True`、`max_text_tokens_per_segment=120`、`interval_silence=200`。

### 6.1 时长与语速实测

逐句 WAV（22.05 kHz / mono / s16）；字数口径 = 非空白字符数（与分镜 §5 预算一致）。

| # | 字数 | df=1.0 时长 | df=1.0 字/秒 | df=1.10 时长 | df=1.10 字/秒 |
|---:|---:|---:|---:|---:|---:|
| 1 | 33 | 6.107 s | 5.40 | 6.36 s | 5.19 |
| 2 | 35 | 6.420 s | 5.45 | 6.85 s | 5.11 |
| 3 | 41 | 9.985 s | 4.11 | 10.84 s | 3.78 |
| 合计 | 109 | **22.51 s** | **4.84** | **24.06 s** | **4.53** |

> 第 3 句"看起来偏慢"是口径问题：`18`、`32`、`.dbk` 会被读成多个音节，而字数只记 2/2/4，所以中英混排句子用字符数估语速会低估。整轨口径更可靠，且 P2 会逐句按分镜预算对齐。

### 6.2 ASR 回译（rough check，whisper-small，仅作吞字粗筛）

- [0] `data backup把linux目录数备分成一个文件并原样还原`
- [1] `開始備份掃描打包Huffman壓縮AS256加密一次執行`
- [2] `目錄數逐字節移至,備份產物是單個Dbk文件,18條功能需求,32組組合全部通過`

判读：**3 句都完整、无整句漏读**；数字 `18/32` 正确；同音字误识（树→数、一致→移至、依次→一次）是 whisper-small 的正常水平，不能据此判 TTS 有错。**需用户用耳朵重点确认 `AES-256`（是否读成"A-E-S 二五六"）与 `.dbk`（是否读成"点 dbk"）**；若读法不理想，P2 改用中文写法（如"AES 二百五十六"、去掉前导点）或拼音注音。

### 6.3 试听文件（交付给用户）

| 文件 | 内容 |
|---|---|
| `course/04-视频/voice/p1test/listen.wav` | 3 句原速（df=1.0）拼接，23.2 s |
| `course/04-视频/voice/p1test-df110/listen.wav` | 3 句放慢（df=1.10）拼接，24.8 s |
| `course/04-视频/voice/p1test/seg_000..002.wav` | 逐句原速 |
| `course/04-视频/voice/p1test-df110/seg_000..002.wav` | 逐句放慢 |
| `course/04-视频/voice/ref/main-ref.wav` | 克隆源（先听这个确认音色） |
| `course/04-视频/voice/listen.html` | 一键试听页（浏览器直接打开） |
| `p1test/rate.json`、`p1test-df110/rate.json` | 语速统计明细 |

## 7. P1 门禁判定（对照 video-handoff §5）

| 门禁项 | 结果 |
|---|---|
| 选定模型 + 部署环境 | ✅ IndexTTS-2.5 @ 4090，命令已实测（§3） |
| 3 句试听、含数字/英文 Huffman/AES-256 | ✅ 已生成（§6） |
| 中文自然、无吞字 | ⏳ 客观指标通过；**待用户主观确认** |
| 单句时长可测 | ✅ `measure_rate.py` 实测（§6.1） |
| 语速/音色参数确定 | ✅ 语速 `duration_factor=1.10` 基线；音色待确认 |

## 8. 语速参数结论

- 实测映射：`duration_factor` **1.0 → 4.84 字/秒**、**1.10 → 4.53 字/秒**（目标 4.2–4.6，1.10 已入窗）。注意它**不是线性**：+10% 参数只换来约 +6.9% 时长，所以不要拿"目标/实测"直接当系数。
- **P1 建议基线：`duration_factor=1.10`**；若用户觉得仍偏快，P2 逐句试 1.15–1.20（每 +0.05 约再慢 3%）。
- P2 的调整顺序（调研 §9.3）：**先改文案（加/删 2–3 字）→ 再逐句调 `duration_factor` → 最后才考虑 ±5% 以内的 `atempo`**；禁止全局硬拉。
- 目标校核：旁白总时长 ≤80 s（分镜 §5 预算 77 s），逐句时长 vs 分镜镜头时长差 >15% 就重生成该句。

## 9. 显存与性能（实测）

| 指标 | 数值 |
|---|---|
| 峰值显存（df=1.0，BigVGAN 回退 torch） | **6642 MiB**（约 6.5 GB） |
| 峰值显存（df=1.10，BigVGAN CUDA kernel 已编译） | **6512 MiB**（约 6.4 GB） |
| 单句推理耗时 | 3.86–4.11 s / 句（22.05 kHz 输出，约 6–10 s 音频） |
| RTF | 0.4121（回退）→ **0.3559**（CUDA kernel 后） |
| 输出格式 | 22050 Hz / mono / 16-bit PCM |
| 磁盘占用 | 权重 5.2 GB + 辅助模型 5.2 GB + venv 8.3 GB + uv 缓存 22 GB |

8 GB 卡（如本机 RTX 5060 Laptop）峰值 6.5 GB 属于"能跑但贴边"，租用 4090 是更稳的选择；本文按用户要求**不在本机部署**。

## 10. 风险与遗留

1. **音色主观项未定**（P1 唯一未闭环项）：Lead 无听觉，只能保证客观指标；用户试听后若不满意，先换 `backup-ref.wav`，再考虑升级 AISHELL-3（44.1 kHz，Apache-2.0）。
2. **播报腔**：AISHELL-1 是新闻朗读，克隆可能带一点播报感；如不可接受，GPT-SoVITS 用 `raw/aishell1-S0101-full.wav`（82.5 s）微调是唯一"用满长样本"的兜底。
3. **中英混排读法**：`AES-256` / `.dbk` / `JSON Lines` 的读法需人工确认；P2 可加拼音注音（`<行|XING2>` 式）或改中文写法。
4. **租用机成本与生命周期**：实例关机/释放会丢系统盘，但 `/root/autodl-tmp` 数据盘保留；若实例 ID 变化，按 §3.2/`setup_indextts_remote.sh` 重跑（含 5.2 GB 权重重下）。**批量合成全部做完前不要释放实例**。
5. **磁盘余量**：数据盘剩 9.5 GB，合成前先 `uv cache clean`。
6. **flash-attn/deepspeed 未装**：不影响推理；若后续要训练（GPT-SoVITS 路线）属另装环境，不在本方案内。
7. **凭据**：SSH 口令只出现在对话里，未写入仓库；仓库内只有 gitignored 的 `course/04-视频/keys/` 临时密钥（可随时删）。若担心泄露，请在完成后更换实例密码/释放实例。

## 11. 复现命令清单（本机侧）

```bash
# 1) 参考音频质检（宿主，无 ffmpeg 也可跑）
python scripts/video/prep_voice_ref.py --input course/04-视频/voice/raw/aishell1-S0101-full.wav --analyze
# 2) 语速统计（对下载回来的逐句 wav）
python scripts/video/measure_rate.py --lines scripts/video/tts-test-lines.txt \
  --wav-dir course/04-视频/voice/p1test-df110 --factor-hint
# 3) 部署到新租用机（GPU 主机）
bash scripts/video/setup_indextts_remote.sh
```

## 12. P2 交接

- **旁白文本唯一来源**：`docs/deliverables/video-storyboard.json`（7 镜）；逐句拆句后写成 `narration-lines.txt`（每句一行），用 `tts_batch_indextts.py` 逐句合成到 `course/04-视频/voice/narration/`（命名 `seg_00.wav…` 与镜号对应）。
- **时长校核**：`measure_rate.py` 输出每句时长/字速 + 总时长；目标旁白总时长 ≤80 s，逐句 vs 镜头时长差 >15% 重生成。
- **语速**：基线 `duration_factor=1.10`，逐句按需 1.05–1.20。
- **后期**：逐句 WAV 直接进 ffmpeg 混音轨（`-16 LUFS` / TP ≤ −1.5 dBFS，48 kHz/24 bit），与录制 webm 拼接；详见 `07-…` §7.2/§7.3 与 `course/04-视频/work/PROBE-infra.md`（录制链路已预检通过）。
- 阶段产物位置沿用 `course/04-视频/voice/`（gitignored），不提交音频/权重。
