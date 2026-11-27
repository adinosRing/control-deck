importScripts("pairing.js");

const youtubeSites = ["https://www.youtube.com/*", "https://m.youtube.com/*", "https://music.youtube.com/*"];
let connection = null;

async function playerCommand(action, value) {
  const tabs = await chrome.tabs.query({url: youtubeSites});
  const players = await Promise.all(tabs.map(async (tab) => {
    try {
      const reply = await chrome.tabs.sendMessage(tab.id, {action: "status"});
      return reply?.video ? {tab, video: reply.video} : null;
    } catch {
      return null;
    }
  }));
  const available = players.filter(Boolean).sort((a, b) => {
    const rank = ({tab, video}) => 2 * Number(video.playing) + Number(tab.active);
    return rank(b) - rank(a) || (b.tab.lastAccessed ?? 0) - (a.tab.lastAccessed ?? 0);
  });
  const selected = available[0];
  if (action === "status") return {video: selected?.video ?? null};
  if (!selected) return {error: "Open a YouTube video on the PC and press Play once."};
  return chrome.tabs.sendMessage(selected.tab.id, {action, value});
}

function connect() {
  if (connection && connection.readyState < WebSocket.CLOSING) return;
  const socket = new WebSocket("ws://127.0.0.1:8766/");
  connection = socket;
  let heartbeat;
  socket.onopen = () => {
    socket.send(JSON.stringify({token: browserPairingToken}));
    heartbeat = setInterval(() => {
      if (socket.readyState === WebSocket.OPEN) socket.send("{}");
    }, 18_000);
  };
  socket.onmessage = async ({data}) => {
    let request;
    try {
      request = JSON.parse(data);
      const result = await playerCommand(request.action, request.value);
      if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({id: request.id, ...result}));
    } catch {
      if (request && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({id: request.id, error: "Reload the YouTube video tab and try again."}));
      }
    }
  };
  socket.onclose = () => {
    clearInterval(heartbeat);
    if (connection === socket) connection = null;
  };
}

chrome.runtime.onStartup.addListener(connect);
chrome.runtime.onInstalled.addListener(connect);
chrome.runtime.onMessage.addListener((message, sender) => {
  if (message?.ready && sender.tab) connect();
});
chrome.alarms.onAlarm.addListener(({name}) => {
  if (name === "pc-reconnect") connect();
});
chrome.alarms.create("pc-reconnect", {periodInMinutes: 1});
connect();
