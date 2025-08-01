import { Player } from "../core/player";
import { loadDir } from "./dir";
import { onDrag, toggleClass } from "./dom";
import { Class } from "./class";
import { showModalDialog } from "./modal";
import { AttachedImageInfo, StreamCodec } from "../core/track";
import { getSettings } from "./settings";

const defaultTrackImageUrl = "/static/img/aurelius.svgz";
const maxImageSize = 300 * 1024;

let player: Player;

let aboutButton: HTMLElement;
let aboutDialog: HTMLElement;
let durationText: HTMLElement;
let favoriteButton: HTMLElement;
let marquee: HTMLAnchorElement;
let nextButton: HTMLElement;
let pauseButton: HTMLElement;
let playButton: HTMLElement;
let prevButton: HTMLElement;
let progressBarEmpty: HTMLElement;
let progressBarFill: HTMLElement;
let progressControls: HTMLElement;
let seekSlider: HTMLElement;
let trackImage: HTMLImageElement;
let unfavoriteButton: HTMLElement;

// [0..1]
let _seekSliderPosition: number | undefined;
let _notificationData: { title: string; body: string; icon?: string } | undefined;

export function setupPlayerUi(inPlayer: Player) {
    player = inPlayer;

    aboutButton = document.getElementById("about-button")!;
    aboutDialog = document.getElementById("about-dialog")!;
    durationText = document.getElementById("duration")!;
    favoriteButton = document.getElementById("favorite-button")!;
    marquee = document.getElementById("marquee") as HTMLAnchorElement;
    nextButton = document.getElementById("next-button")!;
    pauseButton = document.getElementById("pause-button")!;
    playButton = document.getElementById("play-button")!;
    prevButton = document.getElementById("prev-button")!;
    progressBarEmpty = document.getElementById("progress-bar-empty")!;
    progressBarFill = document.getElementById("progress-bar-fill")!;
    progressControls = document.getElementById("progress-controls")!;
    seekSlider = document.getElementById("seek-slider")!;
    trackImage = document.getElementById("track-image") as HTMLImageElement;
    unfavoriteButton = document.getElementById("unfavorite-button")!;

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

    trackImage.onclick = openTrackImageInNewTab;

    marquee.onclick = (e) => {
        e.preventDefault();
        const dirUrl = marquee.getAttribute("data-url");
        if (dirUrl) {
            loadDir(dirUrl);
        }
    };

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
    };
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
        updateButtons();
        updateStatus(); // update favorite indicator in MediaSession
    });
    player.addEventListener("unfavorite", () => {
        updateButtons();
        updateStatus(); // update favorite indicator in MediaSession
    });

    player.addEventListener("play", updateAll);
    player.addEventListener("ended", updateAll);
    player.addEventListener("pause", updateAll);
    player.addEventListener("unpause", updateAll);

    player.addEventListener("autoNext", showDesktopNotification);

    navigator.mediaSession?.setActionHandler("previoustrack", () => {
        player.previous();
    });
    navigator.mediaSession?.setActionHandler("nexttrack", () => {
        player.next();
    });
    navigator.mediaSession?.setActionHandler("seekto", (args) => {
        if (typeof args.seekTime === "number") {
            player.seekTo(args.seekTime);
        }
    });
}

export function openTrackImageInNewTab(): void {
    const trackImage = document.getElementById("track-image") as HTMLImageElement;
    if (trackImage.src && !trackImage.src.endsWith(defaultTrackImageUrl)) {
        window.open(trackImage.src, "_blank");
    }
}

function showDesktopNotification(data = _notificationData): void {
    if (
        !(
            data &&
            "Notification" in window &&
            Notification.permission === "granted" &&
            document.visibilityState === "hidden" &&
            getSettings().desktopNotifications
        )
    ) {
        return;
    }

    const notification = new Notification(data.title, {
        body: data.body,
        icon: data.icon,
        silent: true,
    });

    notification.onclick = () => {
        window.focus();
        notification.close();
    };
}

