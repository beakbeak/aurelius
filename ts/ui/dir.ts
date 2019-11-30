import { Player } from "../core/player.js";
import { DirInfo, fetchDirInfo } from "../core/dir.js";

let player: Player;

let dirList: HTMLElement;
let playlistList: HTMLElement;
let trackList: HTMLElement;

export default async function setupDirUi(
    inPlayer: Player,
    containerId: string,
    url?: string,
) {
    const container = document.getElementById(containerId);
    if (container === null) {
        throw new Error("invalid container");
    }

    player = inPlayer;

    const createList = () => {
        const out = document.createElement("ul");
        out.className = "listing";
        out.style.display = "none";
        return out;
    };

    dirList = createList();
    playlistList = createList();
    trackList = createList();

    container.innerHTML = "";
    container.appendChild(dirList);
    container.appendChild(playlistList);
    container.appendChild(trackList);

    window.onpopstate = () => {
        loadUrl();
    };

    await loadUrl();
}

async function loadUrl(url?: string): Promise<void> {
    if (url === undefined) { // first call
        window.history.replaceState({}, "");
    } else {
        window.history.pushState({}, "", url);
    }

    const info = await fetchDirInfo(url ?? window.location.href);

    populateDirs(info);
    populatePlaylists(info);
    populateTracks(info);
}

function populateDirs(info: DirInfo): void {
    let html =
        `<li>
            <i class="material-icons">arrow_back</i>
            <a href="..">Parent directory</a>
        </li>`;
    if (info.dirs) {
        for (const dir of info.dirs) {
            html +=
                `<li>
                    <i class="material-icons">folder_open</i>
                    <a href="${dir.url}/">${dir.name}/</a>
                </li>`;
        }
    }
    dirList.innerHTML = html;

    const links = dirList.getElementsByTagName("a");
    for (let i = 0; i < links.length; ++i) {
        const link = links[i];

        link.onclick = (e) => {
            e.preventDefault();
            loadUrl(link.href); // ignore Promise
        };
    }

    dirList.style.display = "";
}

function populatePlaylists(info: DirInfo): void {
    playlistList.style.display = "none";

    if (!info.playlists) {
        playlistList.innerHTML = "";
        return;
    }

    let html = ``;
    for (const playlist of info.playlists) {
        html +=
            `<li>
                <i class="material-icons">playlist_play</i>
                <a href="${playlist.url}">${playlist.name}</a>
                <a href="${playlist.url}" class="aux-link">random</a>
            </li>`;
    }
    playlistList.innerHTML = html;

    const links = playlistList.getElementsByTagName("a");
    for (let i = 0; i < links.length; i += 2) {
        const link = links[i];
        const randomLink = links[i + 1];

        link.onclick = (e) => {
            e.preventDefault();
            player.playList(link.href);
        };
        randomLink.onclick = (e) => {
            e.preventDefault();
            player.playList(link.href, true);
        };
    }

    playlistList.style.display = "";
}

function populateTracks(info: DirInfo): void {
    trackList.style.display = "none";

    if (!info.tracks) {
        trackList.innerHTML = "";
        return;
    }

    let html = ``;
    for (const track of info.tracks) {
        html +=
            `<li>
                <i class="material-icons">music_note</i>
                <a href="${track.url}">${track.name}</a>
            </li>`;
    }
    trackList.innerHTML = html;

    const trackUrls: string[] = [];
    for (const file of info.tracks) {
        trackUrls.push(file.url);
    }

    const links = trackList.getElementsByTagName("a");
    for (let i = 0; i < links.length; ++i) {
        const link = links[i];

        link.onclick = (e) => {
            e.preventDefault();
            player.playList(trackUrls, false, i);
        };
    }

    trackList.style.display = "";
}