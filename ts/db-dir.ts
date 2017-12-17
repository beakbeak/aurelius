function getTrackUrls(): string[] {
    const tracks = document.getElementById("tracks");
    if (tracks === null) {
        return [];
    }
    const trackUrls: string[] = [];
    for (const trackLink of tracks.getElementsByTagName("a")) {
        trackUrls.push(trackLink.href);
    }
    return trackUrls;
}

window.onload = () => {
    const player = new Player("header");

    // XXX hack alert
    if (/([0-9]+\.){3}[0-9]+/.test(window.location.hostname)) {
        player.setStreamOptions({ codec: "wav" });
    }

    const playlists = document.getElementById("playlists");
    if (playlists !== null) {
        const playlistLinks: HTMLAnchorElement[] =
            Array.prototype.slice.apply(playlists.getElementsByTagName("a"));

        for (const playlistLink of playlistLinks) {
            const randomLink = document.createElement("a");
            randomLink.textContent = "random";
            randomLink.href = playlistLink.href;
            randomLink.classList.add("aux-link");
            playlistLink.parentElement!.appendChild(randomLink);

            playlistLink.onclick = () => {
                player.playList(playlistLink.href);
                return false;
            };
            randomLink.onclick = () => {
                player.playList(randomLink.href, true);
                return false;
            };
        }
    }

    const tracks = document.getElementById("tracks");
    if (tracks !== null) {
        for (const trackLink of tracks.getElementsByTagName("a")) {
            trackLink.onclick = () => {
                const trackUrls = getTrackUrls();
                const startPos = trackUrls.indexOf(trackLink.href);
                player.playList(trackUrls, false, startPos >= 0 ? startPos : 0)
                return false;
            };
        }
    }
};