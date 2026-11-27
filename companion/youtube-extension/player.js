(() => {
  const player = () => document.querySelector("video.html5-main-video") || document.querySelector("video");
  const captions = () => document.querySelector(".ytp-subtitles-button");
  const music = location.hostname === "music.youtube.com";
  // YouTube Music keeps playing while you browse, so any page with a loaded player counts.
  const playingPage = () => music || /^\/(watch|shorts\/|live\/)/.test(location.pathname);
  // The largest cover the page offers, from Media Session, the player bar, or the video's thumbnail.
  const cover = () => {
    const artwork = Array.from(navigator.mediaSession?.metadata?.artwork || []);
    const size = (item) => Math.max(0, ...String(item.sizes || "").split(/\s+/).map((s) => parseInt(s, 10) || 0));
    artwork.sort((a, b) => size(b) - size(a));
    let src = artwork[0]?.src || (music && document.querySelector("ytmusic-player-bar img.image")?.src) || "";
    const id = new URLSearchParams(location.search).get("v");
    if (!src && id && /^[\w-]{11}$/.test(id)) src = `https://i.ytimg.com/vi/${id}/hqdefault.jpg`;
    // Ask YouTube Music's image server for a 400px square cover.
    if (/googleusercontent\.com\//.test(src)) src = src.replace(/=w\d+-h\d+[^/]*$/, "=w400-h400-l90-rj");
    return /^https:\/\//.test(src) ? src.slice(0, 600) : "";
  };
  const visibleControl = (selector) => {
    const control = document.querySelector(selector);
    return control && !control.disabled && control.getAttribute("aria-disabled") !== "true" && control.getClientRects().length ? control : null;
  };
  const info = (video) => {
    const title = document.querySelector("ytd-watch-metadata h1")?.textContent?.trim()
      || navigator.mediaSession?.metadata?.title || document.querySelector("ytmusic-player-bar .title")?.textContent?.trim() || document.title.replace(/\s*-\s*YouTube$/, "");
    const artist = document.querySelector("ytd-watch-metadata #channel-name a")?.textContent?.trim()
      || navigator.mediaSession?.metadata?.artist || "YouTube";
    const duration = Number.isFinite(video.duration) ? Math.floor(video.duration) : 0;
    return {
      title: title.slice(0, 160),
      artist: artist.slice(0, 160),
      playing: !video.paused && !video.ended,
      duration,
      position: duration ? Math.min(duration, Math.floor(video.currentTime)) : 0,
      volume: Math.round(video.volume * 100),
      muted: video.muted,
      captions: captions()?.getAttribute("aria-pressed") === "true",
      repeat: video.loop,
      art: cover(),
      source: music ? "music" : "youtube"
    };
  };

  async function control(action, value) {
    const video = playingPage() && player();
    if (!video || !video.currentSrc) return {video: null, ...(action === "status" ? {} : {error: "Play something on YouTube or YouTube Music on the PC first."})};
    if (action === "status") return {video: info(video)};
    if (document.querySelector("#movie_player.ad-showing")) return {error: "Wait for the YouTube ad to finish."};

    switch (action) {
      case "toggle":
        if (video.paused) {
          try { await video.play(); }
          catch { return {error: "Press Play once in the PC browser to enable remote playback."}; }
        } else video.pause();
        break;
      case "seek":
        if (!Number.isFinite(video.duration) || video.duration <= 0) return {error: "Seeking is unavailable for this video."};
        video.currentTime = video.duration * value / 100;
        break;
      case "volume":
        video.volume = value / 100;
        video.muted = false;
        break;
      case "mute":
        video.muted = !video.muted;
        break;
      case "previous": {
        const previous = visibleControl(".ytp-prev-button") || visibleControl("ytmusic-player-bar .previous-button");
        if (previous) previous.click();
        else if (Number.isFinite(video.duration)) video.currentTime = 0;
        else return {error: "Previous is unavailable for this video."};
        break;
      }
      case "next": {
        const next = visibleControl(".ytp-next-button") || visibleControl("ytmusic-player-bar .next-button");
        if (!next) return {error: "YouTube has no next video ready in this tab."};
        next.click();
        break;
      }
      case "captions": {
        const button = visibleControl(".ytp-subtitles-button");
        if (!button) return {error: "Captions are unavailable for this video."};
        button.click();
        break;
      }
      case "repeat":
        video.loop = !video.loop;
        break;
      default:
        return {error: "Unknown YouTube control."};
    }
    return {video: info(video)};
  }

  chrome.runtime.onMessage.addListener(({action, value}, _sender, respond) => {
    control(action, value).then(respond, () => respond({error: "YouTube could not complete that action."}));
    return true;
  });
  chrome.runtime.sendMessage({ready: true}).catch(() => {});
})();
