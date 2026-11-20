(function () {
  "use strict";

  const byId = (id) => document.getElementById(id);
  const root = document.documentElement;
  const dashboard = byId("dashboard");
  const icons = {
    chevron: "M2 10l10 5 10-5",
    wifi: "M2 8q10-9 20 0 M5 12q7-6 14 0 M8.5 16q3.5-3 7 0 M11.5 20h1",
    battery: "M2 6h17v12H2z M21 9v6",
    theme: "M12 5a7 7 0 1 0 7 9 7 7 0 0 1-7-9z",
    sun: "M16 12a4 4 0 1 0-8 0 4 4 0 1 0 8 0 M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5",
    music: "M4 14v-2a8 8 0 0 1 16 0v2 M3 13h4v7H3z M17 13h4v7h-4z",
    levels: "M3 20v-4 M7.5 20v-8 M12 20V4 M16.5 20V9 M21 20V12",
    shuffle: "M2 6h4c5 0 6 12 11 12h4m-4-4 4 4-4 4 M2 18h4c5 0 6-12 11-12h4m-4-4 4 4-4 4",
    repeat: "M3 10V8q0-3 3-3h14m-4-4 4 4-4 4 M21 14v2q0 3-3 3H4m4-4-4 4 4 4",
    volume: "M16 8q5 4 0 8 M19 5q7 7 0 14",
    muted: "M16 9l6 6m0-6-6 6",
    pulse: "M1 12h5l2-4 3 9 3-13 3 11 2-3h4",
    temperature: "M10 4a2 2 0 0 1 4 0v9a4 4 0 1 1-4 0z M12 8v8",
    slashes: "M10 6 3 18 M16 6 9 18 M22 6l-7 12",
    browser: "M22 12a10 10 0 1 0-20 0 10 10 0 1 0 20 0z M17 12a5 10 0 1 0-10 0 5 10 0 1 0 10 0z M3 8h18 M2 12h20 M3 16h18",
    terminal: "M4 6l6 6-6 6 M13 18h8"
  };
  const filledIcons = {
    play: "M7 4l13 8-13 8z",
    pause: "M6 4h4v16H6z M14 4h4v16h-4z",
    previous: "M5 5h3v14H5z M8 12l12-7v14z",
    next: "M4 5l12 7-12 7z M16 5h3v14h-3z",
    grid: "M4 4h7v7H4z M13 4h7v7h-7z M4 13h7v7H4z M13 13h7v7h-7z",
    speaker: "M2 9h4l6-5v16l-6-5H2z",
    vscode: "M3 5l2-1 7 6 7-8 4 2v16l-4 2-7-8-7 6-2-1 6-7zm12 7 4 4V8z"
  };

  function fit() {
    const scale = Math.min(window.innerWidth / 600, window.innerHeight / 800);
    dashboard.style.transform = "scale(" + scale + ")";
    // Prevent the unscaled layout box from introducing scrollbars.
    document.body.style.overflow = "hidden";
  }

  function contextFor(canvas, width, height) {
    const density = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = Math.round(width * density);
    canvas.height = Math.round(height * density);
    const context = canvas.getContext("2d");
    context.scale(canvas.width / width, canvas.height / height);
    return context;
  }

  function drawIcon(canvas) {
    const ctx = contextFor(canvas, 48, 48);
    const name = canvas.dataset.icon;
    const color = getComputedStyle(canvas).color;
    ctx.scale(2, 2);
    ctx.strokeStyle = ctx.fillStyle = color;
    ctx.lineWidth = name === "levels" || name === "slashes" ? 1.8 : 1.65;
    ctx.lineJoin = ctx.lineCap = "round";

    if (icons[name]) ctx.stroke(new Path2D(icons[name]));
    if (filledIcons[name]) ctx.fill(new Path2D(filledIcons[name]));
    if (name === "volume" || name === "muted") ctx.fill(new Path2D(filledIcons.speaker));
    if (name === "battery") ctx.fillRect(4, 8, 13, 8);
    if (name === "youtube") {
      ctx.fill(new Path2D("M5 5h14q4 0 4 4v6q0 4-4 4H5q-4 0-4-4V9q0-4 4-4z"));
      ctx.fillStyle = getComputedStyle(root).getPropertyValue("--panel").trim();
      ctx.fill(new Path2D("M10 8l7 4-7 4z"));
    }
    if (name === "captions") {
      ctx.stroke(new Path2D("M3 5h18v14H3z M10 10c-4-3-6 7 0 4 M18 10c-4-3-6 7 0 4"));
    }
    if (name === "claude") {
      ctx.translate(12, 12);
      for (let spoke = 0; spoke < 12; spoke += 1) {
        ctx.rotate(Math.PI / 6);
        ctx.stroke(new Path2D("M-2 2L10 0"));
      }
    }
    if (name === "chatgpt") {
      ctx.translate(12, 12);
      ctx.lineWidth = 1.2;
      for (let loop = 0; loop < 6; loop += 1) {
        ctx.rotate(Math.PI / 3);
        ctx.stroke(new Path2D("M0 4.2-5 1.4v-5.1c0-5.2 5.5-7.7 9.1-4.4L0-5.8v5.7"));
      }
    }
  }

  function drawAlbum() {
    const canvas = byId("album");
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "#242424";
    ctx.fillRect(0, 0, 96, 88);
    let seed = 194;
    function randomPixel(limit) {
      seed = (seed * 16807) % 2147483647;
      return seed % limit;
    }
    for (let star = 0; star < 53; star += 1) {
      ctx.fillStyle = star % 5 === 0 ? "#ededed" : "#989898";
      ctx.fillRect(randomPixel(94), randomPixel(51), star % 5 === 0 ? 2 : 1, 1);
    }
    ctx.fillStyle = "#dfdfdf";
    ctx.beginPath();
    ctx.arc(68, 23, 13, 0, Math.PI * 2);
    ctx.fill();
    ctx.save();
    ctx.clip();
    ctx.fillStyle = "#b2b2b2";
    for (let y = 10; y < 37; y += 2) {
      for (let x = 55 + (y % 4 ? 1 : 0); x < 82; x += 2) ctx.fillRect(x, y, 1, 1);
    }
    ctx.restore();
    ctx.fillStyle = "#777";
    ctx.fill(new Path2D("M0 57 14 63 28 51 41 46 57 58 71 62 86 54 96 59V88H0Z"));
    ctx.fillStyle = "#bfbfbf";
    ctx.fill(new Path2D("M0 74 16 66 23 68 48 47 67 65 74 68 83 64 96 72V88H0Z"));
    ctx.fillStyle = "#171717";
    ctx.fill(new Path2D("M0 76 16 70 22 71 43 55 49 51 44 59 56 66 65 68 79 76 96 78V88H0Z"));
    for (let dot = 0; dot < 160; dot += 1) {
      const x = (dot * 41 + Math.floor(dot / 7) * 13) % 96;
      const y = 64 + (dot * 13) % 24;
      if (y + Math.abs(x - 50) / 5 > 73) {
        ctx.fillStyle = dot % 6 === 0 ? "#bdbdbd" : "#616161";
        ctx.fillRect(x, y, 1, 1);
      }
    }
  }

  const histories = {
    cpu: [16,24,31,26,42,51,43,34,30,39,32,49,43,56,49,38,49,59,57,65,78,58,51,68],
    gpu: [11,20,17,28,38,31,46,36,33,52,44,42,49,65,57,48,59,82,70,58,76,63,57,73],
    ram: [37,35,37,34,36,33,39,40,39,43,47,48,55,52,54,49,50,51,49,44,52,50,55,54],
    vram: [25,22,25,23,24,21,23,23,23,25,31,36,33,39,49,43,47,53,52,48,54,57,49,54]
  };

  function drawPerformance() {
    const theme = getComputedStyle(root);
    const frame = theme.getPropertyValue("--frame").trim();
    const track = theme.getPropertyValue("--track").trim();
    const shade = theme.getPropertyValue("--shade").trim();
    document.querySelectorAll("[data-gauge]").forEach((canvas) => {
      const ctx = contextFor(canvas, 76, 76);
      const startAngle = -Math.PI / 2;
      ctx.lineWidth = 8;
      ctx.strokeStyle = track;
      ctx.beginPath();
      ctx.arc(38, 38, 33, 0, 2 * Math.PI);
      ctx.stroke();
      ctx.strokeStyle = frame;
      ctx.beginPath();
      ctx.arc(38, 38, 33, startAngle, startAngle + (Number(canvas.dataset.gauge) / 100) * 2 * Math.PI);
      ctx.stroke();
    });
    document.querySelectorAll("[data-spark]").forEach((canvas) => {
      const ctx = contextFor(canvas, 92, 34);
      const points = histories[canvas.dataset.spark];
      const graph = new Path2D();
      points.forEach((value, index) => graph.lineTo(index * 92 / (points.length - 1), 32 - value * .32));
      const area = new Path2D(graph);
      area.lineTo(92, 34);
      area.lineTo(0, 34);
      area.closePath();
      ctx.fillStyle = shade;
      ctx.fill(area);
      ctx.save();
      ctx.clip(area);
      ctx.fillStyle = track;
      for (let y = 3; y < 34; y += 3) {
        for (let x = (y % 2) * 2; x < 92; x += 4) ctx.fillRect(x, y, 1, 1);
      }
      ctx.restore();
      ctx.strokeStyle = frame;
      ctx.lineWidth = 1.6;
      ctx.stroke(graph);
    });
  }

  const themeButton = byId("theme");
  function applyTheme(theme) {
    const light = theme === "light";
    root.dataset.theme = theme;
    themeButton.setAttribute("aria-pressed", String(light));
    themeButton.firstElementChild.dataset.icon = light ? "sun" : "theme";
    document.querySelectorAll("canvas[data-icon]").forEach(drawIcon);
    drawPerformance();
  }

  let savedTheme;
  try { savedTheme = localStorage.getItem("kindle-dashboard-theme"); } catch (_) {}
  themeButton.addEventListener("click", () => {
    const theme = root.dataset.theme === "light" ? "dark" : "light";
    applyTheme(theme);
    try { localStorage.setItem("kindle-dashboard-theme", theme); } catch (_) {}
  });

  function updateClock() {
    const now = new Date();
    const days = ["SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"];
    const months = ["JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"];
    const time = (now.getHours() % 12 || 12) + ":" + twoDigits(now.getMinutes()) + (now.getHours() < 12 ? " AM" : " PM");
    write("clock", time);
    write("date", days[now.getDay()] + ", " + months[now.getMonth()] + " " + now.getDate());
  }

  function write(id, value) {
    const element = byId(id);
    if (element.textContent !== value) element.textContent = value;
  }

  function twoDigits(number) {
    return (number < 10 ? "0" : "") + number;
  }

  function timestamp(seconds) {
    return Math.floor(seconds / 60) + ":" + twoDigits(seconds % 60);
  }

  function segments(element, count, percent) {
    if (!element.children.length) {
      for (let index = 0; index < count; index += 1) element.appendChild(document.createElement("i"));
    }
    const filled = Math.round(count * percent / 100);
    for (let index = 0; index < count; index += 1) element.children[index].className = index < filled ? "filled" : "";
  }

  const tracks = [
    { title: "Midnight Drive", artist: "Nexora", length: 236 },
    { title: "Afterglow", artist: "Signal Coast", length: 204 },
    { title: "Static Bloom", artist: "Vesper Lane", length: 261 }
  ];
  let trackIndex = 0;
  let elapsed = 102;
  let playing = true;
  let shuffle = false;
  let repeat = false;
  let volume = 80;
  let previousVolume = 80;
  const seek = byId("seek");

  function updateProgress() {
    const length = tracks[trackIndex].length;
    seek.max = length;
    seek.value = elapsed;
    byId("seek-fill").style.width = (elapsed / length * 100) + "%";
    write("elapsed", timestamp(elapsed));
    write("duration", timestamp(length));
  }

  function updateTransport() {
    const track = tracks[trackIndex];
    write("track-title", track.title);
    write("track-artist", track.artist);
    write("play-status", playing ? "NOW PLAYING" : "PAUSED");
    const play = byId("play");
    const label = playing ? "Pause" : "Play";
    play.setAttribute("aria-label", label);
    play.title = label;
    play.firstElementChild.dataset.icon = playing ? "pause" : "play";
    drawIcon(play.firstElementChild);
    updateProgress();
  }

  function stepTrack(direction) {
    const steps = tracks.length;
    const offset = shuffle ? 1 + Math.floor(Math.random() * (steps - 1)) : direction;
    trackIndex = (trackIndex + offset + steps) % steps;
    elapsed = 0;
    updateTransport();
  }

  function updateVolume() {
    byId("volume").value = volume;
    write("volume-value", volume + "%");
    segments(byId("volume-segments"), 18, volume);
    const mute = byId("mute");
    mute.setAttribute("aria-pressed", String(volume === 0));
    mute.firstElementChild.dataset.icon = volume ? "volume" : "muted";
    drawIcon(mute.firstElementChild);
  }

  byId("play").addEventListener("click", () => { playing = !playing; updateTransport(); });
  byId("previous").addEventListener("click", () => stepTrack(-1));
  byId("next").addEventListener("click", () => stepTrack(1));
  byId("shuffle").addEventListener("click", (event) => {
    shuffle = !shuffle;
    event.currentTarget.setAttribute("aria-pressed", String(shuffle));
  });
  byId("repeat").addEventListener("click", (event) => {
    repeat = !repeat;
    event.currentTarget.setAttribute("aria-pressed", String(repeat));
  });
  seek.addEventListener("input", () => { elapsed = Number(seek.value); updateProgress(); });
  byId("volume").addEventListener("input", (event) => {
    volume = Number(event.currentTarget.value);
    if (volume) previousVolume = volume;
    updateVolume();
  });
  byId("mute").addEventListener("click", () => { volume = volume ? 0 : previousVolume; updateVolume(); });

  const overlay = byId("overlay");
  const confirmExit = byId("confirm-exit");
  const fullscreen = byId("fullscreen");
  const session = document.querySelector('meta[name="dashboard-session"]').content;
  const hasServer = /^[0-9a-f]{48}$/.test(session);
  let returnFocus;
  let closed = false;

  function showDialog(title, message, trigger, options) {
    returnFocus = trigger;
    write("dialog-title", title);
    write("dialog-message", message);
    confirmExit.hidden = !options || !options.exit;
    fullscreen.hidden = !options || !options.fullscreen || !(root.requestFullscreen || root.webkitRequestFullscreen);
    overlay.hidden = false;
    byId("dismiss").focus();
  }

  function closeDialog() {
    overlay.hidden = true;
    if (returnFocus) returnFocus.focus();
  }

  byId("info").addEventListener("click", (event) => {
    showDialog("PERSONAL DASHBOARD", "Demo mode. Music controls, Codex limits, PC readings and quick launch use sample data. Tap the moon or sun to change the display theme.", event.currentTarget, { fullscreen: true });
  });
  document.querySelectorAll("[data-launch]").forEach((button) => {
    button.addEventListener("click", () => {
      const app = button.dataset.launch;
      showDialog(app.toUpperCase(), app + " shortcut selected. This shortcut is simulated in the offline demo.", button);
    });
  });
  byId("exit").addEventListener("click", (event) => {
    showDialog("EXIT DASHBOARD?", hasServer ? "Close this session and stop the local server." : "Close this preview. Reload the page to reopen it.", event.currentTarget, { exit: true });
  });
  byId("dismiss").addEventListener("click", closeDialog);
  document.addEventListener("keydown", (event) => {
    if (overlay.hidden) return;
    if (event.key === "Escape") closeDialog();
    if (event.key === "Tab") {
      const actions = Array.from(overlay.querySelectorAll("button:not([hidden])"));
      const position = actions.indexOf(document.activeElement);
      const next = (position + (event.shiftKey ? -1 : 1) + actions.length) % actions.length;
      event.preventDefault();
      actions[next].focus();
    }
  });
  fullscreen.addEventListener("click", () => {
    const request = root.requestFullscreen || root.webkitRequestFullscreen;
    try {
      const result = request.call(root);
      if (result && result.catch) result.catch(() => write("dialog-message", "The browser could not enter full screen."));
    } catch (_) {
      write("dialog-message", "The browser could not enter full screen.");
    }
  });

  function showClosed() {
    closed = true;
    overlay.hidden = true;
    dashboard.hidden = true;
    byId("closed").hidden = false;
    write("closed-message", hasServer
      ? "Open Personal Dashboard from the Kindle Library or KUAL to start a new session."
      : "Reload the page to reopen the preview.");
  }

  confirmExit.addEventListener("click", () => {
    if (!hasServer) { showClosed(); return; }
    confirmExit.disabled = true;
    const request = new XMLHttpRequest();
    request.open("POST", "/_exit");
    request.setRequestHeader("X-Dashboard-Session", session);
    request.timeout = 4000;
    function showExitError() {
      confirmExit.disabled = false;
      write("dialog-message", "The server could not be reached. Use the Kindle stop launcher to close it.");
    }
    request.onload = () => { if (request.status === 204) showClosed(); else showExitError(); };
    request.onerror = request.ontimeout = showExitError;
    request.send();
  });

  window.addEventListener("resize", fit);
  fit();
  drawAlbum();
  applyTheme(savedTheme === "light" ? "light" : "dark");
  segments(byId("quota"), 14, 64);
  updateVolume();
  updateProgress();
  updateClock();
  // Group redraws in five-second steps to limit e-ink refreshes.
  window.setInterval(() => {
    if (closed || document.hidden) return;
    updateClock();
    if (!playing || !overlay.hidden || document.activeElement === seek) return;
    const length = tracks[trackIndex].length;
    elapsed += 5;
    if (elapsed >= length) {
      if (repeat) elapsed %= length;
      else { stepTrack(1); return; }
    }
    updateProgress();
  }, 5000);
})();
