const $ = (id) => document.getElementById(id);

let options = [];
let userEdited = false;
let lang = localStorage.getItem("microori-lang") || "en";
let lastMapping = { knob: "F8", button: "F9", knob2: "F10" };

const STRINGS = {
  en: {
    brandTitle: "MicroOri Remapper",
    brandSub: "3-Key v1.1.0 · D9 / D7 / D5 · Config saved to board",
    disconnected: "Disconnected",
    connect: "Connection",
    refresh: "Refresh",
    autoConnect: "Auto Connect",
    noPorts: "No USB serial ports",
    noPortHint: "No port found. Plug in MicroOri and refresh.",
    portsCount: (n) => `${n} USB serial port(s) available. Replug and refresh to detect.`,
    knob: "Knob",
    button: "Button",
    knob2: "Knob 2",
    hold: "Hold = key held",
    save: "Save to Board",
    reset: "Restore Default",
    disconnect: "Disconnect",
    saved: "Saved to board",
    autoConnected: "Auto connected",
    disconnectedMsg: "Disconnected",
    resetOk: "Restored defaults F8 / F9 / F10",
    connectedPort: (p) => `Connected · ${p}`,
    log: "Debug Log",
    browserGroup: "Browser",
    editingGroup: "Editing",
    lettersGroup: "Letters A-Z",
    fav: "Browser Favorites",
    search: "Browser Search",
    del: "Delete",
  },
  zh: {
    brandTitle: "MicroOri 改键上位机",
    brandSub: "三键版 v1.1.0 · D9 / D7 / D5 · 配置直存板子",
    disconnected: "未连接",
    connect: "连接",
    refresh: "刷新",
    autoConnect: "自动连接",
    noPorts: "未检测到 USB 串口",
    noPortHint: "没找到端口，插好 MicroOri 后点刷新。",
    portsCount: (n) => `${n} 个 USB 串口可用，拔插后可点刷新识别。`,
    knob: "旋钮",
    button: "按钮",
    knob2: "旋钮 2",
    hold: "按住 = 持续按键",
    save: "保存到板子",
    reset: "恢复默认",
    disconnect: "断开",
    saved: "已保存到板子",
    autoConnected: "已自动连接",
    disconnectedMsg: "已断开",
    resetOk: "已恢复默认 F8 / F9 / F10",
    connectedPort: (p) => `已连接 · ${p}`,
    log: "调试日志",
    browserGroup: "浏览器",
    editingGroup: "编辑键",
    lettersGroup: "字母 A-Z",
    fav: "浏览器收藏",
    search: "浏览器搜索",
    del: "删除",
  },
  ja: {
    brandTitle: "MicroOri キー設定",
    brandSub: "3キー版 v1.1.0 · D9 / D7 / D5 · 設定は基板に保存",
    disconnected: "未接続",
    connect: "接続",
    refresh: "更新",
    autoConnect: "自動接続",
    noPorts: "USBシリアルポートが見つかりません",
    noPortHint: "ポートが見つかりません。MicroOriを接続して更新してください。",
    portsCount: (n) => `${n} 個のUSBシリアルポート。抜き差し後に更新で認識されます。`,
    knob: "ノブ",
    button: "ボタン",
    knob2: "ノブ 2",
    hold: "押し続け = キー長押し",
    save: "基板に保存",
    reset: "初期状態に戻す",
    disconnect: "切断",
    saved: "基板に保存しました",
    autoConnected: "自動接続しました",
    disconnectedMsg: "切断しました",
    resetOk: "初期値 F8 / F9 / F10 に戻しました",
    connectedPort: (p) => `接続済み · ${p}`,
    log: "デバッグログ",
    browserGroup: "ブラウザ",
    editingGroup: "編集キー",
    lettersGroup: "A-Z の文字",
    fav: "ブラウザお気に入り",
    search: "ブラウザ検索",
    del: "Delete",
  },
};

const EN_RULES = [
  [/^已打开 (.+?)，握手确认中\.\.\.$/, (m) => `Opened ${m[1]}, confirming handshake...`],
  [/^握手成功，读取当前映射$/, () => "Handshake OK, reading current mapping"],
  [/^连接断开: (.+)$/, (m) => `Disconnected: ${m[1]}`],
  [/^映射已保存到板子: 旋钮=(.+?) 按钮=(.+?)$/, (m) => `Saved to board: knob=${m[1]} button=${m[2]}`],
  [/^映射已保存到板子: 旋钮1=(.+?) 按钮=(.+?) 旋钮2=(.+?)$/, (m) => `Saved to board: knob 1=${m[1]} button=${m[2]} knob 2=${m[3]}`],
  [/^自动连接 (.+?) 失败: (.+)$/, (m) => `Auto-connect ${m[1]} failed: ${m[2]}`],
  [/^端口读取失败: (.+)$/, (m) => `Port read failed: ${m[1]}`],
  [/^打开 (.+?) 失败: (.+)$/, (m) => `Failed to open ${m[1]}: ${m[2]}`],
  [/^未连接$/, () => "Not connected"],
  [/^无效按键: (.+)$/, (m) => `Invalid key: ${m[1]}`],
];

