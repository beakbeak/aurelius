interface FileInfo {
    name: string;
    duration: number;
    replayGainTrack: number;
    replayGainAlbum: number;
    favorite: boolean;
    tags: {[key: string]: string | undefined};
}

interface PlaylistItem {
    path: string;
    pos: number;
}

function sendJsonRequest(
    method: string,
    url: string,
    data?: any,
): Promise<any> {
    const req = new XMLHttpRequest();
    req.open(method, url);
    return new Promise((resolve, reject) => {
        req.onreadystatechange = () => {
            if (req.readyState !== XMLHttpRequest.DONE) {
                return;
            }
            if (req.status === 200) {
                resolve(req.responseText !== "" ? JSON.parse(req.responseText) : undefined);
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

function fetchJson(url: string): Promise<any> {
    return sendJsonRequest("GET", url);
}

function postJson(
    url: string,
    data?: any,
): Promise<any> {
    return sendJsonRequest("POST", url, data);
}

class Player {
    private _playButton: HTMLElement;
    private _pauseButton: HTMLElement;
    private _nextButton: HTMLElement;
    private _progressBarFill: HTMLElement;
    private _seekSlider: HTMLElement;
    private _statusRight: HTMLElement;
    private _duration: HTMLElement;
    private _favoriteButton: HTMLElement;
    private _unfavoriteButton: HTMLElement;

    private _audio?: HTMLAudioElement;
    private _info?: FileInfo;
    private _trackUrl: string = "";
    private _playlistUrl: string = "";
    private _playlistPos: number = 0;

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

        this._favoriteButton.onclick = () => {
            this.favorite();
        };

        this._unfavoriteButton.style.display = "none";
        this._unfavoriteButton.onclick = () => {
            this.unfavorite();
        };
    }

    private async _getInfo(url: string): Promise<FileInfo> {
        const info = await fetchJson(url);
        if (typeof info !== "object") {
            throw new Error("invalid format");
        }
        return info;
    }

    private _setStatus(info: FileInfo): void {
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

    private _onTimeUpdate(): void {
        if (this._audio === undefined || this._info === undefined) {
            return;
        }

        const currentTime = this._audio.currentTime;
        const duration = this._info.duration;
        const currentTimeStr = this._secondsToString(currentTime);
        const durationStr = this._secondsToString(duration);

        this._duration.textContent = `${currentTimeStr} / ${durationStr}`;

        if (duration > 0) {
            this._seekSlider.style.left = `${(currentTime / duration) * 100}%`;
        } else {
            this._seekSlider.style.left = "0";
        }
    }

    private _onBufferProgress(): void {
        if (this._audio === undefined || this._info === undefined) {
            return;
        }

        const ranges = this._audio.buffered;
        if (ranges.length > 0 && this._info.duration > 0) {
            const start = ranges.start(0);
            const end = ranges.end(ranges.length - 1);
            this._progressBarFill.style.left = `${(start / this._info.duration) * 100}%`;
            this._progressBarFill.style.width = `${((end - start) / this._info.duration) * 100}%`;
        } else {
            this._progressBarFill.style.left = "0";
            this._progressBarFill.style.width = "0";
        }
    }

    private async _play(url: string): Promise<void> {
        const audio = new Audio(`${url}/stream?codec=vorbis&quality=8`);
        const [, info] = await Promise.all([
            new Promise<void>((resolve) => {
                audio.oncanplay = () => {
                    resolve();
                }
            }),
            this._getInfo(`${url}/info`)
        ]);

        if (this._audio !== undefined) {
            this._audio.pause();
        }
        this._audio = audio;
        this._info = info;
        this._trackUrl = url;

        audio.onprogress = () => {
            this._onBufferProgress();
        };
        audio.ontimeupdate = () => {
            this._onTimeUpdate();
            this._onBufferProgress();
        };
        audio.onended = () => {
            this.next();
        };

        if (info.replayGainTrack < 1) {
            audio.volume = info.replayGainTrack;
        }
        audio.play();

        this._progressBarFill.style.left = "0";
        this._progressBarFill.style.width = "0";
        this._playButton.style.display = "none";
        this._pauseButton.style.display = "";

        this._setStatus(info);
    }

    public playTrack(url: string): Promise<void> {
        this._playlistUrl = "";
        return this._play(url);
    }

    public playList(url: string): Promise<void> {
        this._playlistUrl = url;
        this._playlistPos = -1;
        return this.next();
    }

    public async next(): Promise<void> {
        const item = await fetchJson(`${this._playlistUrl}?pos=${this._playlistPos + 1}`);
        this._playlistPos = item.pos;
        await this._play(item.path);
    }

    public pause(): void {
        if (this._audio === undefined) {
            return;
        }
        this._audio.pause();
        this._pauseButton.style.display = "none";
        this._playButton.style.display = "";
    }

    public unpause(): void {
        if (this._audio === undefined) {
            return;
        }
        this._audio.play();
        this._playButton.style.display = "none";
        this._pauseButton.style.display = "";
    }

    public async favorite(): Promise<void> {
        if (this._info === undefined) {
            return Promise.resolve();
        }
        await postJson(`${this._trackUrl}/favorite`);
        this._info.favorite = true;
        this._setStatus(this._info);
    }

    public async unfavorite(): Promise<void> {
        if (this._info === undefined) {
            return Promise.resolve();
        }
        await postJson(`${this._trackUrl}/unfavorite`);
        this._info.favorite = false;
        this._setStatus(this._info);
    }
}