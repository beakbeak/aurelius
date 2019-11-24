import { Player } from "../core/player.js";

export class PlayerUi {
    private readonly _playButton: HTMLElement;
    private readonly _pauseButton: HTMLElement;
    private readonly _nextButton: HTMLElement;
    private readonly _prevButton: HTMLElement;
    private readonly _progressBarEmpty: HTMLElement;
    private readonly _progressBarFill: HTMLElement;
    private readonly _seekSlider: HTMLElement;
    private readonly _statusRight: HTMLElement;
    private readonly _duration: HTMLElement;
    private readonly _favoriteButton: HTMLElement;
    private readonly _unfavoriteButton: HTMLElement;

    // [0..1]
    private _seekSliderPosition: number | undefined;

    constructor(
        public player: Player,
        containerId: string,
    ) {
        const container = document.getElementById(containerId);
        if (container === null) {
            throw new Error("invalid container");
        }

        this._statusRight = getElement(container, "status-right");
        this._playButton = getElement(container, "play-button");
        this._pauseButton = getElement(container, "pause-button");
        this._nextButton = getElement(container, "next-button");
        this._prevButton = getElement(container, "prev-button");
        this._progressBarEmpty = getElement(container, "progress-bar-empty");
        this._progressBarFill = getElement(container, "progress-bar-fill");
        this._seekSlider = getElement(container, "seek-slider");
        this._duration = getElement(container, "duration");
        this._favoriteButton = getElement(container, "favorite-button");
        this._unfavoriteButton = getElement(container, "unfavorite-button");

        this._playButton.onclick = () => {
            this.player.unpause();
        };

        this._pauseButton.style.display = "none";
        this._pauseButton.onclick = () => {
            this.player.pause();
        };

        this._nextButton.onclick = () => {
            this.player.next();
        };
        this._prevButton.onclick = () => {
            this.player.previous();
        };

        this._favoriteButton.onclick = () => {
            this.player.favorite();
        };

        this._unfavoriteButton.style.display = "none";
        this._unfavoriteButton.onclick = () => {
            this.player.unfavorite();
        };

        this._progressBarEmpty.onmousedown = (event) => {
            event.preventDefault();
            this._startSeekSliderDrag(event.clientX, event.screenX);
        };
        this._progressBarEmpty.ontouchstart = (event) => {
            event.preventDefault();
            if (event.changedTouches.length > 0) {
                const touch = event.changedTouches[0];
                this._startSeekSliderDrag(touch.clientX, touch.screenX, touch.identifier);
            }
        }
        this._updateAll();

        window.addEventListener("resize", () => {
            this._updateStatus(); // update marquee distance
        });

        this.player.addEventListener("progress", () => {
            this._updateBuffer();
        });
        this.player.addEventListener("timeupdate", () => {
            this._updateTime();
            this._updateBuffer();
        });
        this.player.addEventListener("favorite", () => {
            this._updateStatus();
        });
        this.player.addEventListener("unfavorite", () => {
            this._updateStatus();
        });

        const updateAll = () => { this._updateAll(); };

        this.player.addEventListener("play", updateAll);
        this.player.addEventListener("ended", updateAll);
        this.player.addEventListener("pause", updateAll);
        this.player.addEventListener("unpause", updateAll);
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

    private _startSeekSliderDrag(
        anchorClientX: number,
        anchorScreenX: number,
        touchId?: number,
    ): void {
        if (this.player.track === undefined) {
            return;
        }

        const rect = this._progressBarEmpty.getBoundingClientRect();
        const anchorClientXOffset = anchorClientX - rect.left;

        const getSeekSliderPosition = (screenX: number): number => {
            let clientXOffset = anchorClientXOffset + (screenX - anchorScreenX);
            if (clientXOffset < 0) {
                clientXOffset = 0;
            } else if (clientXOffset > rect.width) {
                clientXOffset = rect.width;
            }
            return clientXOffset / rect.width;
        };

        this._seekSliderPosition = getSeekSliderPosition(anchorScreenX);
        this._updateTime();

        onDrag((screenX) => {
            this._seekSliderPosition = getSeekSliderPosition(screenX);
            this._updateTime();
        },
        (screenX) => {
            this._seekSliderPosition = undefined;

            if (this.player.track !== undefined) {
                this.player.seekTo(
                    getSeekSliderPosition(screenX) * this.player.track.info.duration);
            }
        }, touchId);
    }

    private _updateStatus(): void {
        if (this.player.track === undefined) {
            this._statusRight.textContent = "";
            this._favoriteButton.style.display = "";
            this._unfavoriteButton.style.display = "none";
            this._favoriteButton.classList.add("inactive");
            return;
        }

        const info = this.player.track.info;
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
        const track = this.player.track;

        if (track === undefined) {
            this._duration.textContent = "";
            this._seekSlider.style.left = "0";
            this._seekSlider.classList.add("inactive");
            this._progressBarEmpty.classList.add("inactive");
            return;
        }
        this._seekSlider.classList.remove("inactive");
        this._progressBarEmpty.classList.remove("inactive");

        const duration = track.info.duration;
        const currentTime = this._seekSliderPosition !== undefined
            ? this._seekSliderPosition * duration : track.currentTime();
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
        const track = this.player.track;

        if (track === undefined) {
            this._progressBarFill.style.left = "0";
            this._progressBarFill.style.width = "0";
            return;
        }

        const ranges = track.audio.buffered;
        if (ranges.length > 0 && track.info.duration > 0) {
            const start = track.startTime + ranges.start(0);
            const end = track.startTime + ranges.end(ranges.length - 1);
            this._progressBarFill.style.left =
                `${Math.max(0, Math.min(100, (start / track.info.duration) * 100))}%`;
            this._progressBarFill.style.width =
                `${Math.max(0, Math.min(100, ((end - start) / track.info.duration) * 100))}%`;
        } else {
            this._progressBarFill.style.left = "0";
            this._progressBarFill.style.width = "0";
        }
    }

    private _updateButtons(): void {
        const track = this.player.track;

        if (track === undefined || track.audio.paused) {
            this._playButton.style.display = "";
            this._pauseButton.style.display = "none";
            if (track === undefined) {
                this._playButton.classList.add("inactive");
            } else {
                this._playButton.classList.remove("inactive");
            }
        } else {
            this._playButton.style.display = "none";
            this._pauseButton.style.display = "";
        }

        if (this.player.hasNext()) {
            this._nextButton.classList.remove("inactive");
        } else {
            this._nextButton.classList.add("inactive");
        }

        if (this.player.hasPrevious()) {
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
}

function getElement(
    container: HTMLElement,
    id: string,
): HTMLElement {
    const element = container.querySelector(`#${id}`);
    if (element === null) {
        throw new Error(`missing ${id}`);
    }
    return element as HTMLElement;
}

function onDrag(
    onMove: (x: number, y: number) => void,
    onStop: (x: number, y: number) => void,
    touchId?: number,
): void {
    if (touchId !== undefined) {
        const onTouchMove = (e: TouchEvent): void => {
            for (const touch of e.changedTouches) {
                if (touch.identifier === touchId) {
                    onMove(touch.screenX, touch.screenY);
                    break;
                }
            }
        };
        const onTouchEnd = (e: TouchEvent): void => {
            for (const touch of e.changedTouches) {
                if (touch.identifier === touchId) {
                    onStop(touch.screenX, touch.screenY);

                    document.removeEventListener("touchmove", onTouchMove);
                    document.removeEventListener("touchend", onTouchEnd);
                    document.removeEventListener("touchcancel", onTouchEnd);
                    break;
                }
            }
        };

        document.addEventListener("touchmove", onTouchMove);
        document.addEventListener("touchend", onTouchEnd);
        document.addEventListener("touchcancel", onTouchEnd);
    } else {
        const onMouseMove = (e: MouseEvent): void => {
            onMove(e.screenX, e.screenY);
        };
        const onMouseUp = (e: MouseEvent): void => {
            onStop(e.screenX, e.screenY);

            document.removeEventListener("mousemove", onMouseMove);
            document.removeEventListener("mouseup", onMouseUp);
        };

        document.addEventListener("mousemove", onMouseMove);
        document.addEventListener("mouseup", onMouseUp);
    }
}