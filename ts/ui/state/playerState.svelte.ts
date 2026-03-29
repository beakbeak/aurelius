import type { Player } from "../../core/player";
import type { Track } from "../../core/track";
import type { TrackInfo } from "../../core/track";

export interface PlayerState {
    readonly track: Track | undefined;
    readonly trackInfo: TrackInfo | undefined;
    readonly paused: boolean;
    readonly currentTime: number;
    readonly duration: number;
    readonly hasNext: boolean;
    readonly hasPrevious: boolean;
    readonly favorite: boolean;
    readonly ended: boolean;
    readonly autoNextFired: boolean;
    readonly updateCounter: number;
}

export function createPlayerState(player: Player): PlayerState {
    let track = $state<Track | undefined>(player.track);
    let paused = $state(true);
    let currentTime = $state(0);
    let duration = $state(0);
    let hasNext = $state(player.hasNext());
    let hasPrevious = $state(player.hasPrevious());
    let favorite = $state(false);
    let ended = $state(false);
    let autoNextFired = $state(false);
    let updateCounter = $state(0);

    function refresh(t: Track): void {
        track = t;
        paused = t.isPaused();
        currentTime = t.currentTime();
        duration = t.info.duration;
        hasNext = player.hasNext();
        hasPrevious = player.hasPrevious();
        favorite = t.info.favorite;
        ended = false;
        updateCounter++;
    }

    player.addEventListener("play", (t: Track) => {
        autoNextFired = false;
        refresh(t);
    });

    player.addEventListener("pause", (t: Track) => {
        paused = true;
        track = t;
        updateCounter++;
    });

    player.addEventListener("unpause", (t: Track) => {
        paused = false;
        track = t;
        updateCounter++;
    });

    player.addEventListener("timeupdate", (t: Track) => {
        currentTime = t.currentTime();
        track = t;
        updateCounter++;
    });

    player.addEventListener("progress", (t: Track) => {
        track = t;
        updateCounter++;
    });

    player.addEventListener("ended", () => {
        track = undefined;
        paused = true;
        currentTime = 0;
        duration = 0;
        hasNext = player.hasNext();
        hasPrevious = player.hasPrevious();
        favorite = false;
        ended = true;
    });

    player.addEventListener("favorite", (t: Track) => {
        favorite = true;
        track = t;
        updateCounter++;
    });

    player.addEventListener("unfavorite", (t: Track) => {
        favorite = false;
        track = t;
        updateCounter++;
    });

    player.addEventListener("autoNext", () => {
        autoNextFired = true;
    });

    return {
        get track() {
            return track;
        },
        get trackInfo() {
            return track?.info;
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
        get hasNext() {
            return hasNext;
        },
        get hasPrevious() {
            return hasPrevious;
        },
        get favorite() {
            return favorite;
        },
        get ended() {
            return ended;
        },
        get autoNextFired() {
            return autoNextFired;
        },
        get updateCounter() {
            return updateCounter;
        },
    };
}
