import type { Player } from "../core/player";
import type { Track } from "../core/track";
import { createSubscriber } from "svelte/reactivity";

export type PlayerState = ReturnType<typeof makePlayerState>;

export function makePlayerState(player: Player) {
    const updateOnBufferProgress = createSubscriber((update) => {
        player.addEventListener("progress", update);
        player.addEventListener("timeupdate", update);
        return () => {
            player.removeEventListener("progress", update);
            player.removeEventListener("timeupdate", update);
        };
    });

    const updateOnPauseUnpause = createSubscriber((update) => {
        player.addEventListener("pause", update);
        player.addEventListener("unpause", update);
        return () => {
            player.removeEventListener("pause", update);
            player.removeEventListener("unpause", update);
        };
    });

    const updateOnTimeUpdate = createSubscriber((update) => {
        player.addEventListener("timeupdate", update);
        return () => {
            player.removeEventListener("timeupdate", update);
        };
    });

    const updateOnFavoriteUnfavorite = createSubscriber((update) => {
        player.addEventListener("favorite", update);
        player.addEventListener("unfavorite", update);
        return () => {
            player.removeEventListener("favorite", update);
            player.removeEventListener("unfavorite", update);
        };
    });

    let track = $state(player.track);
    let paused = $derived.by(() => {
        updateOnPauseUnpause();
        return track?.isPaused() ?? false;
    });
    let currentTime = $derived.by(() => {
        updateOnTimeUpdate();
        return track?.currentTime() ?? 0;
    });
    let duration = $derived(track?.info.duration ?? 0);
    let favorite = $derived.by(() => {
        updateOnFavoriteUnfavorite();
        return track?.info.favorite ?? false;
    });
    let hasNext = $derived.by(() => {
        track; // Update when track updates.
        return player.hasNext();
    });
    let hasPrevious = $derived.by(() => {
        track; // Update when track updates.
        return player.hasPrevious();
    });

    player.addEventListener("play", (t: Track) => {
        track = t;
    });
    player.addEventListener("ended", () => {
        track = undefined;
    });

    return {
        player,
        get track() {
            return track;
        },
        get paused() {
            return paused;
        },
        get currentTime() {
            return currentTime;
        },
        get duration() {
            return duration;
        },
        get favorite() {
            return favorite;
        },
        get hasNext() {
            return hasNext;
        },
        get hasPrevious() {
            return hasPrevious;
        },
        updateOnBufferProgress,
        updateOnPauseUnpause,
        updateOnTimeUpdate,
        updateOnFavoriteUnfavorite,
    };
}
