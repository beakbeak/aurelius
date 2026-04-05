<script lang="ts">
    import type { PlayerState } from "./PlayerState.svelte";
    import Marquee from "./Marquee.svelte";
    import SeekSlider from "./SeekSlider.svelte";
    import { formatDuration } from "./format";
    import { getSettings } from "./settings";
    import { onMount } from "svelte";
    import type { Track } from "../core/track";

    const defaultTrackImageUrl = "/static/img/aurelius.svgz";

    let {
        playerState,
        onAbout,
        onNavigateToDir,
        onShowImageGallery,
    }: {
        playerState: PlayerState;
        onAbout: () => void;
        onNavigateToDir: (url: string) => void;
        onShowImageGallery: () => void;
    } = $props();

    const player = $derived(playerState.player);

    let trackImageUrl = $derived.by(() => {
        const info = playerState.track?.info;
        if (!info) {
            return defaultTrackImageUrl;
        }
        return info.attachedImages.length > 0 ? info.attachedImages[0].url : defaultTrackImageUrl;
    });
    let trackImageCursor = $derived(trackImageUrl !== defaultTrackImageUrl ? "pointer" : "default");

    function openImageGallery(): void {
        if (trackImageUrl !== defaultTrackImageUrl) {
            onShowImageGallery();
        }
    }

    let marqueeText = $derived.by(() => {
        const info = playerState.track?.info;
        if (!info) {
            return "";
        }
        const artist = info.tags["artist"] ?? info.tags["composer"] ?? "";
        const title = info.tags["title"] ?? info.name;
        let album = "";
        if (info.tags["album"] !== undefined) {
            let trackName = "";
            if (info.tags["track"] !== undefined) {
                trackName = ` #${info.tags["track"]}`;
            }
            album = `${info.tags["album"]}${trackName}`;
        }
        return `${artist ? `${artist} - ` : ""}${title}${album ? ` [${album}]` : ""}`;
    });
    let marqueeUrl = $derived(playerState.track?.info.dir ?? "");

    const seekDisabled = $derived(!playerState.track);

    const seekPosition = $derived(
        playerState.duration > 0 ? playerState.currentTime / playerState.duration : 0,
    );

    const seekBufferLeft = $derived.by(() => {
        playerState.updateOnBufferProgress();
        const track = playerState.track;
        if (!track) {
            return 0;
        }
        const ranges = track.buffered();
        if (ranges.length > 0 && track.info.duration > 0) {
            const startTime = track.startTime + ranges.start(0);
            return Math.max(0, Math.min(1, startTime / track.info.duration));
        }
        return 0;
    });

    const seekBufferWidth = $derived.by(() => {
        playerState.updateOnBufferProgress();
        const track = playerState.track;
        if (!track) {
            return 0;
        }
        const ranges = track.buffered();
        if (ranges.length > 0 && track.info.duration > 0) {
            const startTime = track.startTime + ranges.start(0);
            const endTime = track.startTime + ranges.end(ranges.length - 1);
            const left = startTime / track.info.duration;
            const width = (endTime - startTime) / track.info.duration;
            const leftClamped = Math.max(0, Math.min(1, left));
            return Math.max(0, Math.min(1 - leftClamped, width));
        }
        return 0;
    });

    const seekKeyboardStep = $derived(playerState.duration > 0 ? 5 / playerState.duration : 0);

    let seekDragValue = $state<number | undefined>();

    async function onseek(position: number): Promise<void> {
        if (playerState.track !== undefined) {
            await player.seekTo(position * playerState.track.info.duration);
        }
    }

    let durationText = $derived.by(() => {
        const track = playerState.track;
        if (!track) {
            return "";
        }
        const currentTime =
            seekDragValue !== undefined
                ? seekDragValue * playerState.duration
                : playerState.currentTime;
        const currentTimeStr = formatDuration(currentTime);
        const durationStr = formatDuration(playerState.duration);
        return `${currentTimeStr} / ${durationStr}`;
    });

    let notificationData: { title: string; body: string; icon?: string } | undefined;
    function showDesktopNotification(data?: { title: string; body: string; icon?: string }): void {
        const d = data ?? notificationData;
        if (
            !(
                d &&
                "Notification" in window &&
                Notification.permission === "granted" &&
                !document.hasFocus() &&
                getSettings().desktopNotifications
            )
        ) {
            return;
        }
        const notification = new Notification(d.title, {
            body: d.body,
            icon: d.icon,
            silent: true,
        });
        notification.onclick = () => {
            window.focus();
            notification.close();
        };
    }

    // Update MediaSession and notification data when track info changes.
    // Subscribe to event directly so that MediaSession is updated synchronously after previous
    // track ends and current track starts playing.
    onMount(() => {
        const playHandler = (track: Track) => {
            const info = track.info;
            const artist = info.tags["artist"] ?? info.tags["composer"] ?? "";
            const title = info.tags["title"] ?? info.name;
            const favoriteIcon = playerState.favorite ? "\u2665\uFE0E" : "\u2661";

            let album = "";
            if (info.tags["album"] !== undefined) {
                let trackName = "";
                if (info.tags["track"] !== undefined) {
                    trackName = ` #${info.tags["track"]}`;
                }
                album = `${info.tags["album"]}${trackName}`;
            }

            notificationData = {
                title: `${favoriteIcon} ${title}`,
                body: `${artist}${album ? ` / ${album}` : ""}`,
                icon: info.attachedImages.length > 0 ? info.attachedImages[0].url : undefined,
            };

            if (navigator.mediaSession !== undefined) {
                const artwork: MediaImage[] = [];
                info.attachedImages.forEach((imageInfo) => {
                    artwork.push({
                        src: imageInfo.url,
                        type: imageInfo.mimeType,
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
        };
        player.addEventListener("play", playHandler);
        return () => {
            player.removeEventListener("play", playHandler);
        };
    });

    // Show notification when track advances automatically.
    onMount(() => {
        const handler = () => {
            showDesktopNotification();
        };
        player.addEventListener("autoNext", handler);
        return () => {
            player.removeEventListener("autoNext", handler);
        };
    });

    // Set up MediaSession actions.
    onMount(() => {
        navigator.mediaSession?.setActionHandler("previoustrack", async () => {
            await player.previous();
            if (getSettings().mediaSessionNotifications) {
                showDesktopNotification();
            }
        });
        navigator.mediaSession?.setActionHandler("nexttrack", async () => {
            await player.next();
            if (getSettings().mediaSessionNotifications) {
                showDesktopNotification();
            }
        });
        navigator.mediaSession?.setActionHandler("seekto", (args) => {
            if (typeof args.seekTime === "number") {
                player.seekTo(args.seekTime);
            }
        });

        return () => {
            navigator.mediaSession?.setActionHandler("previoustrack", null);
            navigator.mediaSession?.setActionHandler("nexttrack", null);
            navigator.mediaSession?.setActionHandler("seekto", null);
        };
    });
</script>

<nav class="controls">
    <button
        class="track-image-container"
        type="button"
        style:cursor={trackImageCursor}
        aria-label="Open image gallery"
        onclick={openImageGallery}
    >
        <img class="track-image" src={trackImageUrl} alt="cover art" />
    </button>
    <div class="everything-else">
        <div class="marquee-spacer">
            <div class="marquee-container">
                <Marquee contentKey={marqueeText}>
                    <a
                        class="controls__link marquee-text"
                        href={marqueeUrl
                            ? `/media/tree/?path=${encodeURIComponent(marqueeUrl)}`
                            : "#"}
                        title="Jump to directory containing this track"
                        onclick={(e: MouseEvent) => {
                            e.preventDefault();
                            if (marqueeUrl) onNavigateToDir(marqueeUrl);
                        }}
                    >
                        {marqueeText}
                    </a>
                </Marquee>
            </div>
        </div>
        <SeekSlider
            value={seekPosition}
            disabled={seekDisabled}
            bufferLeft={seekBufferLeft}
            bufferWidth={seekBufferWidth}
            bind:seekValue={seekDragValue}
            {onseek}
            keyboardStep={seekKeyboardStep}
        />
        <div class="controls__group controls__group--shift-up">
            <button
                class="btn btn-ghost btn-xl btn-square mx-1.5 btn-primary not-disabled:text-primary-content"
                disabled={!playerState.hasPrevious}
                type="button"
                title="Previous track"
                onclick={() => player.previous()}
            >
                <i class="material-icons text-5xl!">skip_previous</i>
            </button>
            {#if !playerState.track || playerState.paused}
                <button
                    class="btn btn-ghost btn-xl btn-square mx-1.5 btn-primary not-disabled:text-primary-content"
                    disabled={!playerState.track}
                    type="button"
                    title="Play"
                    onclick={() => player.unpause()}
                >
                    <i class="material-icons text-5xl!">play_arrow</i>
                </button>
            {:else}
                <button
                    class="btn btn-ghost btn-xl btn-square mx-1.5 btn-primary not-disabled:text-primary-content"
                    type="button"
                    title="Pause"
                    onclick={() => player.pause()}
                >
                    <i class="material-icons text-5xl!">pause</i>
                </button>
            {/if}
            <button
                class="btn btn-ghost btn-xl btn-square mx-1.5 btn-primary not-disabled:text-primary-content"
                disabled={!playerState.hasNext}
                type="button"
                title="Next track"
                onclick={() => player.next()}
            >
                <i class="material-icons text-5xl!">skip_next</i>
            </button>
            {#if !playerState.favorite}
                <button
                    class="btn btn-ghost btn-xl btn-square mx-1.5 btn-primary not-disabled:text-primary-content"
                    disabled={!playerState.track}
                    type="button"
                    title="Add to favorites"
                    onclick={() => player.favorite()}
                >
                    <i class="material-icons text-4xl!">favorite_border</i>
                </button>
            {:else}
                <button
                    class="btn btn-ghost btn-xl btn-square mx-1.5 btn-primary not-disabled:text-primary-content"
                    type="button"
                    title="Remove from favorites"
                    onclick={() => player.unfavorite()}
                >
                    <i class="material-icons text-4xl! text-red-300">favorite</i>
                </button>
            {/if}
        </div>
        <div class="controls__bottom">
            <button class="controls__link controls__bottom-left" type="button" onclick={onAbout}>
                aurelius
            </button>
            <span class="controls__bottom-center"></span>
            <span class="controls__bottom-right">{durationText}</span>
        </div>
    </div>
</nav>

<style>
    .controls {
        background-color: hsl(0, 0%, 33%);
        color: white;
        display: flex;
        align-items: center;
    }

    .track-image-container {
        display: flex;
        align-items: center;
        cursor: inherit;
    }

    .track-image {
        height: 6rem;
        width: 6rem;
        margin: 0.5rem;
        object-fit: contain;
    }

    /* Controls to the right of the track image */
    .everything-else {
        flex: 1;
        position: relative;
    }

    .marquee-spacer {
        position: relative;
        height: 1.1em;
        margin: 0 0.5rem 0 0.5rem;
    }
    .marquee-container {
        position: absolute;
        top: 0;
        left: 0;
        width: 100%;
        height: 200%;
        overflow: hidden;
    }
    .marquee-text {
        font-size: 1.1em;
    }

    /* Positioning */
    .controls__group {
        display: flex;
        height: 3rem;
        align-items: center;
        justify-content: center;
    }
    .controls__group--shift-up {
        position: relative;
        bottom: 0.5em;
    }

    .controls__link {
        cursor: pointer;
        font-style: italic;
        text-decoration: none;
        color: inherit;
    }
    .controls__link:hover {
        text-decoration: underline;
    }

    /* Text below controls */
    .controls__bottom-left {
        position: absolute;
        bottom: 0.5rem;
        left: 0.5rem;
    }
    .controls__bottom-right {
        position: absolute;
        bottom: 0.5rem;
        right: 0.5rem;
    }

    /* Prevent text from overlapping buttons */
    @media (max-width: 530px) {
        .controls__bottom {
            display: flex;
        }
        .controls__bottom-left {
            position: relative;
        }
        .controls__bottom-right {
            position: relative;
        }
        .controls__bottom-center {
            flex: 1;
        }
        .track-image {
            width: 7rem;
            height: 7rem;
        }
    }
</style>
