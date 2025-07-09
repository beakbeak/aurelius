import { Player } from "../core/player";
import { DirInfo, dirUrlFromTreeUrl, fetchDirInfo, treeUrlFromDirInfo } from "../core/dir";
import { ReplayGainMode, fetchTrackInfo } from "../core/track";
import { Class } from "./class";
import { closestAncestorWithClass } from "./dom";

let player: Player;

let specialList: HTMLElement;
let navigationList: HTMLElement;
let dirList: HTMLElement;
let playlistList: HTMLElement;
let trackList: HTMLElement;

let lastPlaying: HTMLElement | undefined;
let currentDirInfo: DirInfo | undefined;

export async function setupDirUi(inPlayer: Player) {
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
    navigationList = createList();
    dirList = createList();
    playlistList = createList();
    trackList = createList();

    container.innerHTML = "";
    container.appendChild(specialList);
    container.appendChild(navigationList);
    container.appendChild(dirList);
    container.appendChild(playlistList);
    container.appendChild(trackList);

    populateSpecial();

    player.addEventListener("play", highlightPlayingTrack);
    player.addEventListener("ended", unhighlightPlayingTrack);
    player.addEventListener("favorite", () => {
        if (currentDirInfo) {
            loadDir(currentDirInfo.url);
        }
    });
    player.addEventListener("unfavorite", () => {
        if (currentDirInfo) {
            loadDir(currentDirInfo.url);
        }
    });

    window.onpopstate = () => {
        loadDirFromPageUrl();
    };

    await loadDirFromPageUrl();
}

