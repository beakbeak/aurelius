import { Player } from "../core/player";
import { DirInfo, fetchDirInfo } from "../core/dir";
import { ReplayGainMode } from "../core/track";
import { Class } from "./class";
import { closestAncestorWithClass } from "./dom";

let player: Player;

let specialList: HTMLElement;
let dirList: HTMLElement;
let playlistList: HTMLElement;
let trackList: HTMLElement;

let lastPlaying: HTMLElement | undefined;

export default async function setupDirUi(inPlayer: Player) {
    const container = document.querySelector(`.${Class.MainDir}`);
    if (container === null) {
        throw new Error("invalid container");
    }

    player = inPlayer;

    const createList = () => {
        const out = document.createElement("ul");
        out.classList.add(Class.DirListing, Class.Hidden);
        return out;
    };

    specialList = createList();
    dirList = createList();
    playlistList = createList();
    trackList = createList();

    container.innerHTML = "";
    container.appendChild(specialList);
    container.appendChild(dirList);
    container.appendChild(playlistList);
    container.appendChild(trackList);

    populateSpecial();

    player.addEventListener("play", () => {
        highlightPlayingTrack();
    });
    player.addEventListener("ended", () => {
        unhighlightPlayingTrack();
    })

    window.onpopstate = () => {
        loadDir();
    };

    await loadDir();
}

function populateSpecial(): void {
    specialList.innerHTML =
        `<li class="${Class.DirEntry}">
            <i class="${Class.DirIcon} ${Class.MaterialIcons}">favorite_border</i>
            <a class="${Class.DirLink}" href="#">Favorites</a>
        </li>`

    const favoritesLink = specialList.querySelector("a")!;

    favoritesLink.onclick = (e) => {
        e.preventDefault();
        player.playList("/media/favorites", { random: true });
    };

    specialList.classList.remove(Class.Hidden);
}

function highlightPlayingTrack(): void {
    let playingLink = undefined;

    const trackLinks = trackList.querySelectorAll("a");
    for (let i = 0; i < trackLinks.length; ++i) {
        const link = trackLinks[i] as HTMLAnchorElement;
        if (isPlaying(link)) {
            playingLink = link;
            break;
        }
    }

    setPlayingClass(playingLink);
}

function unhighlightPlayingTrack(): void {
    setPlayingClass(undefined);
}

function isPlaying(element: HTMLAnchorElement): boolean {
    if (player.track === undefined) {
        return false;
    }
    return player.track.url.endsWith(element.pathname);
}

function setPlayingClass(element: HTMLAnchorElement | undefined): void {
    lastPlaying?.classList.remove(Class.DirEntry_Playing);
    lastPlaying = element !== undefined ? closestAncestorWithClass(element, Class.DirEntry) : undefined;
    lastPlaying?.classList.add(Class.DirEntry_Playing);
}

/**
 * Populate directory listing with the contents at the given URL and update
 * history. If `url` is `undefined`, the window's current URL is used.
 */
export async function loadDir(url?: string): Promise<void> {
    const info = await fetchDirInfo(url ?? window.location.href);

    if (url === undefined) { // first call
        window.history.replaceState({}, "");
    } else {
        window.history.pushState({}, "", url);
    }

    populateDirs(info);
    populatePlaylists(info);
    populateTracks(info);
}

function populateDirs(info: DirInfo): void {
    let html =
        `<li class="${Class.DirEntry}">
            <i class="${Class.DirIcon} ${Class.MaterialIcons}">arrow_back</i>
            <a class="${Class.DirLink}" href="..">Parent directory</a>
        </li>`;

    for (const dir of info.dirs) {
        html +=
            `<li class="${Class.DirEntry}">
                <i class="${Class.DirIcon} ${Class.MaterialIcons}">folder_open</i>
                <a class="${Class.DirLink}" href="${dir.url}/">${dir.name}/</a>
            </li>`;
    }

    dirList.innerHTML = html;

    const links = dirList.getElementsByTagName("a");
    for (let i = 0; i < links.length; ++i) {
        const link = links[i];

        link.onclick = (e) => {
            e.preventDefault();
            loadDir(link.href); // ignore Promise
        };
    }

    dirList.classList.remove(Class.Hidden);
}

function populatePlaylists(info: DirInfo): void {
    playlistList.classList.add(Class.Hidden);

    if (info.playlists.length === 0) {
        playlistList.innerHTML = "";
        return;
    }

    let html = ``;
    for (const playlist of info.playlists) {
        html +=
            `<li class="${Class.DirEntry}">
                <i class="${Class.DirIcon} ${Class.MaterialIcons}">playlist_play</i>
                <a class="${Class.DirLink}" href="${playlist.url}">${playlist.name}</a>
                <a class="${Class.DirLink} ${Class.DirLink_Aux}" href="${playlist.url}">random</a>
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
            player.playList(link.href, { random: true });
        };
    }

    playlistList.classList.remove(Class.Hidden);
}

function populateTracks(info: DirInfo): void {
    trackList.classList.add(Class.Hidden);

    if (info.tracks.length === 0) {
        trackList.innerHTML = "";
        return;
    }

    let html = ``;
    for (const track of info.tracks) {
        html +=
            `<li class="${Class.DirEntry}">
                <i class="${Class.DirIcon} ${Class.MaterialIcons}">music_note</i>
                <i class="${Class.DirIcon} ${Class.DirIcon_Playing} ${Class.MaterialIcons}">play_arrow</i>
                <a class="${Class.DirLink}" href="${track.url}">${track.name}</a>
            </li>`;
    }
    trackList.innerHTML = html;

    const trackUrls = info.tracks.map(pathUrl => pathUrl.url);

    const links = trackList.getElementsByTagName("a");
    for (let i = 0; i < links.length; ++i) {
        const link = links[i];

        if (isPlaying(link)) {
            setPlayingClass(link);
        }

        link.onclick = (e) => {
            e.preventDefault();
            player.playList(trackUrls, { startPos: i, replayGainHint: ReplayGainMode.Album });
        };
    }

    trackList.classList.remove(Class.Hidden);
}