const JA_RULES = [
  [/^已打开 (.+?)，握手确认中\.\.\.$/, (m) => `${m[1]} を開きました。ハンドシェイク確認中...`],
  [/^握手成功，读取当前映射$/, () => "ハンドシェイク成功、現在のマッピングを読み取り中"],
  [/^连接断开: (.+)$/, (m) => `切断: ${m[1]}`],
  [/^映射已保存到板子: 旋钮=(.+?) 按钮=(.+?)$/, (m) => `基板に保存: ノブ=${m[1]} ボタン=${m[2]}`],
  [/^映射已保存到板子: 旋钮1=(.+?) 按钮=(.+?) 旋钮2=(.+?)$/, (m) => `基板に保存: ノブ1=${m[1]} ボタン=${m[2]} ノブ2=${m[3]}`],
  [/^自动连接 (.+?) 失败: (.+)$/, (m) => `自動接続 ${m[1]} 失敗: ${m[2]}`],
  [/^端口读取失败: (.+)$/, (m) => `ポート読み取り失敗: ${m[1]}`],
  [/^打开 (.+?) 失败: (.+)$/, (m) => `${m[1]} を開けませんでした: ${m[2]}`],
  [/^未连接$/, () => "未接続"],
  [/^无效按键: (.+)$/, (m) => `無効なキー: ${m[1]}`],
];

function text(key) {
  const v = STRINGS[lang] && STRINGS[lang][key];
  return v === undefined ? key : v;
}

function translateMsg(msg) {
  if (lang === "zh") return msg;
  const rules = lang === "en" ? EN_RULES : JA_RULES;
  for (const [re, repl] of rules) {
    const m = msg.match(re);
    if (!m) continue;
    return typeof repl === "function" ? repl(m) : repl;
  }
  return msg;
}

function translateLog(line) {
  if (/^<<|^>>/.test(line)) return line;
  const i = line.indexOf("  ");
  if (i < 0) return translateMsg(line);
  return line.slice(0, i) + "  " + translateMsg(line.slice(i + 2));
}

function groupLabel(g) {
  if (g === "浏览器") return text("browserGroup");
  if (g === "字母 A-Z") return text("lettersGroup");
  if (g === "Browser" || g === "浏览器") return text("browserGroup");
  if (g === "Editing" || g === "编辑键" || g === "編集キー") return text("editingGroup");
  if (g === "Letters A-Z" || g === "A-Z の文字") return text("lettersGroup");
  return g;
}

function optLabel(o) {
  if (o.value === "FAV") return text("fav");
  if (o.value === "SEARCH") return text("search");
  if (o.value === "DEL") return text("del");
  return o.value;
}

async function api(path, opts) {
  const res = await fetch(path, opts);
  return res.json();
}

function fillOptions(select, current) {
  const groups = {};
  for (const o of options) {
    if (!groups[o.group]) groups[o.group] = document.createElement("optgroup");
    groups[o.group].label = groupLabel(o.group);
    const opt = document.createElement("option");
    opt.value = o.value;
    opt.textContent = optLabel(o) + "  ( " + o.value + " )";
    if (o.value === current) opt.selected = true;
    groups[o.group].appendChild(opt);
  }
  select.innerHTML = "";
  Object.values(groups).forEach((g) => select.appendChild(g));
  if (!options.some((o) => o.value === current)) {
    const opt = document.createElement("option");
    opt.value = current;
    opt.textContent = current;
    opt.selected = true;
    select.appendChild(opt);
  }
}

function refreshSelects() {
  fillOptions($("knobSelect"), $("knobSelect").value || lastMapping.knob);
  fillOptions($("buttonSelect"), $("buttonSelect").value || lastMapping.button);
  fillOptions($("knob2Select"), $("knob2Select").value || lastMapping.knob2);
}

