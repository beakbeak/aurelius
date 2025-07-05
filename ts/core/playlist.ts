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
    private readonly _prefix?: string;

    private constructor(url: string, length: number, prefix?: string) {
        this.url = url;
        this._length = length;
        this._prefix = prefix;
    }

    public static async fetch(url: string, prefix?: string): Promise<RemotePlaylist> {
        interface Info {
            length: number;
        }
        
        // Add prefix to the info fetch URL if provided
        const fetchUrl = prefix ? `${url}?prefix=${encodeURIComponent(prefix)}` : url;
        const info = await fetchJson<Info>(fetchUrl);
        return new RemotePlaylist(url, info.length, prefix);
    }

    public length(): number {
        return this._length;
    }

    public async at(pos: number): Promise<PlaylistItem | undefined> {
        const baseUrl = `${this.url}/tracks/${pos}`;
        const fetchUrl = this._prefix ? `${baseUrl}?prefix=${encodeURIComponent(this._prefix)}` : baseUrl;
        return nullToUndefined(await fetchJson<PlaylistItem | null>(fetchUrl));
    }

    public async random(): Promise<PlaylistItem | undefined> {
        if (this._length < 1) {
            return Promise.resolve(undefined);
        }
        const baseUrl = `${this.url}/tracks/${randomInt(0, this._length)}`;
        const fetchUrl = this._prefix ? `${baseUrl}?prefix=${encodeURIComponent(this._prefix)}` : baseUrl;
        return nullToUndefined(await fetchJson<PlaylistItem | null>(fetchUrl));
    }
}
