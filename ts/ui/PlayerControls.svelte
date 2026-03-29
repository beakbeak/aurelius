<script lang="ts">
    import type { Player } from "../core/player";
    import type { PlayerState } from "./state/playerState.svelte";
    import Marquee from "./Marquee.svelte";
    import ProgressBar from "./ProgressBar.svelte";
    import { formatDuration } from "./format";
    import { getSettings } from "./settings";

    const defaultTrackImageUrl = "/static/img/aurelius.svgz";

    let {
        player,
        playerState,
        onAbout,
        onNavigateToDir,
    }: {
        player: Player;
        playerState: PlayerState;
        onAbout: () => void;
        onNavigateToDir: (url: string) => void;
    } = $props();

    let notificationData = $state<{ title: string; body: string; icon?: string } | undefined>(
        undefined,
    );

    let trackImageUrl = $derived.by(() => {
        const info = playerState.trackInfo;
        if (!info) return defaultTrackImageUrl;
        return info.attachedImages.length > 0 ? info.attachedImages[0].url : defaultTrackImageUrl;
    });

    let trackImageCursor = $derived(trackImageUrl !== defaultTrackImageUrl ? "pointer" : "default");

    let marqueeText = $derived.by(() => {
        const info = playerState.trackInfo;
        if (!info) return "";
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

    let marqueeUrl = $derived(playerState.trackInfo?.dir ?? "");

    let durationText = $derived.by(() => {
        const track = playerState.track;
        if (!track) return "";
        const currentTimeStr = formatDuration(playerState.currentTime);
        const durationStr = formatDuration(playerState.duration);
        return `${currentTimeStr} / ${durationStr}`;
    });

    let hasTrack = $derived(playerState.track !== undefined);
    let isPaused = $derived(playerState.paused);
    let isFavorite = $derived(playerState.favorite);

    function openTrackImageInNewTab(): void {
        if (trackImageUrl !== defaultTrackImageUrl) {
            window.open(trackImageUrl, "_blank");
        }
    }

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

    // Update MediaSession and notification data when track info changes
    $effect(() => {
        const info = playerState.trackInfo;
        const track = playerState.track;

        if (!info || !track) {
            notificationData = undefined;
            return;
        }

        const artist = info.tags["artist"] ?? info.tags["composer"] ?? "";
        const title = info.tags["title"] ?? info.name;
        const favoriteIcon = info.favorite ? "\u2665\uFE0E" : "\u2661";

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
    });

    // Handle autoNext notifications
    $effect(() => {
        if (playerState.autoNextFired) {
            showDesktopNotification();
        }
    });

    // Set up MediaSession action handlers once
    $effect(() => {
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

<nav class="controls main__controls">
    <div class="controls__track-image-container">
        <img
            class="controls__track-image"
            src={trackImageUrl}
            alt=""
            style:cursor={trackImageCursor}
            onclick={openTrackImageInNewTab}
        />
    </div>
    <div class="controls__everything-else">
        <div class="controls__marquee-spacer">
            <div class="controls__marquee-container">
                <Marquee text={marqueeText} url={marqueeUrl} onNavigate={onNavigateToDir} />
            </div>
        </div>
        <ProgressBar {player} {playerState} />
        <div class="controls__group controls__group--shift-up">
            <i
                class="controls__button material-icons"
                class:controls__button--disabled={!playerState.hasPrevious}
                title="Previous track"
                onclick={() => player.previous()}
            >
                skip_previous
            </i>
            {#if !hasTrack || isPaused}
                <i
                    class="controls__button material-icons"
                    class:controls__button--disabled={!hasTrack}
                    title="Play"
                    onclick={() => player.unpause()}
                >
                    play_arrow
                </i>
            {:else}
                <i
                    class="controls__button material-icons"
                    title="Pause"
                    onclick={() => player.pause()}
                >
                    pause
                </i>
            {/if}
            <i
                class="controls__button material-icons"
                class:controls__button--disabled={!playerState.hasNext}
                title="Next track"
                onclick={() => player.next()}
            >
                skip_next
            </i>
            {#if !isFavorite}
                <i
                    class="controls__button controls__button--medium material-icons"
                    class:controls__button--disabled={!hasTrack}
                    title="Add to favorites"
                    onclick={() => player.favorite()}
                >
                    favorite_border
                </i>
            {:else}
                <i
                    class="controls__button controls__button--medium material-icons unfavorite-button"
                    title="Remove from favorites"
                    onclick={() => player.unfavorite()}
                >
                    favorite
                </i>
            {/if}
        </div>
        <div class="controls__bottom">
            <span class="controls__link controls__bottom-left" onclick={onAbout}> aurelius </span>
            <span class="controls__bottom-center"></span>
            <span class="controls__bottom-right">{durationText}</span>
        </div>
    </div>
</nav>

<style>
    .unfavorite-button {
        color: hsl(0, 70%, 72.9%);
    }
</style>
