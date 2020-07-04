import { Player } from "../core/player";
import { stripLastPathElement } from "../core/url";
import { loadDir } from "./dir";
import { onDrag } from "./dom";
import { Class } from "./class";
import { showModalDialog } from "./modal";

let player: Player;

let playButton: HTMLElement;
let pauseButton: HTMLElement;
let nextButton: HTMLElement;
let prevButton: HTMLElement;
let progressBarEmpty: HTMLElement;
let progressBarFill: HTMLElement;
let seekSlider: HTMLElement;
let marquee: HTMLAnchorElement;
let durationText: HTMLElement;
let favoriteButton: HTMLElement;
let unfavoriteButton: HTMLElement;
let aboutButton: HTMLElement;
let aboutDialog: HTMLElement;

// [0..1]
let _seekSliderPosition: number | undefined;

export default function setupPlayerUi(
    inPlayer: Player,
) {
    player = inPlayer;

    marquee = document.getElementById("marquee") as HTMLAnchorElement;
    playButton = document.getElementById("play-button")!;
    pauseButton = document.getElementById("pause-button")!;
    nextButton = document.getElementById("next-button")!;
    prevButton = document.getElementById("prev-button")!;
    progressBarEmpty = document.getElementById("progress-bar-empty")!;
    progressBarFill = document.getElementById("progress-bar-fill")!;
    seekSlider = document.getElementById("seek-slider")!;
    durationText = document.getElementById("duration")!;
    favoriteButton = document.getElementById("favorite-button")!;
    unfavoriteButton = document.getElementById("unfavorite-button")!;
    aboutButton = document.getElementById("about-button")!;
    aboutDialog = document.getElementById("about-dialog")!;

    playButton.onclick = () => {
        player.unpause();
    };

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
    unfavoriteButton.onclick = () => {
        player.unfavorite();
    };

    aboutButton.onclick = () => {
        showModalDialog(aboutDialog);
    };

    marquee.onclick = (e) => {
        e.preventDefault();
        if (marquee.href) {
            loadDir(marquee.href);
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

function setMarquee(
    text: string,
    url: string,
): void {
    marquee.textContent = text;
    marquee.href = url;

    if (marquee.clientWidth >= marquee.scrollWidth) {
        marquee.style.animation = "";
        return;
    }

    const scrollLength = marquee.scrollWidth - marquee.clientWidth;
    const scrollTime = scrollLength / 50 /* px/second */;
    const waitTime = 2 /* seconds */;
    const totalTime = 2 * (scrollTime + waitTime);
    const scrollPercent = 100 * (scrollTime / totalTime);
    const waitPercent = 100 * (waitTime / totalTime);

    const style = document.createElement("style");
    style.textContent =
        `@keyframes marquee {
            ${scrollPercent}% {
                transform: translateX(-${scrollLength}px);
            }
            ${scrollPercent + waitPercent}% {
                transform: translateX(-${scrollLength}px);
            }
            ${2 * scrollPercent + waitPercent}% {
                transform: translateX(0px);
            }
        }`;
    marquee.appendChild(style);
    marquee.style.animation = `marquee ${totalTime}s infinite linear`;
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

    onDrag((screenX) => {
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
    const track = player.track;
    if (track === undefined) {
        marquee.textContent = "";
        favoriteButton.classList.remove(Class.Hidden);
        unfavoriteButton.classList.add(Class.Hidden);
        favoriteButton.classList.add(Class.Inactive);
        return;
    }

    const info = track.info;
    let text = "";

    if (info.tags["artist"] !== undefined) {
        text = `${text}${info.tags["artist"]} - `;
    } else if (info.tags["composer"] !== undefined) {
        text = `${text}${info.tags["composer"]} - `;
    }

    if (info.tags["title"] !== undefined) {
        text = `${text}${info.tags["title"]}`;
    } else {
        text = `${text}${info.name}`
    }

    if (info.tags["album"] !== undefined) {
        let trackName = "";
        if (info.tags["track"] !== undefined) {
            trackName = ` #${info.tags["track"]}`;
        }
        text = `${text} [${info.tags["album"]}${trackName}]`;
    }

    setMarquee(text, `${stripLastPathElement(track.url)}/`);

    if (info.favorite) {
        favoriteButton.classList.add(Class.Hidden);
        unfavoriteButton.classList.remove(Class.Hidden);
        unfavoriteButton.classList.remove(Class.Inactive);
    } else {
        favoriteButton.classList.remove(Class.Hidden);
        unfavoriteButton.classList.add(Class.Hidden);
        favoriteButton.classList.remove(Class.Inactive);
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
        seekSlider.classList.add(Class.Inactive);
        progressBarEmpty.classList.add(Class.Inactive);
        return;
    }
    seekSlider.classList.remove(Class.Inactive);
    progressBarEmpty.classList.remove(Class.Inactive);

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

    const ranges = track.buffered();

    if (ranges.length > 0 && track.info.duration > 0) {
        const startTime = track.startTime + ranges.start(0);
        const endTime = track.startTime + ranges.end(ranges.length - 1);

        const left = startTime / track.info.duration;
        const width = (endTime - startTime) / track.info.duration;

        const leftPercent = Math.max(0, Math.min(100, left * 100));
        const widthPercent = Math.max(0, Math.min(100 - leftPercent, width * 100));

        progressBarFill.style.left = `${leftPercent}%`;
        progressBarFill.style.width = `${widthPercent}%`;
    } else {
        progressBarFill.style.left = "0";
        progressBarFill.style.width = "0";
    }
}

function updateButtons(): void {
    const track = player.track;

    if (track === undefined || track.isPaused()) {
        playButton.classList.remove(Class.Hidden);
        pauseButton.classList.add(Class.Hidden);
        if (track === undefined) {
            playButton.classList.add(Class.Inactive);
        } else {
            playButton.classList.remove(Class.Inactive);
        }
    } else {
        playButton.classList.add(Class.Hidden);
        pauseButton.classList.remove(Class.Hidden);
    }

    if (player.hasNext()) {
        nextButton.classList.remove(Class.Inactive);
    } else {
        nextButton.classList.add(Class.Inactive);
    }

    if (player.hasPrevious()) {
        prevButton.classList.remove(Class.Inactive);
    } else {
        prevButton.classList.add(Class.Inactive);
    }
}

function updateAll(): void {
    updateTime();
    updateBuffer();
    updateStatus();
    updateButtons();
}
