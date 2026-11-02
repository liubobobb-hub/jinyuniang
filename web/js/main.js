import { createWhale } from "./pet.js";

const params = new URLSearchParams(location.search);
const obsMode = params.get("obs") === "1" || params.get("obs") === "true";
const ttsOff = params.get("tts") === "0";
const canvas = document.getElementById("stage");
const fallback = document.getElementById("fallback");
const cssWhale = document.getElementById("css-whale");

document.documentElement.dataset.booted = "1";

let pet = null;
try {
  pet = createWhale(canvas);
  const bg = document.documentElement.dataset.bg || "studio";
  pet.setBackground(bg, "#00FF00");
} catch (err) {
  console.error(err);
  showFallback();
}

const ro = new ResizeObserver(() => pet?.resize());
ro.observe(canvas);
window.addEventListener("resize", () => pet?.resize());

const statePill = document.getElementById("state-pill");
const queuePill = document.getElementById("queue-pill");
const connPill = document.getElementById("conn");
const bubble = document.getElementById("bubble");
const whoEl = document.getElementById("who");
const askEl = document.getElementById("ask");
const replyEl = document.getElementById("reply");
const brainLine = document.getElementById("brain-line");
const errorEl = document.getElementById("form-error");
const audioHint = document.getElementById("audio-hint");
const petName = document.getElementById("pet-name");
const sizeLine = document.getElementById("size-line");
const obsLink = document.getElementById("obs-link");
const bgSwitch = document.getElementById("bg-switch");

const stateLabel = { idle: "待机", thinking: "思考中", speaking: "说话中" };
let chroma = "#00FF00";
let samples = [];
let sampleIndex = 0;
let spokenKey = "";
let mockRunning = false;
let bootedState = false;
let edgeAudio = null;
const barrageEl = document.getElementById("barrage");
const liveFeedEl = document.getElementById("live-feed");
const liveHintEl = document.getElementById("live-hint");
const barrageLanes = Array(10).fill(0);
let barrageSeq = 0;

function spawnBarrage(user, text) {
  if (!barrageEl || !text) return;
  const now = performance.now();
  let lane = 0;
  let best = Infinity;
  for (let i = 0; i < barrageLanes.length; i++) {
    if (barrageLanes[i] <= now) { lane = i; best = -1; break; }
    if (barrageLanes[i] < best) { best = barrageLanes[i]; lane = i; }
  }
  const dur = 9 + Math.min(6, Math.floor(String(text).length / 12));
  barrageLanes[lane] = now + dur * 450;
  const el = document.createElement("div");
  el.className = "barrage-item";
  el.style.top = (12 + lane * 7.2) + "%";
  el.style.animationDuration = dur + "s";
  el.style.fontSize = (20 + (barrageSeq++ % 3) * 2) + "px";
  const name = document.createElement("b");
  name.textContent = (user || "观众");
  el.appendChild(name);
  el.appendChild(document.createTextNode(String(text)));
  barrageEl.appendChild(el);
  el.addEventListener("animationend", () => el.remove());
  while (barrageEl.children.length > 40) barrageEl.firstElementChild.remove();
}


function showFallback() {
  fallback.hidden = false;
  canvas.style.display = "none";
}

function applyBg(mode, persist) {
  document.documentElement.dataset.bg = mode;
  if (persist && !obsMode) localStorage.setItem("whale-bg", mode);
  bgSwitch?.querySelectorAll("button").forEach((btn) => {
    btn.classList.toggle("on", btn.dataset.bg === mode);
  });
  const key = mode === "magenta" ? "#FF00FF" : chroma;
  document.documentElement.style.setProperty("--key", key);
  pet?.setBackground(mode, key);
}

applyBg(document.documentElement.dataset.bg || "studio", false);

bgSwitch?.addEventListener("click", (event) => {
  const btn = event.target.closest("button[data-bg]");
  if (!btn) return;
  applyBg(btn.dataset.bg, true);
});

