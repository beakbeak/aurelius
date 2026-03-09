import { Track, StreamConfig, ReplayGainMode } from "./track";
import { Playlist, PlaylistItem, LocalPlaylist, RemotePlaylist } from "./playlist";
import { PlayHistory } from "./history";
import EventDispatcher from "./eventdispatcher";
import { copyJson } from "./json";
import { LogLevel, serverLog } from "./log";

export interface PlayerEventMap {
    play: (track: Track) => void;
    progress: (track: Track) => void;
    timeupdate: (track: Track) => void;
    ended: (track: Track) => void;
    pause: (track: Track) => void;
    unpause: (track: Track) => void;
    favorite: (track: Track) => void;
    unfavorite: (track: Track) => void;
    autoNext: (track: Track) => void;
}
export type PlayerEvent = keyof PlayerEventMap;

export type PlayerStreamConfig =
    | StreamConfig
    | (Omit<StreamConfig, "replayGain"> & {
          replayGain: "auto";
      });

export interface PlayerConfig {
    streamConfig?: PlayerStreamConfig;
    enableStallDetection?: boolean;
}

const preloadLeadTimeSeconds = 5;

export class Player extends EventDispatcher<PlayerEventMap> {
    private _history = new PlayHistory();
    private _playlistPos = -1;
    private _random = false;
    private _replayGainHint = ReplayGainMode.Track;

    private _stallDetectionTimer?: number;
    private _lastStallCheck = { timeMs: 0, seekPos: 0 };
    private _stallDetectionIntervalMs = 1000;
    private _stallThresholdMs = 2000;
    private _isRestarting = false;
    private _stallDetectionEnabled = true;

    private _preload?: { track?: Track; item: PlaylistItem };
    private _streamConfig: PlayerStreamConfig;

    public track?: Track;
    public playlist?: Playlist;

    public constructor(config: PlayerConfig = {}) {
        super();
        this._streamConfig = config.streamConfig || {};
        this._stallDetectionEnabled = config.enableStallDetection ?? true;
    }

    public get streamConfig(): PlayerStreamConfig {
        return this._streamConfig;
    }

    public set streamConfig(value: PlayerStreamConfig) {
        this._streamConfig = value;
        this._discardPreload();
    }

    private _discardPreload(): void {
        if (this._preload?.track) {
            this._preload.track.destroy();
        }
        this._preload = undefined;
    }

    private async _peekNextItem(): Promise<PlaylistItem | undefined> {
        const historyNext = this._history.peekNext();
        if (historyNext !== undefined) {
            return historyNext;
        }
        if (this.playlist === undefined || this.playlist.length() < 1) {
            return undefined;
        }
        if (this._random) {
            return this.playlist.random();
        }
        if (this._playlistPos < this.playlist.length() - 1) {
            return this.playlist.at(this._playlistPos + 1);
        }
        return undefined;
    }

    private async _preloadNext(): Promise<void> {
        if (this._preload) {
            return;
        }
        try {
            const nextItem = await this._peekNextItem();
            if (!nextItem) {
                return;
            }
            const preload: { track?: Track; item: PlaylistItem } = { item: nextItem };
            this._preload = preload;

            const streamConfig = this._resolveStreamConfig();
            const track = await Track.fetch(nextItem.path, streamConfig, 0);

            // Check that this preload wasn't invalidated while we were fetching.
            if (this._preload !== preload) {
                track.destroy();
                return;
            }
            preload.track = track;
            serverLog(LogLevel.Info, "preloaded next track", { track: track.info.name });
        } catch (e) {
            serverLog(LogLevel.Warn, "failed to preload next track", { error: `${e}` });
        }
    }

    private _resolveStreamConfig(): StreamConfig {
        if (this._streamConfig.replayGain === "auto") {
            const config = copyJson(this._streamConfig);
            config.replayGain = this._replayGainHint;
            return config;
        }
        return this._streamConfig;
    }

