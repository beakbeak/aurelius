interface FileInfo {
    name: string;
    duration: number;
    tags: {[key: string]: string | undefined};
}

function fetchJson(url: string): Promise<any> {
    const req = new XMLHttpRequest();
    req.open("GET", url);
    return new Promise((resolve, reject) => {
        req.onreadystatechange = () => {
            if (req.readyState !== XMLHttpRequest.DONE) {
                return;
            }
            if (req.status === 200) {
                try {
                    resolve(JSON.parse(req.responseText));
                } catch (e) {
                    reject(e);
                }
            } else {
                reject(new Error("request failed"));
            }
        }
        req.send();
    });
}

class Player {
    private _playButton: HTMLElement;
    private _pauseButton: HTMLElement;
    private _progressBarFill: HTMLElement;
    private _seekSlider: HTMLElement;
    private _statusRight: HTMLElement;
    private _duration: HTMLElement;

    private _audio?: HTMLAudioElement;
    private _info?: FileInfo;

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
        this._progressBarFill = this._getElement(container, "progress-bar-fill");
        this._seekSlider = this._getElement(container, "seek-slider");
        this._duration = this._getElement(container, "duration");

        this._playButton.onclick = () => {
            this.unpause();
        };

        this._pauseButton.style.display = "none";
        this._pauseButton.onclick = () => {
            this.pause();
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

    public async play(url: string): Promise<void> {
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

        audio.onprogress = () => {
            this._onBufferProgress();
        };
        audio.ontimeupdate = () => {
            this._onTimeUpdate();
            this._onBufferProgress();
        };

        audio.play();

        this._progressBarFill.style.left = "0";
        this._progressBarFill.style.width = "0";
        this._playButton.style.display = "none";
        this._pauseButton.style.display = "";

        this._setStatus(info);
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
}