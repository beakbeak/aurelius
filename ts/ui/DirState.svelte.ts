import type { Player } from "../core/player";
import type { DirInfo } from "../core/dir";
import { dirUrlFromTreeUrl, fetchDirInfo, treeUrlFromDirInfo } from "../core/dir";
import { ReplayGainMode, fetchTrackInfo } from "../core/track";
import { RemotePlaylist } from "../core/playlist";

export class DirState {
    dirInfo = $state<DirInfo | undefined>(undefined);

    constructor(public player: Player) {
        window.addEventListener("popstate", () => {
            this.loadDirFromPageUrl();
        });
    }

    async loadDir(url: string, addHistory = true): Promise<DirInfo> {
        const info = await fetchDirInfo(url);
        if (addHistory && info.url !== this.dirInfo?.url) {
            window.history.pushState({}, "", treeUrlFromDirInfo(info));
        }
        this.dirInfo = info;
        setDocumentTitleFromPath(info.path);
        window.scrollTo(0, 0);
        return info;
    }

    async loadDirFromPageUrl(): Promise<void> {
        const url = window.location.href;
        const urlObj = new URL(url);
        const pathParam = urlObj.searchParams.get("dir");
        const playParam = urlObj.searchParams.get("play");

        if (playParam) {
            await this.playTrackByUrl(playParam, false);
            if (this.dirInfo !== undefined) {
                window.history.replaceState({}, "", treeUrlFromDirInfo(this.dirInfo));
            }
        } else if (pathParam) {
            const info = await this.loadDir(pathParam, false);
            window.history.replaceState({}, "", treeUrlFromDirInfo(info));
        } else {
            await this.loadDir(dirUrlFromTreeUrl(url), false);
        }
    }

    async navigateToParent(): Promise<void> {
        if (this.dirInfo) {
            await this.loadDir(this.dirInfo.parent);
        }
    }

    async navigateToTopLevel(): Promise<void> {
        if (this.dirInfo) {
            await this.loadDir(this.dirInfo.topLevel);
        }
    }

    async playTrackByIndex(index: number): Promise<void> {
        if (!this.dirInfo || !this.dirInfo.tracks || index >= this.dirInfo.tracks.length) {
            return;
        }
        const trackUrls = this.dirInfo.tracks.map((track) => track.url);
        await this.player.playList(trackUrls, {
            startPos: index,
            replayGainHint: ReplayGainMode.Album,
        });
    }

    async playTrackByUrl(url: string, addHistory = true): Promise<void> {
        const trackInfo = await fetchTrackInfo(url);
        const info = await this.loadDir(trackInfo.dir, addHistory);
        const index = info.tracks.findIndex((t) => t.url === url);
        if (index >= 0) {
            await this.playTrackByIndex(index);
        } else {
            await this.player.playTrack(url);
        }
    }

    async playFavorites(prefix?: string): Promise<void> {
        const effectivePrefix = prefix ?? this.dirInfo?.path;
        const favoritesUrl = "/media/playlists/favorites";

        if (effectivePrefix !== undefined && effectivePrefix === this.dirInfo?.path) {
            const allFavoritesWithPrefix = await RemotePlaylist.fetch(
                favoritesUrl,
                effectivePrefix,
            );
            const currentDirFavorites = this.dirInfo.tracks.filter((track) => !!track.favorite);
            if (currentDirFavorites.length === allFavoritesWithPrefix.length()) {
                await this.player.playList(
                    currentDirFavorites.map((track) => track.url),
                    { replayGainHint: ReplayGainMode.Album },
                );
                return;
            }
        }
        await this.player.playList(favoritesUrl, { random: true, prefix: effectivePrefix });
    }

    async reloadCurrentDir(): Promise<void> {
        if (this.dirInfo) {
            await this.loadDir(this.dirInfo.url);
        }
    }
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
