# 桌宠式 AI 小鲸鱼直播

一只漂在代码编辑器旁边的小鲸鱼。观众发弹幕，她用口语讲一句编程，嘴巴跟着动。这个窗口拿去给 OBS 或直播伴侣采集，再推到抖音或其他平台。

形象使用社区二创「鲸鱼娘 / 蓝色大肥鱼」立绘（`web/brand/dafeiyu/`）：待机 `fish_idle.png`，说话切到 `fish_happy.png`。这不是官方形象；署名与许可见 `web/brand/dafeiyu/NOTICE.md`。

## 链路

和示意图一致，六步都落在这一套本地程序里：

1. **观众发弹幕**，例如「小鲸鱼，帮我讲讲这段代码」。
2. **弹幕采集**。排练用页面按钮或 `scripts/send_danmaku.sh`。正式用你自己的脚本 `POST /api/danmaku`。本服务不抓抖音非官方接口。
3. **AI 大脑**。配置了 `DEEPSEEK_API_KEY` 就走 DeepSeek 的 OpenAI 兼容接口（默认 `deepseek-chat`）。没密钥，或接口失败，就用本地小鲸鱼口吻接话，直播不会卡住。
4. **桌宠形象**。浏览器里的小鲸鱼收到 WebSocket 状态：`idle` 待机、`thinking` 思考、`speaking` 说话。说话时显示字幕并张嘴，时长按回复字数估算。
5. **画面合成**。OBS 里把这个窗口叠在代码编辑器上面。绿幕或透明背景都准备好了。
6. **推流开播**。OBS 推流，或把 OBS 输出交给抖音直播伴侣。弹幕仍从第 2 步进来，不从推流软件里读。

弹幕一次只处理一条。后面的排在队列里，说完再接下一条。

## 环境

- Go 1.22 或更高
- Chrome、Edge 或其他新一点的浏览器
- 不需要 Node.js，也不需要 MySQL

依赖在第一次 `go run` 时下载。如果模块代理很慢，可以换国内代理后再跑：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
```

## 快速开始

在仓库根目录：

```bash
cp .env.example .env
go run ./cmd/whale-live
```

或者：

```bash
make run
```

终端会打印两个地址。浏览器打开桌宠页面，例如：

```text
http://127.0.0.1:8787/
```

页面左下角可以发弹幕。默认那句就是示意图里的「小鲸鱼，帮我讲讲这段代码」。没填密钥时，她会用本地回复把这句接住，气泡和嘴型都会动。

另开一个终端也可以推：

```bash
./scripts/send_danmaku.sh "小鲸鱼，帮我讲讲这段代码" "小鱼干"
```

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\send_danmaku.ps1 -Text "小鲸鱼，帮我讲讲这段代码" -User "小鱼干"
```

`make demo` 等价于对默认地址推那句示意图弹幕。

## 本地语音（edge-tts）

正式开播若要上云端合成（Azure / 火山 / 阿里 / Fish Audio），见 [docs/cloud-tts.md](docs/cloud-tts.md)。


默认开启。AI 回完一句后，本机用 Microsoft 神经音色合成 MP3，页面播放，鲸鱼娘跟嘴；队列按真实音频时长再接下一条。

```powershell
pip install edge-tts mutagen
```

配置在 `config.yaml` 的 `tts:`，或 `.env`：

```text
WHALE_TTS_ENABLED=true
WHALE_TTS_VOICE=zh-CN-XiaoyiNeural
```

常用女声：`zh-CN-XiaoyiNeural`（活泼）、`zh-CN-XiaoxiaoNeural`（温和）。关掉 TTS 时自动退回浏览器 `speechSynthesis`。

## 接上 DeepSeek

编辑 `.env`（不要写进 `config.yaml`，那个文件会被提交）：

```text
DEEPSEEK_API_KEY=sk-你的密钥
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-chat
```

重启 `go run`。启动日志里的「大脑」会变成 `DeepSeek / deepseek-chat`。

