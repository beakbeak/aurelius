import { fetchJson, nullToUndefined } from "./json";

export interface PlaylistItem {
    readonly path: string;
    readonly pos: number;
}

export interface Playlist {
    length(): number;
    at(pos: number): Promise<PlaylistItem | undefined>;
    random(): Promise<PlaylistItem | undefined>;
}

/** Return a random integer in the range `[min, max)`.*/
function randomInt(min: number, max: number): number {
    min = Math.ceil(min);
    max = Math.floor(max);
    return Math.floor(Math.random() * (max - min)) + min;
}

/** A playlist constructed and stored by the client. */
export class LocalPlaylist implements Playlist {
    private readonly _urls: string[];

    public constructor(urls: string[]) {
        this._urls = urls;
    }

    public length(): number {
        return this._urls.length;
    }

    public async at(pos: number): Promise<PlaylistItem | undefined> {
        const url = this._urls[pos];
        if (url === undefined) {
            return undefined;
        }
        return { path: url, pos: pos };
    }

    public async random(): Promise<PlaylistItem | undefined> {
        if (this._urls.length < 1) {
            return undefined;
        }
        const pos = randomInt(0, this._urls.length);
        return { path: this._urls[pos], pos: pos };
    }
}

/**
 * A playlist managed by the server. The client does not store the full contents
 * of the playlist.
 */
export class RemotePlaylist implements Playlist {
    public readonly url: string;

    private readonly _length: number;

    private constructor(url: string, length: number) {
        this.url = url;
        this._length = length;
    }

    public static async fetch(url: string): Promise<RemotePlaylist> {
        interface Info {
            length: number;
        }
        const info = await fetchJson<Info>(`${url}/info`);
        return new RemotePlaylist(url, info.length);
    }

    public length(): number {
        return this._length;
    }

    public async at(pos: number): Promise<PlaylistItem | undefined> {
        return nullToUndefined(await fetchJson<PlaylistItem | null>(`${this.url}/${pos}`));
    }

    public async random(): Promise<PlaylistItem | undefined> {
        if (this._length < 1) {
            return Promise.resolve(undefined);
        }
        return nullToUndefined(
            await fetchJson<PlaylistItem | null>(`${this.url}/${randomInt(0, this._length)}`),
        );
    }
}
