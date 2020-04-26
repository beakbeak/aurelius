import * as util from "./util";

export interface TrackInfo {
    readonly name: string;
    readonly duration: number;
    readonly replayGainTrack: number;
    readonly replayGainAlbum: number;
    readonly tags: {[key: string]: string | undefined};

    favorite: boolean;
}

export async function fetchTrackInfo(url: string): Promise<TrackInfo> {
    return util.fetchJson<TrackInfo>(`${url}/info`);
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
    private _listeners: { name: string; func: any; }[] = [];

    private constructor(
        public readonly url: string,
        public readonly info: TrackInfo,
        public readonly options: StreamOptions,
        public readonly startTime: number,
        private readonly _audio: HTMLAudioElement,
        private readonly _playablePromise: Promise<void>,
    ) {
    }

    public destroy(): void {
        for (const listener of this._listeners) {
            this._audio.removeEventListener(listener.name, listener.func);
        }
        this._listeners.length = 0;

        this._audio.pause();
        this._audio.src = "";
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

        const info = await fetchTrackInfo(url);

        let audio: HTMLAudioElement;
        if (recycledTrack !== undefined) {
            recycledTrack.destroy();
            audio = recycledTrack._audio;
        } else {
            audio = new Audio();
        }
        
        audio.volume = info.replayGainTrack < 1 ? info.replayGainTrack : 1;

        const playablePromise = new Promise<void>((resolve, reject) => {
            audio.src = `${url}/stream${Track.streamQuery(options, startTime)}`;
            audio.oncanplay = () => {
                resolve();
            };
            audio.onerror = (reason) => {
                reject(reason);
            };

            audio.load();
        });
        return new Track(url, info, options, startTime, audio, playablePromise);
    }

    public isPlayable(): boolean {
        return this._audio.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA;
    }

    public waitUntilPlayable(): Promise<void> {
        return this._playablePromise;
    }

    public async play(): Promise<void> {
        if (!this.isPlayable()) {
            await this.waitUntilPlayable();
        }
        this._audio.play();
    }

    public pause(): void {
        this._audio.pause();
    }

    public isPaused(): boolean {
        return this._audio.paused;
    }

    public addEventListener<K extends keyof HTMLMediaElementEventMap>(
        name: K,
        func: (this: HTMLAudioElement, ev: HTMLMediaElementEventMap[K]) => any,
        useCapture?: boolean,
    ): void {
        this._listeners.push({ name: name, func: func });
        this._audio.addEventListener(name, func, useCapture);
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
        return this.startTime + this._audio.currentTime;
    }

    public rewind(): void {
        this._audio.currentTime = 0;
    }

    public buffered(): TimeRanges {
        return this._audio.buffered;
    }

    public seekTo(seconds: number): boolean {
        const adjustedTime = seconds - this.startTime;

        const ranges = this._audio.buffered;
        for (let i = 0; i < ranges.length; ++i) {
            if (adjustedTime >= ranges.start(i) && adjustedTime < ranges.end(i)) {
                this._audio.currentTime = adjustedTime;
                return true;
            }
        }
        return false;
    }
}
