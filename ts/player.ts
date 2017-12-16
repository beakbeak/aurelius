//
// Utilities ///////////////////////////////////////////////////////////////////
//
function sendJsonRequest<Response>(
    method: string,
    url: string,
    data?: any,
): Promise<Response> {
    const req = new XMLHttpRequest();
    req.open(method, url);
    return new Promise((resolve, reject) => {
        req.onreadystatechange = () => {
            if (req.readyState !== XMLHttpRequest.DONE) {
                return;
            }
            if (req.status === 200) {
                resolve(JSON.parse(req.responseText));
            } else {
                reject(new Error("request failed"));
            }
        }

        if (data !== undefined) {
            req.setRequestHeader("Content-Type", "application/json");
            req.send(JSON.stringify(data));
        } else {
            req.send();
        }
    });
}

function fetchJson<Response>(url: string): Promise<Response> {
    return sendJsonRequest<Response>("GET", url);
}

function nullToUndefined<T>(value: T | null): T | undefined {
    return value !== null ? value : undefined;
}

function postJson<Response>(
    url: string,
    data?: any,
): Promise<Response> {
    return sendJsonRequest<Response>("POST", url, data);
}

// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Math/random
// The maximum is exclusive and the minimum is inclusive
function randomInt(
    min: number,
    max: number,
): number {
    min = Math.ceil(min);
    max = Math.floor(max);
    return Math.floor(Math.random() * (max - min)) + min;
}

//
// Player //////////////////////////////////////////////////////////////////////
//
interface PlaylistItem {
    readonly path: string;
    readonly pos: number;
}

class Playlist {
    public readonly url: string;
    public readonly length: number;

    private constructor(
        url: string,
        length: number,
    ) {
        this.url = url;
        this.length = length;
    }

    public static async fetch(url: string): Promise<Playlist> {
        interface Info {
            length: number;
        }
        const info = await fetchJson<Info>(url);
        return new Playlist(url, info.length);
    }

    public async at(pos: number): Promise<PlaylistItem | undefined> {
        return nullToUndefined(await fetchJson<PlaylistItem | null>(
            `${this.url}?pos=${pos}`
        ));
    }

    public async random(): Promise<PlaylistItem | undefined> {
        if (this.length < 1) {
            return Promise.resolve(undefined);
        }
        return nullToUndefined(await fetchJson<PlaylistItem | null>(
            `${this.url}?pos=${randomInt(0, this.length)}`
        ));
    }
}

class PlayHistory {
    private static readonly _maxLength = 1024;

    private _urls: string[] = [];
    private _index = 0;

    public push(url: string): void {
        this._urls.splice(this._index + 1, this._urls.length - (this._index + 1), url);
        if (this._urls.length > PlayHistory._maxLength) {
            this._urls.shift();
        } else if (this._urls.length > 1) {
            ++this._index;
        }
    }

    public previous(): string | undefined {
        if (this._index === 0) {
            return undefined;
        }
        --this._index;
        return this._urls[this._index];
    }

    public next(): string | undefined {
        if (this._index >= (this._urls.length - 1)) {
            return undefined;
        }
        ++this._index;
        return this._urls[this._index];
    }
}

interface TrackInfo {
    readonly name: string;
    readonly duration: number;
    readonly replayGainTrack: number;
    readonly replayGainAlbum: number;
    readonly tags: {[key: string]: string | undefined};

    favorite: boolean;
}

interface StreamOptions {
    readonly codec?: "mp3" | "vorbis" | "flac";
    readonly quality?: number;
    readonly bitRate?: number;
    readonly sampleRate?: number;
    readonly sampleFormat?: string;
    readonly channelLayout?: string;
//    readonly replayGain?: "track" | "album" | "off";
//    readonly preventClipping?: boolean;
}

class Track {
    public readonly url: string;
    public readonly info: TrackInfo;
    public readonly audio: HTMLAudioElement;

    private _listeners: { name: string; func: any; }[] = [];

    private constructor(
        url: string,
        info: TrackInfo,
        audio: HTMLAudioElement,
    ) {
        this.url = url;
        this.info = info;
        this.audio = audio;
    }

