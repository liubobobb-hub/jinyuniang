# 抖音弹幕怎么接进来

小鲸鱼的开播链路是：

1. 观众在直播间发弹幕
2. 你自己的采集程序拿到这条弹幕
3. 采集程序 `POST /api/danmaku`
4. 小鲸鱼思考、回答，桌宠页面播出
5. OBS 或直播伴侣采集桌宠窗口，叠在代码画面上
6. 推流到抖音或其他平台

本服务**不内置**抖音弹幕抓取。非官方接口经常改、也容易违反平台规则，所以快乐路径不依赖它们。`POST /api/danmaku/douyin/pull` 会固定返回 `501`，用来标明这个位置故意空着。

## 推荐做法

画面和弹幕分开处理。

- **画面**：抖音直播伴侣采集 OBS 的输出，或者直播伴侣自己做窗口采集。桌宠用 OBS 浏览器源 / 窗口采集，见 README 的 OBS 一节。
- **弹幕**：用你已经在用、而且你有权使用的来源，把文本推给本机。

来源可以是：

- 直播中控台旁边你自己写的转发脚本
- 你自己的弹幕网关
- 本仓库的模拟弹幕（排练）

推送格式：

```bash
curl -sS -X POST http://127.0.0.1:8787/api/danmaku \
  -H 'Content-Type: application/json' \
  -d '{"user":"小鱼干","text":"小鲸鱼，帮我讲讲这段代码","source":"douyin"}'
```

如果服务绑到了局域网，并且设置了 `WHALE_INGEST_TOKEN`，请求要加：

```text
X-Ingest-Token: 你的令牌
```

或 `Authorization: Bearer 你的令牌`。本机（127.0.0.1）默认不校验，除非 `ingest.strict: true`。

字段：

| 字段 | 说明 |
| --- | --- |
| `user` | 观众昵称，最多 32 字，空则记为「观众」 |
| `text` | 弹幕正文，最多 400 字 |
| `source` | 来源标记，例如 `douyin`、`bilibili`、`script`，只写进日志 |

返回 `200` 表示已进队列。队列满时返回 `429`，采集端过一两秒再试即可。同一时间只回答一条，后面的在队列里等。

## 一个最小转发函数

不包含任何平台登录或页面抓取，只负责把你已经拿到的文本送进来。Node 18+：

```js
async function pushDanmaku(user, text) {
  const res = await fetch("http://127.0.0.1:8787/api/danmaku", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ user, text, source: "douyin" }),
  });
  if (!res.ok) {
    throw new Error(await res.text());
  }
}
```

采集程序跑在别的机器上时，把地址换成小鲸鱼那台电脑的局域网 IP，并带上令牌。不要把没有令牌的端口暴露到公网。