    // Start backup stall detection timer to catch unreported stalls.
    private _startStallDetection(): void {
        if (!this._stallDetectionEnabled) {
            return;
        }
        this._stopStallDetection();
        serverLog(LogLevel.Info, "start stall detection");
        this._lastStallCheck = { timeMs: Date.now(), seekPos: this.track?.currentTime() || 0 };
        this._stallDetectionTimer = window.setInterval(() => {
            this._restartIfStalled();
        }, this._stallDetectionIntervalMs);
    }

    private _stopStallDetection(): void {
        if (this._stallDetectionTimer === undefined) {
            return;
        }
        serverLog(LogLevel.Info, "stop stall detection");
        clearInterval(this._stallDetectionTimer);
        this._stallDetectionTimer = undefined;
    }

    // Check if playback has stalled without triggering a 'stalled' event.
    // Restarts playback if position hasn't advanced for _stallThresholdMs.
    private _restartIfStalled(): void {
        if (this.track === undefined || this.track.isPaused() || this._isRestarting) {
            return;
        }

        const currentTime = this.track.currentTime();
        const now = Date.now();

        if (currentTime === this._lastStallCheck.seekPos) {
            const stallDuration = now - this._lastStallCheck.timeMs;
            if (stallDuration >= this._stallThresholdMs) {
                serverLog(
                    LogLevel.Warn,
                    `playback stalled for ${stallDuration / 1000}s; restarting`,
                    { track: this.track?.info.name },
                );
                this._isRestarting = true;
                this._play(this.track.url, currentTime).finally(() => {
                    this._isRestarting = false;
                });
                return;
            }
        } else {
            this._lastStallCheck = { timeMs: now, seekPos: currentTime };
        }
    }

    private _attachTrackListeners(track: Track): void {
        let wasPaused = false;
        let preloadTriggered = false;

        track.addEventListener("pause", () => {
            serverLog(LogLevel.Info, "track: pause", { track: track.info.name });
            this._stopStallDetection();
            wasPaused = true;
            this.dispatchEvent("pause", track);
        });
        track.addEventListener("play", () => {
            serverLog(LogLevel.Info, "track: play", { track: track.info.name });
            this._startStallDetection();
            // Don't dispatch "unpause" in response to initial "play" event
            if (wasPaused) {
                this.dispatchEvent("unpause", track);
            }
            wasPaused = false;
        });

        track.addEventListener("progress", () => {
            this.dispatchEvent("progress", track);
        });
        track.addEventListener("timeupdate", () => {
            this._lastStallCheck = { timeMs: Date.now(), seekPos: this.track?.currentTime() || 0 };
            this.dispatchEvent("timeupdate", track);

            // Trigger preloading when nearing end of track.
            if (!preloadTriggered && track.info.duration > 0) {
                const remaining = track.info.duration - track.currentTime();
                if (remaining <= preloadLeadTimeSeconds) {
                    preloadTriggered = true;
                    this._preloadNext();
                }
            }
        });

        track.addEventListener("ended", async () => {
            serverLog(LogLevel.Info, "track: ended", { track: track.info.name });
            this._stopStallDetection();
            const advanced = await this.next();
            if (advanced) {
                this.dispatchEvent("autoNext", track);
            } else {
                if (this.track !== undefined) {
                    this.track.destroy();
                    delete this.track;
                }
                this.dispatchEvent("ended", track);
            }
        });

        track.addEventListener("stalled", () => {
            serverLog(LogLevel.Info, "track: stalled", { track: track.info.name });
            if (this.track !== track || this.track.isPaused()) {
                return;
            }
            this._play(this.track.url, this.track.currentTime());
        });
    }

    private async _play(url: string, startTime?: number): Promise<void> {
        console.debug(new Date().toISOString(), "Player._play", url, startTime);
        this._stopStallDetection();

        // Use preloaded track if it matches.
        let track: Track;
        if (!startTime && this._preload?.track && this._preload.item.path === url) {
            track = this._preload.track;
            this._preload = undefined;
        } else {
            this._discardPreload();
            const streamConfig = this._resolveStreamConfig();
            track = await Track.fetch(url, streamConfig, startTime);
        }

        if (this.track) {
            this.track.destroy();
        }
        this.track = track;
        console.debug("Track info:", track.info);

        this._attachTrackListeners(track);
        await track.play();
        this._startStallDetection();
        this.dispatchEvent("play", track);
    }

