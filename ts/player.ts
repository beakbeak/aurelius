import * as util from "./util.js";

export interface PlaylistItem {
    readonly path: string;
    readonly pos: number;
}

export interface Playlist {
    length(): number;
    at(pos: number): Promise<PlaylistItem | undefined>;
    random(): Promise<PlaylistItem | undefined>;
}

class LocalPlaylist implements Playlist {
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

class RemotePlaylist implements Playlist {
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

export class PlayHistory {
    private static readonly _maxLength = 1024;

    private _items: PlaylistItem[] = [];
    private _index = 0;

    public push(item: PlaylistItem): void {
        this._items.splice(this._index + 1, this._items.length - (this._index + 1), item);
        if (this._items.length > PlayHistory._maxLength) {
            this._items.shift();
        } else if (this._items.length > 1) {
            ++this._index;
        }
    }

    public pushFront(item: PlaylistItem): void {
        this._items.splice(this._index, 0, item);
        if (this._items.length > PlayHistory._maxLength) {
            this._items.pop();
        }
    }

    public hasPrevious(): boolean {
        return this._items.length > 1 && this._index > 0;
    }

    public hasNext(): boolean {
        return this._index < (this._items.length - 1);
    }

    public previous(): PlaylistItem | undefined {
        if (this._index === 0) {
            return undefined;
        }
        --this._index;
        return this._items[this._index];
    }

    public next(): PlaylistItem | undefined {
        if (this._index >= (this._items.length - 1)) {
            return undefined;
        }
        ++this._index;
        return this._items[this._index];
    }
}

export interface TrackInfo {
    readonly name: string;
    readonly duration: number;
    readonly replayGainTrack: number;
    readonly replayGainAlbum: number;
    readonly tags: {[key: string]: string | undefined};

    favorite: boolean;
}

export interface StreamOptions {
    codec?: "mp3" | "vorbis" | "flac" | "wav";
    quality?: number;
    bitRate?: number;
    sampleRate?: number;
    sampleFormat?: string;
    channelLayout?: string;
//    replayGain?: "track" | "album" | "off";
//    preventClipping?: boolean;
}

export class Track {
    public readonly url: string;
    public readonly info: TrackInfo;
    public readonly options: StreamOptions;
    public readonly startTime: number;
    public readonly audio: HTMLAudioElement;

    private _listeners: { name: string; func: any; }[] = [];

    private constructor(
        url: string,
        info: TrackInfo,
        options: StreamOptions,
        startTime: number,
        audio: HTMLAudioElement,
    ) {
        this.url = url;
        this.info = info;
        this.options = options;
        this.startTime = startTime;
        this.audio = audio;
    }

    public destroy(): void {
        for (const listener of this._listeners) {
            this.audio.removeEventListener(listener.name, listener.func);
        }
        this.audio.pause();
        this.audio.src = "";
    }

    private static streamQuery(
        options: StreamOptions,
        startTime = 0,
    ): string {
        const keys = Object.keys(options) as (keyof StreamOptions)[];
        let query = "";
        let i = 0;
        for (; i < keys.length; ++i) {
            query += `${i === 0 ? "?" : "&"}${keys[i]}=${options[keys[i]]}`;
        }
        if (startTime > 0) {
            query += `${i === 0 ? "?" : "&"}startTime=${startTime}s`;
        }
        return query;
    }

    public static async fetch(
        url: string,
        options: StreamOptions,
        startTime = 0,
        recycledTrack?: Track,
    ): Promise<Track> {
        options = util.copyJson(options);

        const info = await util.fetchJson<TrackInfo>(`${url}/info`);

        let audio: HTMLAudioElement;
        if (recycledTrack !== undefined) {
            recycledTrack.destroy();
            audio = recycledTrack.audio;
        } else {
            audio = new Audio();
        }
        
        audio.volume = info.replayGainTrack < 1 ? info.replayGainTrack : 1;

        await new Promise((resolve, reject) => {
            audio.src = `${url}/stream${Track.streamQuery(options, startTime)}`;
            audio.oncanplay = () => {
                resolve();
            };
            audio.onerror = (reason) => {
                reject(reason);
            };
        });
        return new Track(url, info, options, startTime, audio);
    }

    public addEventListener<K extends keyof HTMLMediaElementEventMap>(
        name: K,
        func: (this: HTMLAudioElement, ev: HTMLMediaElementEventMap[K]) => any,
        useCapture?: boolean,
    ): void {
        this._listeners.push({ name: name, func: func });
        this.audio.addEventListener(name, func, useCapture);
    }

    public async favorite(): Promise<void> {
        if (this.info.favorite) {
            return;
        }
        await util.postJson(`${this.url}/favorite`);
        this.info.favorite = true;
    }