const PAY_GOAL = 500;
const PAY_REPLY = 20;
const PAY_BOSS = 50;
const payAmountEl = document.getElementById("pay-amount");
const payFillEl = document.getElementById("pay-fill");
const payToastEl = document.getElementById("pay-toast");
const payDockAmountEl = document.getElementById("pay-dock-amount");
let payDay = "";
let payYen = 0;
let payToastTimer = 0;
let lastPaidKey = "";
let liveState = "idle";

function todayKey() {
  const d = new Date();
  return d.getFullYear() + "-" + String(d.getMonth() + 1).padStart(2, "0") + "-" + String(d.getDate()).padStart(2, "0");
}

function loadPay() {
  const day = todayKey();
  try {
    const raw = JSON.parse(localStorage.getItem("whale-pay") || "null");
    if (raw && raw.day === day && Number.isFinite(raw.yen)) {
      payDay = day;
      payYen = Math.max(0, Math.floor(raw.yen));
      paintPay();
      return;
    }
  } catch (_) {}
  payDay = day;
  payYen = 0;
  savePay();
  paintPay();
}

function savePay() {
  localStorage.setItem("whale-pay", JSON.stringify({ day: payDay || todayKey(), yen: payYen }));
}

function paintPay() {
  const text = "¥" + payYen;
  if (payAmountEl) payAmountEl.textContent = text;
  if (payDockAmountEl) payDockAmountEl.textContent = text;
  if (payFillEl) {
    const pct = Math.max(0, Math.min(100, (payYen / PAY_GOAL) * 100));
    payFillEl.style.width = pct + "%";
  }
}

function flashPay(msg, kind) {
  if (!payToastEl) return;
  payToastEl.hidden = false;
  payToastEl.textContent = msg;
  payToastEl.className = "pay-toast " + (kind || "");
  clearTimeout(payToastTimer);
  payToastTimer = setTimeout(() => {
    payToastEl.hidden = true;
  }, 2600);
}

function adjustPay(delta, msg, kind) {
  payYen = Math.max(0, payYen + delta);
  savePay();
  paintPay();
  if (msg) flashPay(msg, kind);
}

function maybeEarnFromReply(ev) {
  if (!ev || ev.state !== "speaking" || !ev.reply) return;
  const key = (ev.startedAt || "") + "|" + ev.reply;
  if (key === lastPaidKey) return;
  lastPaidKey = key;
  adjustPay(PAY_REPLY, "回弹幕入账 +" + PAY_REPLY, "good");
}

function bossPatrol() {
  if (document.hidden) return;
  if (liveState === "speaking" || liveState === "thinking") {
    flashPay("主管巡场：装忙通过", "good");
    return;
  }
  if (Math.random() < 0.55) {
    adjustPay(-PAY_BOSS, "被主管抓住摸鱼 -" + PAY_BOSS, "bad");
  } else {
    flashPay("主管巡场：侥幸没被抓", "good");
  }
}

function scheduleBoss() {
  const wait = 45000 + Math.floor(Math.random() * 45000);
  setTimeout(() => {
    bossPatrol();
    scheduleBoss();
  }, wait);
}

loadPay();
scheduleBoss();

function setState(ev) {
  const state = ev.state || "idle";
  liveState = state;
  document.documentElement.dataset.state = state;
  pet?.setMood(state);
  cssWhale?.classList.toggle("is-talking", state === "speaking");
  if (statePill) {
    statePill.textContent = stateLabel[state] || state;
    statePill.className = "pill state-" + state;
  }
  if (queuePill) queuePill.textContent = "队列 " + (ev.queue ?? 0);

  if (state === "idle") {
    bubble.classList.add("is-hidden");
    stopEdgeAudio();
    if (!ttsOff && window.speechSynthesis) speechSynthesis.cancel();
    return;
  }
  bubble.classList.remove("is-hidden");
  bubble.dataset.mode = state;
  whoEl.textContent = ev.user || "观众";
  askEl.textContent = ev.danmaku || "";
  if (state === "thinking") {
    replyEl.textContent = "让我想想怎么讲更清楚…";
    return;
  }
  replyEl.textContent = ev.reply || "";
  maybeSpeak(ev);
  maybeEarnFromReply(ev);
  remember(ev);
}

