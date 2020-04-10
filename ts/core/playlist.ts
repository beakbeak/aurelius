import * as util from "./util";

export interface PlaylistItem {
    readonly path: string;
    readonly pos: number;
}

export interface Playlist {
    length(): number;
    at(pos: number): Promise<PlaylistItem | undefined>;
    random(): Promise<PlaylistItem | undefined>;
}

export class LocalPlaylist implements Playlist {
    private readonly _urls: string[];

    public constructor(urls: string[]) {
        this._urls = urls;
    }

    public length(): number {
        return this._urls.length;
    }

    public at(pos: number): Promise<PlaylistItem | undefined> {
        const url = this._urls[pos];
        if (url === undefined) {
            return Promise.resolve(undefined);
        }
        return Promise.resolve({ path: url, pos: pos });
    }

    public random(): Promise<PlaylistItem | undefined> {
        if (this._urls.length < 1) {
            return Promise.resolve(undefined);
        }
        const pos = util.randomInt(0, this._urls.length);
        return Promise.resolve({ path: this._urls[pos], pos: pos });
    }
}

export class RemotePlaylist implements Playlist {
    public readonly url: string;

    private readonly _length: number;

    private constructor(
        url: string,
        length: number,
    ) {
        this.url = url;
        this._length = length;
    }

    public static async fetch(url: string): Promise<Playlist> {
        interface Info {
            length: number;
        }
        const info = await util.fetchJson<Info>(url);
        return new RemotePlaylist(url, info.length);
    }

    public length(): number {
        return this._length;
    }

    public async at(pos: number): Promise<PlaylistItem | undefined> {
        return util.nullToUndefined(await util.fetchJson<PlaylistItem | null>(
            `${this.url}?pos=${pos}`
        ));
    }

    public async random(): Promise<PlaylistItem | undefined> {
        if (this._length < 1) {
            return Promise.resolve(undefined);
        }
        return util.nullToUndefined(await util.fetchJson<PlaylistItem | null>(
            `${this.url}?pos=${util.randomInt(0, this._length)}`
        ));
    }
}
