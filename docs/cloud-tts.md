# 云端 TTS 方案说明

当前直播桌宠用本机 **edge-tts**（`python -m edge_tts`）合成语音，再把 `AudioURL` + 时长交给前端对口型。本地省事，但依赖 Python、音色与稳定性受本机网络/微软接口影响，不适合多机部署和正式开播。

本文只写**规划与选型**，不改代码。落地时保持现有管道不变：`Speak(text) → 落盘 mp3 → /media/… → duration 嘴型`。

## 目标

| 项 | 要求 |
| --- | --- |
| 接口 | 与 `internal/tts` 的 `Speak` 一致，返回可播放 URL 与时长 |
| 音色 | 中文女声，偏活泼「鲸鱼娘」；可配置 voice / speaker |
| 延迟 | 单句尽量 &lt; 1.5s（短句），失败可退回 edge 或浏览器朗读 |
| 密钥 | 只放 `.env`，不进仓库 |
| 部署 | 无本机 Python 也能跑（只依赖 `whale-live.exe` + 云端密钥） |

建议在 `config.yaml` 扩展：

```yaml
tts:
  provider: edge   # edge | azure | volc | aliyun | fish
  enabled: true
  voice: zh-CN-XiaoyiNeural
  dir: data/audio
  # 云端共用
  # api_key / region / app_id 等从环境变量读取
```

## 候选服务对比

### 1. Azure Speech（Cognitive Services）

- **定位**：企业级，神经音色全，文档稳。
- **能力**：REST / SDK 合成；中文有 `zh-CN-XiaoyiNeural` 等，与现 edge 音色名接近，迁移成本低。
- **鉴权**：`AZURE_SPEECH_KEY` + `AZURE_SPEECH_REGION`。
- **计费**：按字符；有免费额度，量上来按量付。
- **优点**：音色可控、SSML、稳定性好。
- **缺点**：国内访问可能要代理/专线；开通与账单偏「微软云」流程。
- **接入要点**：HTTP TTS → 写 `data/audio/{id}.mp3` → 用 mutagen/ffmpeg 或响应头估时长。

### 2. 火山引擎（字节）语音合成

- **定位**：国内直播/短视频常用，延迟与中文表现好。
- **能力**：HTTP/SDK；多音色、可调语速情感；部分支持流式。
- **鉴权**：`VOLC_ACCESS_KEY` / `VOLC_SECRET_KEY`（或 AppId + Token，以控制台为准）。
- **计费**：按字符或并发包，国内价通常友好。
- **优点**：国内网络稳、开播场景多。
- **缺点**：控制台与 API 版本要盯文档；音色 ID 与 Azure 不通用。
- **接入要点**：优先非流式整句 mp3/wav，再复用现有 `/media` 静态服务。

### 3. 阿里云智能语音（NLS / 语音合成）

- **定位**：国内云标配，和现有阿里账号体系好绑。
- **能力**：一句话合成、长文本；多中文音色。
- **鉴权**：`ALIYUN_AK` / `ALIYUN_SK` 或 Token（NLS Token 有时效）。
- **计费**：按字符，常有免费试用。
- **优点**：国内可达性好、文档中文全。
- **缺点**：Token 刷新要写进客户端；部分产品线要绑项目 AppKey。
- **接入要点**：拿到音频字节后落盘；Token 用后台定时刷新，别写死在 yaml。

### 4. Fish Audio

- **定位**：偏「声音克隆 / 角色音色」，适合固定人设桌宠。
- **能力**：API 合成；可上传参考音做克隆（注意版权与授权）。
- **鉴权**：`FISH_API_KEY`。
- **计费**：按用量订阅/积分，相对新，价格变动快。
- **优点**：角色声线统一，比通用神经女声更「像一个固定主播」。
- **缺点**：合规与素材授权要自己把关；SLA/国内线路需实测。
- **接入要点**：选定 reference / model id 写进配置；仍输出 mp3 给前端。

## 推荐落地顺序

1. **先抽象接口**  
   `type Speaker interface { Speak(ctx, id, text) (url string, dur time.Duration, err error) }`  
   `edge` 实现保留；云端各自一个文件（`azure.go` / `volc.go` / …）。

2. **优先接一家国内云做主路径**  
   - 人在国内、要稳：火山或阿里。  
   - 已有 Azure、要同款 Neural 音色名：Azure。  
   - 要固定「鲸鱼娘」声线：Fish Audio（有合法参考音再上）。

3. **edge 作 fallback**  
   `provider=volc` 失败时可选降级 `edge`，再不行前端 `speechSynthesis`。

4. **观测**  
   打日志：provider、耗时、字符数、是否降级；方便对账与排查口型不同步。

## 环境变量草案

```bash
# 通用
WHALE_TTS_PROVIDER=edge          # edge | azure | volc | aliyun | fish
WHALE_TTS_ENABLED=true
WHALE_TTS_VOICE=zh-CN-XiaoyiNeural

# Azure
AZURE_SPEECH_KEY=
AZURE_SPEECH_REGION=eastasia

# 火山
VOLC_TTS_APP_ID=
VOLC_TTS_ACCESS_TOKEN=
# 或 AK/SK，以控制台文档为准

# 阿里
ALIYUN_NLS_APP_KEY=
ALIYUN_ACCESS_KEY_ID=
ALIYUN_ACCESS_KEY_SECRET=

# Fish Audio
FISH_API_KEY=
FISH_REFERENCE_ID=
```

具体字段名以各云最新控制台为准，实现时再对齐 `.env.example`。

## 与现有 edge 的关系

| | 本地 edge-tts | 云端 TTS |
| --- | --- | --- |
| 依赖 | Python + edge-tts + mutagen | 仅 HTTP + 密钥 |
| 音色 | 微软 Neural（免费调用侧） | 各云自有价目与音色 ID |
| 适合 | 开发、单机试播 | 正式开播、无 Python 机器、多实例 |
| 口型 | 已有 duration | 同样要测真实时长，勿只用字数估算 |

开发机可继续 `provider: edge`；发布机改为云端 provider，同一套前端不用改。

## 下一步（实现时）

- [ ] 抽出 `Speaker` 接口，edge 迁到该接口下  
- [ ] 实现第一家云端（建议火山或阿里）+ 配置/env  
- [ ] 失败降级与超时（建议 3–5s）  
- [ ] 更新 `.env.example` / README「本地语音」一节链到本文  
- [ ] 用一句固定文案压测延迟与口型

有选定的厂商和账号后，再改 `internal/tts` 落地即可。