    public seekTo(seconds: number): Promise<void> {
        if (this.track === undefined) {
            return Promise.resolve();
        }
        if (this.track.seekTo(seconds)) {
            this._lastStallCheck = { timeMs: Date.now(), seekPos: seconds };
            return Promise.resolve();
        }
        return this._play(this.track.url, seconds);
    }

    public playTrack(url: string): Promise<void> {
        this._discardPreload();
        if (this.playlist !== undefined) {
            this.playlist = undefined;
            this._playlistPos = -1;
            this._history = new PlayHistory();
        }
        this._history.push({ path: url, pos: 0 });
        return this._play(url);
    }

    public async playList(
        playlistUrlOrTrackUrls: string | string[],
        configOverride?: {
            random?: boolean;
            startPos?: number;
            replayGainHint?: ReplayGainMode;
            prefix?: string;
        },
    ): Promise<boolean> {
        this._discardPreload();

        const config = {
            random: false,
            startPos: 0,
            replayGainHint: ReplayGainMode.Track,
            ...configOverride,
        };

        if (typeof playlistUrlOrTrackUrls === "string") {
            const url = playlistUrlOrTrackUrls;
            this.playlist = await RemotePlaylist.fetch(url, config.prefix);
        } else {
            const trackUrls = playlistUrlOrTrackUrls;
            this.playlist = new LocalPlaylist(trackUrls);
        }

        this._playlistPos = config.startPos - 1;
        this._history = new PlayHistory();
        this._random = config.random;
        this._replayGainHint = config.replayGainHint;

        return this.next();
    }

    public hasNext(): boolean {
        if (this._history.hasNext()) {
            return true;
        }
        if (this.playlist === undefined || this.playlist.length() < 1) {
            return false;
        }
        return this._random || this._playlistPos < this.playlist.length() - 1;
    }

    public hasPrevious(): boolean {
        if (this._history.hasPrevious()) {
            return true;
        }
        if (this._random || this.playlist === undefined || this.playlist.length() < 1) {
            return false;
        }
        return this._playlistPos > 0;
    }

    public async next(): Promise<boolean> {
        serverLog(LogLevel.Info, "player: next", { track: this.track?.info.name });
        let item = this._history.next();
        if (item === undefined) {
            // Use the pre-selected preloaded item if available.
            if (this._preload?.item) {
                item = this._preload.item;
            } else if (this.playlist === undefined || this.playlist.length() < 1) {
                return false;
            } else if (this._random) {
                item = await this.playlist.random();
            } else if (this._playlistPos < this.playlist.length() - 1) {
                item = await this.playlist.at(this._playlistPos + 1);
            }

            if (item === undefined) {
                return false;
            }
            this._history.push(item);
        }
        this._playlistPos = item.pos;
        await this._play(item.path);
        return true;
    }

    public async previous(): Promise<boolean> {
        serverLog(LogLevel.Info, "player: previous", { track: this.track?.info.name });
        this._discardPreload();
        let item = this._history.previous();
        if (item === undefined) {
            if (
                this._random ||
                this.playlist === undefined ||
                this.playlist.length() < 1 ||
                this._playlistPos <= 0
            ) {
                return false;
            }

            item = await this.playlist.at(this._playlistPos - 1);
            if (item === undefined) {
                return false;
            }
            this._history.pushFront(item);
        }
        this._playlistPos = item.pos;
        await this._play(item.path);
        return true;
    }

    public pause(): void {
        if (this.track === undefined || this.track.isPaused()) {
            return;
        }
        this._stopStallDetection();
        this.track.pause();
    }

    public unpause(): void {
        if (this.track === undefined || !this.track.isPaused()) {
            return;
        }
        this.track.play();
        this._startStallDetection();
    }

    public async favorite(): Promise<void> {
        if (this.track === undefined) {
            return;
        }
        await this.track.favorite();
        this.dispatchEvent("favorite", this.track);
    }

    public async unfavorite(): Promise<void> {
        if (this.track === undefined) {
            return;
        }
        await this.track.unfavorite();
        this.dispatchEvent("unfavorite", this.track);
    }
}