    public destroy(): void {
        for (const listener of this._listeners) {
            this.audio.removeEventListener(listener.name, listener.func);
        }
        this.audio.pause();
        this.audio.src = "";
    }

    private static streamQuery(options: StreamOptions): string {
        const keys = Object.keys(options) as (keyof StreamOptions)[];
        let query = "";
        for (let i = 0; i < keys.length; ++i) {
            query += `${i === 0 ? "?" : "&"}${keys[i]}=${options[keys[i]]}`;
        }
        return query;
    }

    public static async fetch(
        url: string,
        options: StreamOptions,
    ): Promise<Track> {
        const [info, audio] = await Promise.all([
            fetchJson<TrackInfo>(`${url}/info`),
            new Promise<HTMLAudioElement>((resolve) => {
                const audio = new Audio(`${url}/stream${Track.streamQuery(options)}`);
                audio.oncanplay = () => {
                    resolve(audio);
                };
            }),
        ]);

        if (info.replayGainTrack < 1) {
            audio.volume = info.replayGainTrack;
        }

        return new Track(url, info, audio);
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
        await postJson(`${this.url}/favorite`);
        this.info.favorite = true;
    }

    public async unfavorite(): Promise<void> {
        await postJson(`${this.url}/unfavorite`);
        this.info.favorite = false;
    }
}

class Player {
    private _playButton: HTMLElement;
    private _pauseButton: HTMLElement;
    private _nextButton: HTMLElement;
    private _prevButton: HTMLElement;
    private _progressBarFill: HTMLElement;
    private _seekSlider: HTMLElement;
    private _statusRight: HTMLElement;
    private _duration: HTMLElement;
    private _favoriteButton: HTMLElement;
    private _unfavoriteButton: HTMLElement;

    private _track?: Track;
    private _playlist?: Playlist;
    private _history = new PlayHistory(); 

    private _getElement(
        container: HTMLElement,
        id: string,
    ): HTMLElement {
        const element = container.querySelector(`#${id}`);
        if (element === null) {
            throw new Error(`missing ${id}`);
        }
        return element as HTMLElement;
    }

    constructor(containerId: string) {
        const container = document.getElementById(containerId);
        if (container === null) {
            throw new Error("invalid container");
        }

        this._statusRight = this._getElement(container, "status-right");
        this._playButton = this._getElement(container, "play-button");
        this._pauseButton = this._getElement(container, "pause-button");
        this._nextButton = this._getElement(container, "next-button");
        this._prevButton = this._getElement(container, "prev-button");
        this._progressBarFill = this._getElement(container, "progress-bar-fill");
        this._seekSlider = this._getElement(container, "seek-slider");
        this._duration = this._getElement(container, "duration");
        this._favoriteButton = this._getElement(container, "favorite-button");
        this._unfavoriteButton = this._getElement(container, "unfavorite-button");

        this._playButton.onclick = () => {
            this.unpause();
        };

        this._pauseButton.style.display = "none";
        this._pauseButton.onclick = () => {
            this.pause();
        };

        this._nextButton.onclick = () => {
            this.next();
        };
        this._prevButton.onclick = () => {
            this.previous();
        };

        this._favoriteButton.onclick = () => {
            this.favorite();
        };

        this._unfavoriteButton.style.display = "none";
        this._unfavoriteButton.onclick = () => {
            this.unfavorite();
        };
    }

    private _updateStatus(): void {
        if (this._track === undefined) {
            return;
        }
        const info = this._track.info;

        let text = "";
        if (info.tags["artist"] !== undefined) {
            text = `${text}${info.tags["artist"]} - `;
        }
        if (info.tags["title"] !== undefined) {
            text = `${text}${info.tags["title"]}`;
        } else {
            text = `${text}${info.name}`
        }
        if (info.tags["album"] !== undefined) {
            let track = "";
            if (info.tags["track"] !== undefined) {
                track = ` #${info.tags["track"]}`;
            }
            text = `${text} [${info.tags["album"]}${track}]`;
        }

        this._statusRight.textContent = text;

        if (info.favorite) {
            this._favoriteButton.style.display = "none";
            this._unfavoriteButton.style.display = "";
        } else {
            this._favoriteButton.style.display = "";
            this._unfavoriteButton.style.display = "none";
        }
    }