function populateSpecial(info?: DirInfo): void {
    let favoritesText = "Favorites";
    let prefix: string | undefined;

    if (info && info.path !== "/") {
        // Extract directory name from path (e.g., "/foo/bar" -> "bar")
        const pathParts = info.path.replace(/\/$/, "").split("/");
        const dirName = pathParts[pathParts.length - 1];
        favoritesText = `Favorites in ${dirName}/`;
        prefix = info.path;
    }

    specialList.innerHTML =
        //
        `<li class="${Class.DirEntry}">
            <i class="${Class.DirIcon} ${Class.MaterialIcons}">favorite_border</i>
            <a class="${Class.DirLink}" href="#">${favoritesText}</a>
        </li>`;

    const favoritesLink = specialList.querySelector("a")!;

    favoritesLink.onclick = (e) => {
        e.preventDefault();
        playFavorites(prefix);
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
    const trackUrl = element.getAttribute("data-url");
    if (!trackUrl) {
        return false;
    }
    return player.track.url.endsWith(trackUrl);
}

function setPlayingClass(element: HTMLAnchorElement | undefined): void {
    lastPlaying?.classList.remove(Class.DirEntry_Playing);
    lastPlaying =
        element !== undefined ? closestAncestorWithClass(element, Class.DirEntry) : undefined;
    lastPlaying?.classList.add(Class.DirEntry_Playing);
}

async function loadDirFromPageUrl(): Promise<void> {
    const url = window.location.href;
    const urlObj = new URL(url);
    const pathParam = urlObj.searchParams.get("dir");
    const playParam = urlObj.searchParams.get("play");

    if (playParam) {
        await playTrackByUrl(playParam, /*addHistory=*/ false);
        if (currentDirInfo !== undefined) {
            window.history.replaceState({}, "", treeUrlFromDirInfo(currentDirInfo));
        }
    } else if (pathParam) {
        const info = await loadDir(pathParam, /*addHistory=*/ false);
        window.history.replaceState({}, "", treeUrlFromDirInfo(info));
    } else {
        await loadDir(dirUrlFromTreeUrl(url), /*addHistory=*/ false);
    }
}

/**
 * Populate directory listing with the contents at the given URL and update
 * history. If `url` is `undefined`, the window's current URL is used.
 */
export async function loadDir(url: string, addHistory = true): Promise<DirInfo> {
    const info = await fetchDirInfo(url);
    currentDirInfo = info;
    if (addHistory) {
        window.history.pushState({}, "", treeUrlFromDirInfo(info));
    }
    setDocumentTitleFromPath(info.path);
    populateSpecial(info);
    populateNavigation(info);
    populateDirs(info);
    populatePlaylists(info);
    populateTracks(info);
    return info;
}

function setDocumentTitleFromPath(path: string) {
    const cleanedPath = path.replace(/\/$/g, "");
    if (cleanedPath === "") {
        document.title = `aurelius`;
        return;
    }
    const urlTokens = cleanedPath.split("/");
    const leafDir = urlTokens[urlTokens.length - 1];
    document.title = `${leafDir} | aurelius`;
}

function activateDirLinks(list: HTMLElement): void {
    const links = list.getElementsByTagName("a");
    for (let i = 0; i < links.length; ++i) {
        const link = links[i];

        link.onclick = (e) => {
            e.preventDefault();
            const dirUrl = link.getAttribute("data-url");
            if (dirUrl !== null) {
                loadDir(dirUrl); // ignore Promise
            }
        };
    }
}

function populateNavigation(info: DirInfo): void {
    navigationList.innerHTML =
        //
        `<li class="${Class.DirEntry}">
            <i class="${Class.DirIcon} ${Class.MaterialIcons}">vertical_align_top</i>
            <a class="${Class.DirLink}"
                href="/media/tree/?dir=${encodeURIComponent(info.topLevel)}"
                data-url="${info.topLevel}"
            >Top level</a>
        </li>
        <li class="${Class.DirEntry}">
            <i class="${Class.DirIcon} ${Class.MaterialIcons}">arrow_back</i>
            <a class="${Class.DirLink}"
                href="/media/tree/?dir=${encodeURIComponent(info.parent)}"
                data-url="${info.parent}"
            >Parent directory</a>
        </li>`;
    activateDirLinks(navigationList);
    navigationList.classList.remove(Class.Hidden);
}

function populateDirs(info: DirInfo): void {
    let html = "";
    for (const dir of info.dirs) {
        html +=
            //
            `<li class="${Class.DirEntry}">
                <i class="${Class.DirIcon} ${Class.MaterialIcons}">folder_open</i>
                <a class="${Class.DirLink}"
                    href="/media/tree/?dir=${encodeURIComponent(dir.url)}"
                    data-url="${dir.url}"
                >${dir.name}/</a>
            </li>`;
    }
    dirList.innerHTML = html;
    activateDirLinks(dirList);
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
            //
            `<li class="${Class.DirEntry}">
                <i class="${Class.DirIcon} ${Class.MaterialIcons}">playlist_play</i>
                <a class="${Class.DirLink}" href="#" data-url="${playlist.url}">${playlist.name}</a>
                <a class="${Class.DirLink} ${Class.DirLink_Aux}" href="#" data-url="${playlist.url}">random</a>
            </li>`;
    }
    playlistList.innerHTML = html;

    const links = playlistList.getElementsByTagName("a");
    for (let i = 0; i < links.length; i += 2) {
        const link = links[i];
        const randomLink = links[i + 1];

        link.onclick = (e) => {
            e.preventDefault();
            const playlistUrl = link.getAttribute("data-url");
            if (playlistUrl) {
                player.playList(playlistUrl);
            }
        };
        randomLink.onclick = (e) => {
            e.preventDefault();
            const playlistUrl = randomLink.getAttribute("data-url");
            if (playlistUrl) {
                player.playList(playlistUrl, { random: true });
            }
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
        const iconName = track.favorite ? "favorite_border" : "music_note";
        html +=
            //
            `<li class="${Class.DirEntry}">
                <i class="${Class.DirIcon} ${Class.MaterialIcons}">${iconName}</i>
                <i class="${Class.DirIcon} ${Class.DirIcon_Playing} ${Class.MaterialIcons}"
                >play_arrow</i>
                <a class="${Class.DirLink}"
                    href="/media/tree/?play=${encodeURIComponent(track.url)}"
                    data-url="${track.url}"
                >${track.name}</a>
            </li>`;
    }
    trackList.innerHTML = html;

    const links = trackList.getElementsByTagName("a");
    for (let i = 0; i < links.length; ++i) {
        const link = links[i];

        if (isPlaying(link)) {
            setPlayingClass(link);
        }

        link.onclick = (e) => {
            e.preventDefault();
            playTrackByIndex(i, info);
        };
    }

    trackList.classList.remove(Class.Hidden);
}

export async function playTrackByUrl(url: string, addHistory = true): Promise<void> {
    const trackInfo = await fetchTrackInfo(url);
    const dirInfo = await loadDir(trackInfo.dir, addHistory);
    const index = dirInfo.tracks.findIndex((t) => t.url === url);
    if (index >= 0) {
        await playTrackByIndex(index, dirInfo);
    } else {
        await player.playTrack(url);
    }
}

export async function playTrackByIndex(index: number, dirInfo = currentDirInfo): Promise<void> {
    if (!dirInfo || !dirInfo.tracks || index >= dirInfo.tracks.length) {
        return;
    }
    const trackUrls = dirInfo.tracks.map((track) => track.url);
    await player.playList(trackUrls, { startPos: index, replayGainHint: ReplayGainMode.Album });
}

export async function navigateToParent(): Promise<void> {
    if (currentDirInfo) {
        await loadDir(currentDirInfo.parent);
    }
}

export async function navigateToTopLevel(): Promise<void> {
    if (currentDirInfo) {
        await loadDir(currentDirInfo.topLevel);
    }
}

export function playFavorites(prefix?: string): void {
    if (prefix === undefined && currentDirInfo && currentDirInfo.path !== "/") {
        prefix = currentDirInfo.path;
    }
    player.playList("/media/playlists/favorites", { random: true, prefix });
}
