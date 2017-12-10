class Player {
    //private _container: HTMLElement;
    private _playButton: HTMLElement;
    private _pauseButton: HTMLElement;
    private _audio: HTMLAudioElement | undefined;

    constructor(containerId: string) {
        const container = document.getElementById(containerId);
        if (container === null) {
            throw new Error("invalid container");
        }
        //this._container = container;

        const playButton = document.querySelector("#play-button");
        if (playButton === null) {
            throw new Error("missing play-button");
        }
        this._playButton = playButton as HTMLElement;
        this._playButton.onclick = () => {
            this.unpause();
        };

        const pauseButton = document.querySelector("#pause-button");
        if (pauseButton === null) {
            throw new Error("missing pause-button");
        }
        this._pauseButton = pauseButton as HTMLElement;
        this._pauseButton.style.display = "none";
        this._pauseButton.onclick = () => {
            this.pause();
        };
    }

    public play(url: string): void {
        if (this._audio !== undefined) {
            this._audio.pause();
        }
        this._audio = new Audio(url);
        this._audio.autoplay = true;

        this._playButton.style.display = "none";
        this._pauseButton.style.display = "";
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