function remember(ev) {
  // 回复仍显示在气泡；实时列表只记观众弹幕。
}

function pushLiveDanmaku(user, text, source) {
  if (!liveFeedEl || !text) return;
  const empty = liveFeedEl.querySelector(".live-empty");
  if (empty) empty.remove();
  const item = document.createElement("div");
  item.className = "live-item";
  const who = document.createElement("span");
  who.className = "who";
  who.textContent = user || "观众";
  const when = document.createElement("span");
  when.className = "when";
  const d = new Date();
  when.textContent = String(d.getHours()).padStart(2, "0") + ":" + String(d.getMinutes()).padStart(2, "0") + ":" + String(d.getSeconds()).padStart(2, "0");
  const txt = document.createElement("div");
  txt.className = "txt";
  txt.textContent = text;
  if (source) item.dataset.source = source;
  item.append(who, when, txt);
  liveFeedEl.appendChild(item);
  while (liveFeedEl.children.length > 80) liveFeedEl.firstElementChild.remove();
  liveFeedEl.scrollTop = liveFeedEl.scrollHeight;
  if (liveHintEl) liveHintEl.textContent = "已接收 " + liveFeedEl.querySelectorAll(".live-item").length + " 条";
}

function ensureLiveEmpty() {
  if (!liveFeedEl || liveFeedEl.children.length) return;
  const p = document.createElement("p");
  p.className = "live-empty";
  p.textContent = "还没有弹幕。等采集脚本 POST /api/danmaku，或点「开始模拟连发」。";
  liveFeedEl.appendChild(p);
}


function pickVoice() {
  if (!window.speechSynthesis) return null;
  const voices = speechSynthesis.getVoices();
  return (
    voices.find((v) => /zh[-_]CN/i.test(v.lang) && /female|xiaoxiao|yaoyao|tingting/i.test(v.name)) ||
    voices.find((v) => /zh[-_]CN/i.test(v.lang)) ||
    voices.find((v) => /^zh/i.test(v.lang)) ||
    null
  );
}

function stopEdgeAudio() {
  if (!edgeAudio) return;
  try {
    edgeAudio.pause();
    edgeAudio.removeAttribute("src");
    edgeAudio.load();
  } catch (_) {}
  edgeAudio = null;
}

function playEdgeAudio(url) {
  stopEdgeAudio();
  if (window.speechSynthesis) speechSynthesis.cancel();
  const audio = new Audio(url);
  audio.preload = "auto";
  edgeAudio = audio;
  const p = audio.play();
  if (p && typeof p.catch === "function") {
    p.catch((err) => {
      console.warn("audio play blocked", err);
      audioHint && (audioHint.hidden = false);
    });
  }
}

function maybeSpeak(ev) {
  if (ttsOff || ev.state !== "speaking" || !ev.reply) return;
  const key = (ev.startedAt || "") + "|" + (ev.audioUrl || "") + "|" + ev.reply;
  if (key === spokenKey) return;
  spokenKey = key;
  if (ev.audioUrl) {
    playEdgeAudio(ev.audioUrl);
    return;
  }
  if (!window.speechSynthesis) return;
  const utter = new SpeechSynthesisUtterance(ev.reply);
  utter.lang = "zh-CN";
  utter.rate = 1.02;
  utter.pitch = 1.22;
  const voice = pickVoice();
  if (voice) utter.voice = voice;
  speechSynthesis.cancel();
  speechSynthesis.speak(utter);
}

if (window.speechSynthesis) {
  speechSynthesis.onvoiceschanged = () => pickVoice();
}