请求打到 `https://api.deepseek.com/chat/completions`。如果 `DEEPSEEK_BASE_URL` 以 `/v1` 结尾，则会打到 `/v1/chat/completions`。两种 DeepSeek 都接受。

人设是直播间里的编程搭子：短句、口语、适合念出来。要换说法，在 `config.yaml` 里写 `character.system_prompt`，或直接改 `internal/character/prompt.go`。超时默认 25 秒，失败就退回本地回复，页面上那条会标记为兜底（日志里 `degraded=true`）。

## 给 OBS 采集

固定画面 **1280×720**。绿幕颜色默认 **#00FF00**，可在 `config.yaml` 的 `pet.chroma` 改。

采集请用这个地址，不要用带控制面板的排练页：

```text
http://127.0.0.1:8787/?obs=1
```

可选参数：

| 参数 | 作用 |
| --- | --- |
| `obs=1` | 去掉控制面板和顶部链路，只留小鲸鱼、名字和字幕 |
| `bg=chroma` | 纯绿，给窗口采集做色度键。`obs=1` 时默认就是它 |
| `bg=magenta` | 洋红 `#FF00FF`。画面里绿色太多时换这个 |
| `bg=transparent` | 背景透明，给 OBS 浏览器源 |
| `label=0` | 隐藏底部「小鲸鱼」名牌 |
| `tts=0` | 不要浏览器朗读，只保留嘴型和字幕 |

### 方式 A：OBS 浏览器源（推荐）

1. 来源 → 浏览器。
2. URL 填 `http://127.0.0.1:8787/?obs=1&bg=transparent`。
3. 宽度 `1280`，高度 `720`。
4. 勾选「自定义 CSS 不用填」，勾选背景透明（Transparent）。
5. 需要声音时勾选「通过 OBS 控制音频」。浏览器的自动播放策略有时要先在普通窗口里点一下页面，OBS 里再刷新源。
6. 取消「不可见时关闭源」，免得一切走小鲸鱼就断线。

浏览器源能保留透明背景，鲸鱼边缘比较干净。

### 方式 B：窗口采集 + 色度键

适合直播伴侣只支持窗口、不支持浏览器源的时候。

用应用模式打开固定窗口（按你机器上的 Chrome 路径调整）：

```bash
google-chrome --app="http://127.0.0.1:8787/?obs=1" --window-size=1280,720
```

Windows 大致是：

```text
chrome.exe --app=http://127.0.0.1:8787/?obs=1 --window-size=1280,720
```

OBS 或直播伴侣里窗口采集这个窗口，加色度键：

- 关键色 `#00FF00`
- 相似度先放 300 左右，再看头发丝和气泡边，慢慢加
- 平滑度不要开太大，否则鲸鱼边缘会发虚

字幕气泡和名牌是实色，不会被绿幕抠掉。

### 和代码窗口叠在一起

1. 场景里先放代码编辑器的窗口采集或显示器采集。
2. 上面再放小鲸鱼这个源，位置偏到编辑器一侧，别挡住正在写的那几行。
3. 观众弹幕进来后，气泡出现在鲸鱼上方，说完自动收起。

第 5 步「画面合成」就是这一层，发生在 OBS 里，不发生在小鲸鱼页面里。

## 抖音直播伴侣

直播伴侣负责把合成好的画面推上去。两种常见接法：

- OBS 直接推流到抖音的 RTMP 地址（在抖音创作者后台拿推流码）。
- OBS 打开虚拟摄像头，直播伴侣把这个摄像头当成画面；或者直播伴侣窗口采集 OBS 的预览窗口。

弹幕不要指望从直播伴侣里自动流进小鲸鱼。用你自己的采集脚本调用 `POST /api/danmaku`。原因和字段说明在 [docs/douyin.md](docs/douyin.md)。

## 推送真实弹幕

```bash
curl -sS -X POST http://127.0.0.1:8787/api/danmaku \
  -H 'Content-Type: application/json; charset=utf-8' \
  -d '{"user":"小鱼干","text":"这个 nil pointer 怎么回事呀","source":"douyin"}'
```