    public async unfavorite(): Promise<void> {
        if (!this.info.favorite) {
            return;
        }
        await util.postJson(`${this.url}/unfavorite`);
        this.info.favorite = false;
    }

    public currentTime(): number {
        return this.startTime + this.audio.currentTime;
    }
}

export interface PlayerEventMap {
    play: () => void;
    progress: () => void;
    timeupdate: () => void;
    ended: () => void;
    pause: () => void;
    unpause: () => void;
    favorite: () => void;
    unfavorite: () => void;
}
export type PlayerEvent = keyof PlayerEventMap;

export class Player extends util.EventDispatcher<PlayerEventMap> {
    public track?: Track;
    public playlist?: Playlist;
    public history = new PlayHistory();
    public playlistPos = -1;
    public random = false;
    public streamOptions: StreamOptions = { codec: "vorbis", quality: 8 };

    private async _play(
        url: string,
        startTime?: number,
    ): Promise<void> {
        const track = await Track.fetch(url, this.streamOptions, startTime, this.track);
        this.track = track;

        track.addEventListener("progress", () => {
            this._dispatchEvent("progress");
        });
        track.addEventListener("timeupdate", () => {
            this._dispatchEvent("timeupdate");
        });
        track.addEventListener("ended", async () => {
            if (!await this.next()) {
                this.pause();
                if (this.track !== undefined) {
                    this.track.audio.currentTime = 0;
                }
                this._dispatchEvent("ended");
            }
        });

        track.audio.play();
        this._dispatchEvent("play");
    }

    public seekTo(seconds: number): Promise<void> {
        if (this.track === undefined) {
            return Promise.resolve();
        }
        return this._play(this.track.url, seconds);
    }

    public playTrack(url: string): Promise<void> {
        if (this.playlist !== undefined) {
            this.playlist = undefined;
            this.playlistPos = -1;
            this.history = new PlayHistory();
        }
        this.history.push({ path: url, pos: 0 });
        return this._play(url);
    }

    public async playList(
        playlistUrlOrTrackUrls: string | string[],
        random = false,
        startPos = 0,
    ): Promise<boolean> {
        if (typeof playlistUrlOrTrackUrls === "string") {
            const url = playlistUrlOrTrackUrls;
            this.playlist = await RemotePlaylist.fetch(url);
        } else {
            const trackUrls = playlistUrlOrTrackUrls;
            this.playlist = new LocalPlaylist(trackUrls);
        }

        this.playlistPos = startPos - 1;
        this.history = new PlayHistory();
        this.random = random;

        return this.next();
    }

    public hasNext(): boolean {
        if (this.history.hasNext()) {
            return true;
        }
        if (this.playlist === undefined || this.playlist.length() < 1) {
            return false;
        }
        return this.random || this.playlistPos < (this.playlist.length() - 1);
    }

    public hasPrevious(): boolean {
        if (this.history.hasPrevious()) {
            return true;
        }
        if (this.random || this.playlist === undefined || this.playlist.length() < 1) {
            return false;
        }
        return this.playlistPos > 0;
    }

    public async next(): Promise<boolean> {
        let item = this.history.next();
        if (item === undefined) {
            if (this.playlist === undefined || this.playlist.length() < 1) {
                return false;
            }

            if (this.random) {
                item = await this.playlist.random();
            } else if (this.playlistPos < (this.playlist.length() - 1)) {
                item = await this.playlist.at(this.playlistPos + 1);
            }

            if (item === undefined) {
                return false;
            }
            this.history.push(item);
        }
        this.playlistPos = item.pos;
        await this._play(item.path);
        return true;
    }

    public async previous(): Promise<boolean> {
        let item = this.history.previous();
        if (item === undefined) {
            if (this.playlist === undefined || this.playlist.length() < 1
                || this.playlistPos <= 0)
            {
                return false;
            }

            item = await this.playlist.at(this.playlistPos - 1);
            if (item === undefined) {
                return false;
            }
            this.history.pushFront(item);
        }
        this.playlistPos = item.pos;
        await this._play(item.path);
        return true;
    }

    public pause(): void {
        if (this.track === undefined || this.track.audio.paused) {
            return;
        }
        this.track.audio.pause();
        this._dispatchEvent("pause");
    }

    public unpause(): void {
        if (this.track === undefined || !this.track.audio.paused) {
            return;
        }
        this.track.audio.play();
        this._dispatchEvent("unpause");
    }

    public async favorite(): Promise<void> {
        if (this.track === undefined) {
            return;
        }
        await this.track.favorite();
        this._dispatchEvent("favorite");
    }

    public async unfavorite(): Promise<void> {
        if (this.track === undefined) {
            return;
        }
        await this.track.unfavorite();
        this._dispatchEvent("unfavorite");
    }

    public setStreamOptions(options: StreamOptions): void {
        this.streamOptions = options;
    }
}