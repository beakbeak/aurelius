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
                reject(new Error(`request failed (${req.status}): ${url}`));
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

function copyJson(obj: any) {
    return JSON.parse(JSON.stringify(obj));
}

//
// Player //////////////////////////////////////////////////////////////////////
//
interface PlaylistItem {
    readonly path: string;
    readonly pos: number;
}

interface Playlist {
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
        const pos = randomInt(0, this._urls.length);
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
        const info = await fetchJson<Info>(url);
        return new RemotePlaylist(url, info.length);
    }

    public length(): number {
        return this._length;
    }

    public async at(pos: number): Promise<PlaylistItem | undefined> {
        return nullToUndefined(await fetchJson<PlaylistItem | null>(
            `${this.url}?pos=${pos}`
        ));
    }

    public async random(): Promise<PlaylistItem | undefined> {
        if (this._length < 1) {
            return Promise.resolve(undefined);
        }
        return nullToUndefined(await fetchJson<PlaylistItem | null>(
            `${this.url}?pos=${randomInt(0, this._length)}`
        ));
    }
}

class PlayHistory {
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

interface TrackInfo {
    readonly name: string;
    readonly duration: number;
    readonly replayGainTrack: number;
    readonly replayGainAlbum: number;
    readonly tags: {[key: string]: string | undefined};

    favorite: boolean;
}

interface StreamOptions {
    codec?: "mp3" | "vorbis" | "flac" | "wav";
    quality?: number;
    bitRate?: number;
    sampleRate?: number;
    sampleFormat?: string;
    channelLayout?: string;
//    replayGain?: "track" | "album" | "off";
//    preventClipping?: boolean;
}

class Track {
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
        options = copyJson(options);

        const info = await fetchJson<TrackInfo>(`${url}/info`);

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
        await postJson(`${this.url}/favorite`);
        this.info.favorite = true;
    }

    public async unfavorite(): Promise<void> {
        if (!this.info.favorite) {
            return;
        }
        await postJson(`${this.url}/unfavorite`);
        this.info.favorite = false;
    }

    public currentTime(): number {
        return this.startTime + this.audio.currentTime;
    }
}

class Player {
    private _playButton: HTMLElement;
    private _pauseButton: HTMLElement;
    private _nextButton: HTMLElement;
    private _prevButton: HTMLElement;
    private _progressBarEmpty: HTMLElement;
    private _progressBarFill: HTMLElement;
    private _seekSlider: HTMLElement;
    private _statusRight: HTMLElement;
    private _duration: HTMLElement;
    private _favoriteButton: HTMLElement;
    private _unfavoriteButton: HTMLElement;

    private _track?: Track;
    private _playlist?: Playlist;
    private _history = new PlayHistory();
    private _playlistPos = -1;
    private _random = false;
    private _streamOptions: StreamOptions = { codec: "vorbis", quality: 8 };

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
        this._progressBarEmpty = this._getElement(container, "progress-bar-empty");
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

        this._progressBarEmpty.ondblclick = (event: MouseEvent) => {
            if (this._track !== undefined) {
                const rect = this._progressBarEmpty.getBoundingClientRect();
                this.seekTo(((event.clientX - rect.left) / rect.width) * this._track.info.duration);
            }
        };

        this._updateAll();