成功时返回 `{"ok":true,"queue":1,"state":"..."}`。队列满了返回 `429`。

给局域网采集机用时：

1. `config.yaml` 的 `server.addr` 改成 `":8787"`，或设置 `WHALE_ADDR=:8787`。
2. `.env` 里设置 `WHALE_INGEST_TOKEN` 为一长串随机字符。
3. 采集请求加上头 `X-Ingest-Token`。

本机页面和 `send_danmaku.sh` 默认不用带令牌。若希望本机也校验，把 `ingest.strict` 设为 `true`（或 `WHALE_INGEST_STRICT=true`）。不要把没设令牌的端口暴露到公网。

其他接口：

| 方法 | 路径 | 作用 |
| --- | --- | --- |
| GET | `/api/health` | 存活检查 |
| GET | `/api/status` | 当前状态、队列、大脑模式 |
| GET | `/api/config` | 给页面的公开配置，不含密钥 |
| GET | `/api/history?limit=20` | 最近的弹幕和回复 |
| POST | `/api/mock/once` | 推一条示例弹幕，正文可空 |
| POST | `/api/mock/start` | 按间隔连发示例，最短 2 秒 |
| POST | `/api/mock/stop` | 停止连发 |
| GET | `/ws` | 桌宠状态流 |
| POST | `/api/danmaku/douyin/pull` | 固定 501，提醒改走 `/api/danmaku` |

WebSocket 消息：

- `hello`：角色名、尺寸、绿幕色、大脑模式
- `state`：`idle` / `thinking` / `speaking`，带观众名、弹幕、回复、`durationMs`、是否本地回复

页面在 `speaking` 时用浏览器 `speechSynthesis`（`zh-CN`）朗读。没有语音引擎时，嘴型仍按 `durationMs` 动。

## 日志和 MySQL

默认把每一轮互动追加到 `data/interactions.jsonl`，同时在内存里留最近 100 条给演播室列表。这个文件含观众昵称，已经在 `.gitignore` 里。

MySQL 默认关闭。要开：

1. 建库，并执行 [schema/mysql.sql](schema/mysql.sql)（程序连上后也会自己建表）。
2. `.env`：

```text
WHALE_MYSQL_ENABLED=true
WHALE_MYSQL_DSN=user:pass@tcp(127.0.0.1:3306)/whale_live?parseTime=true&charset=utf8mb4
```

连不上就打一条警告，继续用文件日志，不影响开播。

## 项目结构

```text
cmd/whale-live/          启动入口
internal/brain/          DeepSeek 客户端和本地回复
internal/character/      小鲸鱼人设
internal/pipeline/       单线程队列和 idle/thinking/speaking
internal/danmaku/        示例弹幕
internal/server/         HTTP 和 WebSocket
internal/store/          JSON 行日志，可选 MySQL
web/                     Three.js 桌宠页面
scripts/send_danmaku.*   推一条弹幕
docs/douyin.md           抖音弹幕怎么接
config.yaml              非机密配置
.env.example             密钥模板
```

Three.js 以 MIT 许可放在 `web/vendor/`（见该目录里的 `LICENSE-three.txt`），页面不依赖外网 CDN。

## 常见问题

**页面是空的。** 在仓库根目录启动。程序会向上查找 `web/index.html`，`go run ./cmd/whale-live` 放在根目录最稳。

**有气泡但没有声音。** 先用鼠标点一下排练页，浏览器才允许朗读。OBS 浏览器源要勾选控制音频。Linux 上如果没有中文语音包，嘴型仍然正常。

**绿幕抠不干净。** 确认采集的是 `/?obs=1`，不是带演播室面板的页面。关键色用 `#00FF00`。鲸鱼身上没有绿色。

**填了密钥却总是本地回复。** 看终端里的 `deepseek failed`。常见是密钥、余额或网络。超时后会自动用本地句子把这一条说完。

**端口被占用。** 换 `WHALE_ADDR=127.0.0.1:8788`。

## 开发

```bash
make test
make fmt
```

测试不访问真实的 DeepSeek，用本地假回复和 `httptest`。

