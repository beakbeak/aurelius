import { fetchJson } from "./json";

export interface PathUrl {
    readonly name: string;
    readonly url: string;
}

export interface DirInfo {
    readonly url: string;
    readonly topLevel: string;
    readonly parent: string;
    readonly path: string;
    readonly dirs: PathUrl[];
    readonly playlists: PathUrl[];
    readonly tracks: PathUrl[];
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