    private _secondsToString(totalSeconds: number): string {
        const minutes = (totalSeconds / 60) | 0;
        const seconds = (totalSeconds - minutes * 60) | 0;
        if (seconds < 10) {
            return `${minutes}:0${seconds}`;
        } else {
            return `${minutes}:${seconds}`;
        }
    }

    private _updateTime(): void {
        if (this._track === undefined) {
            this._duration.textContent = "";
            this._seekSlider.style.left = "0";
            return;
        }

        const currentTime = this._track.audio.currentTime;
        const duration = this._track.info.duration;
        const currentTimeStr = this._secondsToString(currentTime);
        const durationStr = this._secondsToString(duration);

        this._duration.textContent = `${currentTimeStr} / ${durationStr}`;

        if (duration > 0) {
            this._seekSlider.style.left = `${(currentTime / duration) * 100}%`;
        } else {
            this._seekSlider.style.left = "0";
        }
    }

    private _updateBuffer(): void {
        if (this._track === undefined) {
            this._progressBarFill.style.left = "0";
            this._progressBarFill.style.width = "0";
            return;
        }

        const ranges = this._track.audio.buffered;
        if (ranges.length > 0 && this._track.info.duration > 0) {
            const start = ranges.start(0);
            const end = ranges.end(ranges.length - 1);
            this._progressBarFill.style.left =
                `${(start / this._track.info.duration) * 100}%`;
            this._progressBarFill.style.width =
                `${((end - start) / this._track.info.duration) * 100}%`;
        } else {
            this._progressBarFill.style.left = "0";
            this._progressBarFill.style.width = "0";
        }
    }

    private async _play(url: string): Promise<void> {
        const track = await Track.fetch(url, { codec: "vorbis", quality: 8 });

        if (this._track !== undefined) {
            this._track.destroy();
        }
        this._track = track;

        track.addEventListener("progress", () => {
            this._updateBuffer();
        });
        track.addEventListener("timeupdate", () => {
            this._updateTime();
            this._updateBuffer();
        });
        track.addEventListener("ended", async () => {
            if (!await this.next()) {
                this.pause();
                if (this._track !== undefined) {
                    this._track.audio.currentTime = 0;
                }
                this._updateBuffer();
                this._updateTime();
                this._updateStatus();
            }
        });

        track.audio.play();

        this._progressBarFill.style.left = "0";
        this._progressBarFill.style.width = "0";
        this._playButton.style.display = "none";
        this._pauseButton.style.display = "";

        this._updateStatus();
    }

    public playTrack(url: string): Promise<void> {
        if (this._playlist !== undefined) {
            this._playlist = undefined;
            this._history = new PlayHistory();
        }
        this._history.push(url);
        return this._play(url);
    }

    public async playList(url: string): Promise<boolean> {
        this._playlist = await Playlist.fetch(url);
        this._history = new PlayHistory();
        return this.next();
    }

    public async next(): Promise<boolean> {
        let url = this._history.next();
        if (url === undefined) {
            if (this._playlist !== undefined) {
                const item = await this._playlist.random();
                if (item !== undefined) {
                    url = item.path;
                }
            }
            if (url === undefined) {
                return false;
            }
            this._history.push(url);
        }
        await this._play(url);
        return true;
    }

    public async previous(): Promise<boolean> {
        let url = this._history.previous();
        if (url === undefined) {
            return false;
        }
        await this._play(url);
        return true;
    }

    public pause(): void {
        if (this._track === undefined) {
            return;
        }
        this._track.audio.pause();
        this._pauseButton.style.display = "none";
        this._playButton.style.display = "";
    }

    public unpause(): void {
        if (this._track === undefined) {
            return;
        }
        this._track.audio.play();
        this._playButton.style.display = "none";
        this._pauseButton.style.display = "";
    }

    public async favorite(): Promise<void> {
        if (this._track === undefined) {
            return;
        }
        await this._track.favorite();
        this._updateStatus();
    }

    public async unfavorite(): Promise<void> {
        if (this._track === undefined) {
            return;
        }
        await this._track.unfavorite();
        this._updateStatus();
    }
}