interface FileInfo {
    name: string;
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
    private _audio: HTMLAudioElement | undefined;
    private _statusRight: HTMLElement;

    constructor(containerId: string) {
        const container = document.getElementById(containerId);
        if (container === null) {
            throw new Error("invalid container");
        }

        const statusRight = container.querySelector("#status-right");
        if (statusRight === null) {
            throw new Error("missing status-right");
        }
        this._statusRight = statusRight as HTMLElement;

        const playButton = container.querySelector("#play-button");
        if (playButton === null) {
            throw new Error("missing play-button");
        }
        this._playButton = playButton as HTMLElement;
        this._playButton.onclick = () => {
            this.unpause();
        };

        const pauseButton = container.querySelector("#pause-button");
        if (pauseButton === null) {
            throw new Error("missing pause-button");
        }
        this._pauseButton = pauseButton as HTMLElement;
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

        audio.play();

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