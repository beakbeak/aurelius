import { fetchJson } from "./json";
import type { TrackInfo } from "./track";

export interface DirEntry {
    readonly name: string;
    readonly url: string;
}

export interface DirInfo {
    readonly url: string;
    readonly topLevel: string;
    readonly parent: string;
    readonly path: string;
    readonly dirs: DirEntry[];
    readonly playlists: DirEntry[];
    readonly tracks: TrackInfo[];
}

export function fetchDirInfo(url: string): Promise<DirInfo> {
    return fetchJson<DirInfo>(url);
}

export function treeUrlFromDirInfo(info: DirInfo): string {
    return encodeURI(`/media/tree/${info.path}`);
}

export function dirUrlFromTreeUrl(treeUrl: string): string {
    const treePath = decodeURIComponent(new URL(treeUrl).pathname);
    const dirPath = encodeURIComponent(treePath.replace(/^\/media\/tree\/|\/+$/g, ``));
    return `/media/dirs/at:${dirPath}`;
}