function applyLang() {
  document.documentElement.lang = lang;
  document.title = text("brandTitle");
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    el.textContent = text(el.dataset.i18n);
  });
  document.querySelectorAll(".lang-btn").forEach((b) => {
    b.classList.toggle("active", b.dataset.lang === lang);
  });
  refreshSelects();
  pollStatus();
}

function setLang(next) {
  lang = next;
  localStorage.setItem("microori-lang", lang);
  applyLang();
}

function renderLog(lines) {
  const box = $("log");
  box.innerHTML = lines.map((line) => {
    let cls = "info";
    if (line.includes("<< ")) cls = "recv";
    else if (line.includes(">> ")) cls = "send";
    return `<div class="${cls}">${escapeHtml(translateLog(line))}</div>`;
  }).join("");
  box.scrollTop = box.scrollHeight;
}

function escapeHtml(s) {
  const div = document.createElement("div");
  div.textContent = s;
  return div.innerHTML;
}

function setPill(connected, port) {
  const pill = $("statusPill");
  if (connected) {
    pill.textContent = text("connectedPort")(port);
    pill.className = "pill pill-on";
  } else {
    pill.textContent = text("disconnected");
    pill.className = "pill pill-off";
  }
}

async function refreshPorts() {
  const data = await api("/api/ports");
  const sel = $("portSelect");
  sel.innerHTML = "";
  if (!data.ports || data.ports.length === 0) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = text("noPorts");
    sel.appendChild(opt);
    $("portInfo").textContent = text("noPortHint");
    return;
  }
  data.ports.forEach((p) => {
    const opt = document.createElement("option");
    opt.value = p.name;
    opt.textContent = `${p.name}  ${p.product || "USB"} ${p.vid ? "(" + p.vid + ":" + p.pid + ")" : ""}`;
    sel.appendChild(opt);
  });
  $("portInfo").textContent = text("portsCount")(data.ports.length);
}

async function pollStatus() {
  let data;
  try {
    data = await api("/api/status");
  } catch (e) {
    return;
  }
  setPill(data.connected, data.port || "");
  renderLog(data.log || []);
  if (data.mapping) {
    lastMapping = data.mapping;
    if (data.connected && !userEdited) {
      fillOptions($("knobSelect"), data.mapping.knob);
      fillOptions($("buttonSelect"), data.mapping.button);
      fillOptions($("knob2Select"), data.mapping.knob2);
    }
  }
  if (!data.connected) {
    await refreshPorts();
  }
}

function showSave(msg, ok) {
  const el = $("saveMsg");
  el.textContent = msg;
  el.className = ok ? "muted ok" : "muted err";
  setTimeout(() => { el.textContent = ""; }, 3500);
}

async function init() {
  const opts = await api("/api/options");
  options = opts.options || [];

  document.querySelectorAll(".lang-btn").forEach((b) => {
    b.addEventListener("click", () => setLang(b.dataset.lang));
  });

  applyLang();

  $("refreshBtn").addEventListener("click", refreshPorts);
  $("autoBtn").addEventListener("click", async () => {
    const r = await api("/api/connect-auto", { method: "POST" });
    showSave(r.ok ? text("autoConnected") : translateMsg(r.error || ""), r.ok);
    await pollStatus();
  });
  $("knobSelect").addEventListener("change", () => { userEdited = true; });
  $("buttonSelect").addEventListener("change", () => { userEdited = true; });
  $("knob2Select").addEventListener("change", () => { userEdited = true; });
  $("portSelect").addEventListener("change", async (e) => {
    if (!e.target.value) return;
    const r = await api("/api/connect", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ port: e.target.value }),
    });
    showSave(r.ok ? text("connectedPort")(e.target.value) : translateMsg(r.error || ""), r.ok);
    await pollStatus();
  });
  $("saveBtn").addEventListener("click", async () => {
    const r = await api("/api/mapping", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        knob: $("knobSelect").value,
        button: $("buttonSelect").value,
        knob2: $("knob2Select").value,
      }),
    });
    showSave(r.ok ? text("saved") : translateMsg(r.error || ""), r.ok);
    if (r.ok) userEdited = false;
    await pollStatus();
  });
  $("resetBtn").addEventListener("click", async () => {
    const r = await api("/api/reset", { method: "POST" });
    showSave(r.ok ? text("resetOk") : translateMsg(r.error || ""), r.ok);
    await pollStatus();
  });
  $("disconnectBtn").addEventListener("click", async () => {
    await api("/api/disconnect", { method: "POST" });
    showSave(text("disconnectedMsg"), true);
    await pollStatus();
  });

  await pollStatus();
  setInterval(pollStatus, 2500);
}

init();
