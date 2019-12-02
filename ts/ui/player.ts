import { Player } from "../core/player.js";
import { loadDir } from "./dir.js";
import * as util from "./util.js";
import * as coreUtil from "../core/util.js";

let player: Player;

let playButton: HTMLElement;
let pauseButton: HTMLElement;
let nextButton: HTMLElement;
let prevButton: HTMLElement;
let progressBarEmpty: HTMLElement;
let progressBarFill: HTMLElement;
let seekSlider: HTMLElement;
let statusRight: HTMLElement;
let durationText: HTMLElement;
let favoriteButton: HTMLElement;
let unfavoriteButton: HTMLElement;

// [0..1]
let _seekSliderPosition: number | undefined;

export default function setupPlayerUi(
    inPlayer: Player,
    containerId: string,
) {
    const container = document.getElementById(containerId);
    if (container === null) {
        throw new Error("invalid container");
    }

    player = inPlayer;

    statusRight = util.getElement(container, "status-right");
    playButton = util.getElement(container, "play-button");
    pauseButton = util.getElement(container, "pause-button");
    nextButton = util.getElement(container, "next-button");
    prevButton = util.getElement(container, "prev-button");
    progressBarEmpty = util.getElement(container, "progress-bar-empty");
    progressBarFill = util.getElement(container, "progress-bar-fill");
    seekSlider = util.getElement(container, "seek-slider");
    durationText = util.getElement(container, "duration");
    favoriteButton = util.getElement(container, "favorite-button");
    unfavoriteButton = util.getElement(container, "unfavorite-button");

    playButton.onclick = () => {
        player.unpause();
    };

    pauseButton.style.display = "none";
    pauseButton.onclick = () => {
        player.pause();
    };

    nextButton.onclick = () => {
        player.next();
    };
    prevButton.onclick = () => {
        player.previous();
    };

    favoriteButton.onclick = () => {
        player.favorite();
    };

    unfavoriteButton.style.display = "none";
    unfavoriteButton.onclick = () => {
        player.unfavorite();
    };

    statusRight.onclick = () => {
        if (player.track !== undefined) {
            loadDir(`${coreUtil.stripLastPathElement(player.track.url)}/`);
        }
    }

    progressBarEmpty.onmousedown = (event) => {
        event.preventDefault();
        startSeekSliderDrag(event.clientX, event.screenX);
    };
    progressBarEmpty.ontouchstart = (event) => {
        event.preventDefault();
        if (event.changedTouches.length > 0) {
            const touch = event.changedTouches[0];
            startSeekSliderDrag(touch.clientX, touch.screenX, touch.identifier);
        }
    }
    updateAll();

    window.addEventListener("resize", () => {
        updateStatus(); // update marquee distance
    });

    player.addEventListener("progress", () => {
        updateBuffer();
    });
    player.addEventListener("timeupdate", () => {
        updateTime();
        updateBuffer();
    });
    player.addEventListener("favorite", () => {
        updateStatus();
    });
    player.addEventListener("unfavorite", () => {
        updateStatus();
    });

    player.addEventListener("play", updateAll);
    player.addEventListener("ended", updateAll);
    player.addEventListener("pause", updateAll);
    player.addEventListener("unpause", updateAll);
}

function setStatusText(text: string): void {
    const element = statusRight;

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

function startSeekSliderDrag(
    anchorClientX: number,
    anchorScreenX: number,
    touchId?: number,
): void {
    if (player.track === undefined) {
        return;
    }

    const rect = progressBarEmpty.getBoundingClientRect();
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

    _seekSliderPosition = getSeekSliderPosition(anchorScreenX);
    updateTime();

    util.onDrag((screenX) => {
        _seekSliderPosition = getSeekSliderPosition(screenX);
        updateTime();
    },
    (screenX) => {
        _seekSliderPosition = undefined;

        if (player.track !== undefined) {
            player.seekTo(
                getSeekSliderPosition(screenX) * player.track.info.duration);
        }
    }, touchId);
}

function updateStatus(): void {
    if (player.track === undefined) {
        statusRight.textContent = "";
        favoriteButton.style.display = "";
        unfavoriteButton.style.display = "none";
        favoriteButton.classList.add("inactive");
        return;
    }

    const info = player.track.info;
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

    setStatusText(text);

    if (info.favorite) {
        favoriteButton.style.display = "none";
        unfavoriteButton.style.display = "";
        unfavoriteButton.classList.remove("inactive");
    } else {
        favoriteButton.style.display = "";
        unfavoriteButton.style.display = "none";
        favoriteButton.classList.remove("inactive");
    }
}

function secondsToString(totalSeconds: number): string {
    const minutes = (totalSeconds / 60) | 0;
    const seconds = (totalSeconds - minutes * 60) | 0;
    if (seconds < 10) {
        return `${minutes}:0${seconds}`;
    } else {
        return `${minutes}:${seconds}`;
    }
}

function updateTime(): void {
    const track = player.track;

    if (track === undefined) {
        durationText.textContent = "";
        seekSlider.style.left = "0";
        seekSlider.classList.add("inactive");
        progressBarEmpty.classList.add("inactive");
        return;
    }
    seekSlider.classList.remove("inactive");
    progressBarEmpty.classList.remove("inactive");

    const duration = track.info.duration;
    const currentTime = _seekSliderPosition !== undefined
        ? _seekSliderPosition * duration : track.currentTime();
    const currentTimeStr = secondsToString(currentTime);
    const durationStr = secondsToString(duration);

    durationText.textContent = `${currentTimeStr} / ${durationStr}`;

    if (duration > 0) {
        seekSlider.style.left = `${(currentTime / duration) * 100}%`;
    } else {
        seekSlider.style.left = "0";
    }
}

function updateBuffer(): void {
    const track = player.track;

    if (track === undefined) {
        progressBarFill.style.left = "0";
        progressBarFill.style.width = "0";
        return;
    }

    const ranges = track.audio.buffered;
    if (ranges.length > 0 && track.info.duration > 0) {
        const start = track.startTime + ranges.start(0);
        const end = track.startTime + ranges.end(ranges.length - 1);
        progressBarFill.style.left =
            `${Math.max(0, Math.min(100, (start / track.info.duration) * 100))}%`;
        progressBarFill.style.width =
            `${Math.max(0, Math.min(100, ((end - start) / track.info.duration) * 100))}%`;
    } else {
        progressBarFill.style.left = "0";
        progressBarFill.style.width = "0";
    }
}

function updateButtons(): void {
    const track = player.track;

    if (track === undefined || track.audio.paused) {
        playButton.style.display = "";
        pauseButton.style.display = "none";
        if (track === undefined) {
            playButton.classList.add("inactive");
        } else {
            playButton.classList.remove("inactive");
        }
    } else {
        playButton.style.display = "none";
        pauseButton.style.display = "";
    }

    if (player.hasNext()) {
        nextButton.classList.remove("inactive");
    } else {
        nextButton.classList.add("inactive");
    }

    if (player.hasPrevious()) {
        prevButton.classList.remove("inactive");
    } else {
        prevButton.classList.add("inactive");
    }
}

function updateAll(): void {
    updateTime();
    updateBuffer();
    updateStatus();
    updateButtons();
}
