import type { Player } from "../../core/player";
import type { DirInfo } from "../../core/dir";
import { dirUrlFromTreeUrl, fetchDirInfo, treeUrlFromDirInfo } from "../../core/dir";
import { ReplayGainMode, fetchTrackInfo } from "../../core/track";
import { RemotePlaylist } from "../../core/playlist";

export type DirState = ReturnType<typeof createDirState>;

export function createDirState(player: Player) {
    let dirInfo = $state<DirInfo | undefined>(undefined);

    async function loadDir(url: string, addHistory = true): Promise<DirInfo> {
        const info = await fetchDirInfo(url);
        if (addHistory && info.url !== dirInfo?.url) {
            window.history.pushState({}, "", treeUrlFromDirInfo(info));
        }
        dirInfo = info;
        setDocumentTitleFromPath(info.path);
        window.scrollTo(0, 0);
        return info;
    }

    async function loadDirFromPageUrl(): Promise<void> {
        const url = window.location.href;
        const urlObj = new URL(url);
        const pathParam = urlObj.searchParams.get("dir");
        const playParam = urlObj.searchParams.get("play");

        if (playParam) {
            await playTrackByUrl(playParam, false);
            if (dirInfo !== undefined) {
                window.history.replaceState({}, "", treeUrlFromDirInfo(dirInfo));
            }
        } else if (pathParam) {
            const info = await loadDir(pathParam, false);
            window.history.replaceState({}, "", treeUrlFromDirInfo(info));
        } else {
            await loadDir(dirUrlFromTreeUrl(url), false);
        }
    }

    async function navigateToParent(): Promise<void> {
        if (dirInfo) {
            await loadDir(dirInfo.parent);
        }
    }

    async function navigateToTopLevel(): Promise<void> {
        if (dirInfo) {
            await loadDir(dirInfo.topLevel);
        }
    }

    async function playTrackByIndex(index: number): Promise<void> {
        if (!dirInfo || !dirInfo.tracks || index >= dirInfo.tracks.length) {
            return;
        }
        const trackUrls = dirInfo.tracks.map((track) => track.url);
        await player.playList(trackUrls, {
            startPos: index,
            replayGainHint: ReplayGainMode.Album,
        });
    }

    async function playTrackByUrl(url: string, addHistory = true): Promise<void> {
        const trackInfo = await fetchTrackInfo(url);
        const info = await loadDir(trackInfo.dir, addHistory);
        const index = info.tracks.findIndex((t) => t.url === url);
        if (index >= 0) {
            await playTrackByIndex(index);
        } else {
            await player.playTrack(url);
        }
    }

    async function playFavorites(prefix?: string): Promise<void> {
        const effectivePrefix = prefix ?? dirInfo?.path;
        const favoritesUrl = "/media/playlists/favorites";

        if (effectivePrefix !== undefined && effectivePrefix === dirInfo?.path) {
            const allFavoritesWithPrefix = await RemotePlaylist.fetch(
                favoritesUrl,
                effectivePrefix,
            );
            const currentDirFavorites = dirInfo.tracks.filter((track) => !!track.favorite);
            if (currentDirFavorites.length === allFavoritesWithPrefix.length()) {
                await player.playList(
                    currentDirFavorites.map((track) => track.url),
                    { replayGainHint: ReplayGainMode.Album },
                );
                return;
            }
        }
        await player.playList(favoritesUrl, { random: true, prefix: effectivePrefix });
    }

    async function reloadCurrentDir(): Promise<void> {
        if (dirInfo) {
            await loadDir(dirInfo.url);
        }
    }

    window.addEventListener("popstate", () => {
        loadDirFromPageUrl();
    });

    return {
        get dirInfo() {
            return dirInfo;
        },
        loadDir,
        loadDirFromPageUrl,
        navigateToParent,
        navigateToTopLevel,
        playTrackByIndex,
        playTrackByUrl,
        playFavorites,
        reloadCurrentDir,
    };
}

function setDocumentTitleFromPath(path: string): void {
    const cleanedPath = path.replace(/\/$/g, "");
    if (cleanedPath === "") {
        document.title = "aurelius";
        return;
    }
    const urlTokens = cleanedPath.split("/");
    const leafDir = urlTokens[urlTokens.length - 1];
    document.title = `${leafDir} | aurelius`;
}
