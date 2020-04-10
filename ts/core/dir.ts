import * as util from "./util";

export interface PathUrl {
    readonly name: string;
    readonly url: string;
}

export interface DirInfo {
    readonly dirs: PathUrl[];
    readonly playlists: PathUrl[];
    readonly tracks: PathUrl[];
}

export function fetchDirInfo(url: string): Promise<DirInfo> {
    return util.fetchJson<DirInfo>(`${util.stripQueryString(url)}?info`);
}
