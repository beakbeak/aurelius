import { stripQueryString } from "./url";
import { fetchJson } from "./json";

export interface PathUrl {
    readonly name: string;
    readonly url: string;
}

export interface DirInfo {
    readonly dirs: PathUrl[];
    readonly playlists: PathUrl[];
    readonly tracks: PathUrl[];
    readonly notes: string;
}

export function fetchDirInfo(url: string): Promise<DirInfo> {
    return fetchJson<DirInfo>(`${stripQueryString(url)}?info`);
}