function setMarquee(text: string, url: string): void {
    marquee.textContent = text;
    marquee.href = `/media/tree/?path=${encodeURIComponent(url)}`;
    marquee.setAttribute("data-url", url);

    if (marquee.clientWidth >= marquee.scrollWidth) {
        marquee.style.animation = "";
        return;
    }

    const scrollLength = marquee.scrollWidth - marquee.clientWidth;
    const scrollTime = scrollLength / 50; /* px/second */
    const waitTime = 2; /* seconds */
    const totalTime = 2 * (scrollTime + waitTime);
    const scrollPercent = 100 * (scrollTime / totalTime);
    const waitPercent = 100 * (waitTime / totalTime);

    const style = document.createElement("style");
    style.textContent = `@keyframes marquee {
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

function startSeekSliderDrag(anchorClientX: number, anchorScreenX: number, touchId?: number): void {
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

    onDrag(
        (screenX) => {
            _seekSliderPosition = getSeekSliderPosition(screenX);
            updateTime();
        },
        (screenX) => {
            _seekSliderPosition = undefined;

            if (player.track !== undefined) {
                player.seekTo(getSeekSliderPosition(screenX) * player.track.info.duration);
            }
        },
        touchId,
    );
}

function filterTrackImages(
    images: AttachedImageInfo[],
): (AttachedImageInfo & { originalIndex: number })[] {
    const imagesWithIndex = images.map((img, index) => ({ ...img, originalIndex: index }));

    switch (player.streamConfig.codec) {
        case StreamCodec.Flac:
        case StreamCodec.Wav:
            // When a lossless codec is chosen, assume that bandwidth is less of a concern,
            // so do not filter out large images.
            return imagesWithIndex;
        default:
            return imagesWithIndex.filter((img) => {
                if (img.size > maxImageSize) {
                    console.debug("skipping oversized image", {
                        ...img,
                        maxSize: maxImageSize,
                    });
                    return false;
                }
                return true;
            });
    }
}

function updateStatus(): void {
    const track = player.track;
    if (track === undefined) {
        marquee.textContent = "";
        trackImage.src = defaultTrackImageUrl;
        trackImage.style.cursor = "default";
        _notificationData = undefined;
        return;
    }

    const info = track.info;
    let text = "";

    const artist = info.tags["artist"] ?? info.tags["composer"] ?? "";
    const title = info.tags["title"] ?? info.name;
    const favoriteIcon = info.favorite ? "♥︎" : "♡";

    let album = "";
    if (info.tags["album"] !== undefined) {
        let trackName = "";
        if (info.tags["track"] !== undefined) {
            trackName = ` #${info.tags["track"]}`;
        }
        album = `${info.tags["album"]}${trackName}`;
        text = `${text} [${info.tags["album"]}${trackName}]`;
    }

    setMarquee(`${artist ? `${artist} - ` : ""}${title}${album ? ` [${album}]` : ""}`, info.dir);

    // Store notification data for later use
    _notificationData = {
        title: `${favoriteIcon} ${title}`,
        body: `${artist}${album ? ` / ${album}` : ""}`,
        icon: undefined, // Will be set below
    };

    const filteredImages = filterTrackImages(info.attachedImages);
    let newTrackImageUrl = "";
    if (filteredImages.length > 0) {
        newTrackImageUrl = `${track.url}/images/${filteredImages[0].originalIndex}`;
        trackImage.style.cursor = "pointer";
        // Set notification icon
        if (_notificationData) {
            _notificationData.icon = newTrackImageUrl;
        }
    } else {
        newTrackImageUrl = defaultTrackImageUrl;
        trackImage.style.cursor = "default";
    }
    if (!trackImage.src.endsWith(newTrackImageUrl)) {
        trackImage.src = newTrackImageUrl;
    }

    if (navigator.mediaSession !== undefined) {
        const artwork: MediaImage[] = [];
        filteredImages.forEach((imageInfo) => {
            artwork.push({
                src: `${track.url}/images/${imageInfo.originalIndex}`,
                type: `${imageInfo.mimeType}`,
                sizes: "",
            });
        });

        navigator.mediaSession.metadata = new MediaMetadata({
            artist,
            title: `${favoriteIcon} ${title}`,
            album,
            artwork,
        });
        if (navigator.mediaSession.setPositionState !== undefined) {
            navigator.mediaSession.setPositionState({
                duration: info.duration,
                position: Math.min(track.currentTime(), info.duration),
            });
        }
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
        return;
    }

    const duration = track.info.duration;
    const currentTime =
        _seekSliderPosition !== undefined ? _seekSliderPosition * duration : track.currentTime();
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

    if (track === undefined) {
        progressControls.classList.add(Class.Controls_Disabled);
        playButton.classList.add(Class.ControlsButton_Disabled);
        favoriteButton.classList.add(Class.ControlsButton_Disabled);

        playButton.classList.remove(Class.Hidden);
        pauseButton.classList.add(Class.Hidden);

        favoriteButton.classList.remove(Class.Hidden);
        unfavoriteButton.classList.add(Class.Hidden);
    } else {
        progressControls.classList.remove(Class.Controls_Disabled);
        playButton.classList.remove(Class.ControlsButton_Disabled);
        favoriteButton.classList.remove(Class.ControlsButton_Disabled);

        if (track.isPaused()) {
            playButton.classList.remove(Class.Hidden);
            pauseButton.classList.add(Class.Hidden);
        } else {
            playButton.classList.add(Class.Hidden);
            pauseButton.classList.remove(Class.Hidden);
        }

        if (track.info.favorite) {
            favoriteButton.classList.add(Class.Hidden);
            unfavoriteButton.classList.remove(Class.Hidden);
        } else {
            favoriteButton.classList.remove(Class.Hidden);
            unfavoriteButton.classList.add(Class.Hidden);
        }
    }

    toggleClass(nextButton, Class.ControlsButton_Disabled, !player.hasNext());
    toggleClass(prevButton, Class.ControlsButton_Disabled, !player.hasPrevious());
}

function updateAll(): void {
    updateTime();
    updateBuffer();
    updateStatus();
    updateButtons();
}