window.addEventListener("pointerdown", () => { audioHint?.remove(); if (edgeAudio && edgeAudio.paused) edgeAudio.play().catch(() => {}); }, { once: true });

function setError(text) {
  if (!errorEl) return;
  errorEl.hidden = !text;
  errorEl.textContent = text || "";
}



document.getElementById("mock-toggle")?.addEventListener("click", async () => {
  const btn = document.getElementById("mock-toggle");
  const path = mockRunning ? "/api/mock/stop" : "/api/mock/start";
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: mockRunning ? "{}" : JSON.stringify({ intervalSec: 12 }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    setError(data.message || "模拟弹幕没能启动");
    return;
  }
  mockRunning = Boolean(data.running);
  btn.textContent = mockRunning ? "停止模拟连发" : "开始模拟连发";
});

document.getElementById("obs-open")?.addEventListener("click", () => {
  window.open("/?obs=1", "whale-obs", "width=1280,height=760,menubar=no,toolbar=no,location=no,status=no");
});

document.getElementById("copy-link")?.addEventListener("click", async () => {
  const url = obsLink?.href || location.origin + "/?obs=1";
  try {
    await navigator.clipboard.writeText(url);
    document.getElementById("copy-link").textContent = "已复制";
  } catch {
    document.getElementById("copy-link").textContent = url;
  }
});

async function loadConfig() {
  try {
    const res = await fetch("/api/config");
    const cfg = await res.json();
    chroma = cfg.chroma || chroma;
    if (cfg.character) petName.textContent = cfg.character;
    if (sizeLine && cfg.width && cfg.height) {
      sizeLine.textContent =
        "采集尺寸 " + cfg.width + "×" + cfg.height + "。带这块面板的页面只用于排练，推流用采集链接。";
    }
    if (obsLink && cfg.obsURL) obsLink.href = cfg.obsURL;
    const brain = cfg.brain === "deepseek" ? "DeepSeek · " + (cfg.model || "deepseek-chat") : "本地模拟（还没填 DEEPSEEK_API_KEY）";
    if (brainLine) brainLine.textContent = "大脑：" + brain;
    applyBg(document.documentElement.dataset.bg || "studio", false);
    const samplesRes = await fetch("/api/mock/samples");
    const payload = await samplesRes.json();
    samples = payload.items || [];
  } catch {
    if (brainLine) brainLine.textContent = "配置接口暂时没读到，桌宠仍然可以看。";
  }
}

function connect() {
  const url = (location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/ws";
  let socket;
  try {
    socket = new WebSocket(url);
  } catch {
    scheduleReconnect();
    return;
  }
  if (connPill) {
    connPill.textContent = "连接中";
    connPill.className = "pill";
  }
  socket.addEventListener("open", () => {
    if (connPill) {
      connPill.textContent = "已连接";
      connPill.className = "pill on";
    }
  });
  socket.addEventListener("message", (event) => {
    let msg;
    try {
      msg = JSON.parse(event.data);
    } catch {
      return;
    }
    if (msg.type === "hello") {
      if (msg.character) petName.textContent = msg.character;
      if (msg.chroma) chroma = msg.chroma;
      return;
    }
    if (msg.type === "danmaku") {
      spawnBarrage(msg.user, msg.text);
      pushLiveDanmaku(msg.user, msg.text, msg.source);
      return;
    }
    if (msg.type === "state") {
      if (!bootedState && msg.state === "idle" && !msg.reply) {
        bootedState = true;
      }
      setState(msg);
      bootedState = true;
    }
  });
  socket.addEventListener("close", () => {
    if (connPill) {
      connPill.textContent = "重连中";
      connPill.className = "pill off";
    }
    scheduleReconnect();
  });
}

let retry = 500;
function scheduleReconnect() {
  setTimeout(() => {
    retry = Math.min(retry * 1.6, 5000);
    connect();
  }, retry);
}

connect();
loadConfig();