        window.addEventListener("resize", () => {
            this._updateStatus(); // update marquee distance
        });
    }

    private _setStatusText(text: string): void {
        const element = this._statusRight;

        element.textContent = text;
        if (element.clientWidth >= element.scrollWidth) {
            element.style.animation = "";
            return;
        }

        const scrollLength = element.scrollWidth - element.clientWidth;
        const scrollTime = scrollLength / 50 /* px/second */;
        const waitTime = 2 /* seconds */;
        const totalTime = 2 * (scrollTime + waitTime);
        const scrollPercent = 100 * (scrollTime / totalTime);
        const waitPercent = 100 * (waitTime / totalTime);

        const style = document.createElement("style");
        style.innerText =
            `@keyframes marquee { ${scrollPercent}% { transform: translateX(-${scrollLength}px); }`
            + ` ${scrollPercent + waitPercent}% {transform: translateX(-${scrollLength}px); }`
            + ` ${2 * scrollPercent + waitPercent}% {transform: translateX(0px);} }`;
        element.appendChild(style);
        element.style.animation = `marquee ${totalTime}s infinite linear`;
    }

    private _updateStatus(): void {
        if (this._track === undefined) {
            this._statusRight.textContent = "";
            this._favoriteButton.style.display = "";
            this._unfavoriteButton.style.display = "none";
            this._favoriteButton.classList.add("inactive");
            return;
        }

        const info = this._track.info;
        let text = "";

        if (info.tags["composer"] !== undefined) {
            text = `${text}${info.tags["composer"]} - `;
        } else if (info.tags["artist"] !== undefined) {
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

        this._setStatusText(text);

        if (info.favorite) {
            this._favoriteButton.style.display = "none";
            this._unfavoriteButton.style.display = "";
            this._unfavoriteButton.classList.remove("inactive");
        } else {
            this._favoriteButton.style.display = "";
            this._unfavoriteButton.style.display = "none";
            this._favoriteButton.classList.remove("inactive");
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
            this._seekSlider.classList.add("inactive");
            this._progressBarEmpty.classList.add("inactive");
            return;
        }
        this._seekSlider.classList.remove("inactive");
        this._progressBarEmpty.classList.remove("inactive");

        const currentTime = this._track.currentTime();
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
            const start = this._track.startTime + ranges.start(0);
            const end = this._track.startTime + ranges.end(ranges.length - 1);
            this._progressBarFill.style.left =
                `${Math.max(0, Math.min(100, (start / this._track.info.duration) * 100))}%`;
            this._progressBarFill.style.width =
                `${Math.max(0, Math.min(100, ((end - start) / this._track.info.duration) * 100))}%`;
        } else {
            this._progressBarFill.style.left = "0";
            this._progressBarFill.style.width = "0";
        }
    }

    private _updateButtons(): void {
        if (this._track === undefined || this._track.audio.paused) {
            this._playButton.style.display = "";
            this._pauseButton.style.display = "none";
            if (this._track === undefined) {
                this._playButton.classList.add("inactive");
            } else {
                this._playButton.classList.remove("inactive");
            }
        } else {
            this._playButton.style.display = "none";
            this._pauseButton.style.display = "";
        }

        if (this.hasNext()) {
            this._nextButton.classList.remove("inactive");
        } else {
            this._nextButton.classList.add("inactive");
        }

        if (this.hasPrevious()) {
            this._prevButton.classList.remove("inactive");
        } else {
            this._prevButton.classList.add("inactive");
        }
    }

    private _updateAll(): void {
        this._updateTime();
        this._updateBuffer();
        this._updateStatus();
        this._updateButtons();
    }

    private async _play(
        url: string,
        startTime?: number,
    ): Promise<void> {
        const track = await Track.fetch(url, this._streamOptions, startTime, this._track);
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
                this._updateAll();
            }
        });

        track.audio.play();
        this._updateAll();
    }

    public seekTo(seconds: number): Promise<void> {
        if (this._track === undefined) {
            return Promise.resolve();
        }
        return this._play(this._track.url, seconds);
    }

    public playTrack(url: string): Promise<void> {
        if (this._playlist !== undefined) {
            this._playlist = undefined;
            this._playlistPos = -1;
            this._history = new PlayHistory();
        }
        this._history.push({ path: url, pos: 0 });
        return this._play(url);
    }

    public async playList(
        playlistUrlOrTrackUrls: string | string[],
        random = false,
        startPos = 0,
    ): Promise<boolean> {
        if (typeof playlistUrlOrTrackUrls === "string") {
            const url = playlistUrlOrTrackUrls;
            this._playlist = await RemotePlaylist.fetch(url);
        } else {
            const trackUrls = playlistUrlOrTrackUrls;
            this._playlist = new LocalPlaylist(trackUrls);
        }

        this._playlistPos = startPos - 1;
        this._history = new PlayHistory();
        this._random = random;

        return this.next();
    }

    public hasNext(): boolean {
        if (this._history.hasNext()) {
            return true;
        }
        if (this._playlist === undefined || this._playlist.length() < 1) {
            return false;
        }
        return this._random || this._playlistPos < (this._playlist.length() - 1);
    }

    public hasPrevious(): boolean {
        if (this._history.hasPrevious()) {
            return true;
        }
        if (this._random || this._playlist === undefined || this._playlist.length() < 1) {
            return false;
        }
        return this._playlistPos > 0;
    }

    public async next(): Promise<boolean> {
        let item = this._history.next();
        if (item === undefined) {
            if (this._playlist === undefined || this._playlist.length() < 1) {
                return false;
            }

            if (this._random) {
                item = await this._playlist.random();
            } else if (this._playlistPos < (this._playlist.length() - 1)) {
                item = await this._playlist.at(this._playlistPos + 1);
            }

            if (item === undefined) {
                return false;
            }
            this._history.push(item);
        }
        this._playlistPos = item.pos;
        await this._play(item.path);
        return true;
    }

    public async previous(): Promise<boolean> {
        let item = this._history.previous();
        if (item === undefined) {
            if (this._playlist === undefined || this._playlist.length() < 1
                || this._playlistPos <= 0)
            {
                return false;
            }

            item = await this._playlist.at(this._playlistPos - 1);
            if (item === undefined) {
                return false;
            }
            this._history.pushFront(item);
        }
        this._playlistPos = item.pos;
        await this._play(item.path);
        return true;
    }

    public pause(): void {
        if (this._track === undefined || this._track.audio.paused) {
            return;
        }
        this._track.audio.pause();
        this._updateAll();
    }

    public unpause(): void {
        if (this._track === undefined || !this._track.audio.paused) {
            return;
        }
        this._track.audio.play();
        this._updateAll();
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

    public setStreamOptions(options: StreamOptions): void {
        this._streamOptions = options;
    }
}