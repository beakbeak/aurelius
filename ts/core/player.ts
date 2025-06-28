import { Track, StreamConfig, ReplayGainMode } from "./track";
import { Playlist, LocalPlaylist, RemotePlaylist } from "./playlist";
import { PlayHistory } from "./history";
import EventDispatcher from "./eventdispatcher";
import { copyJson } from "./json";

export interface PlayerEventMap {
    play: () => void;
    progress: () => void;
    timeupdate: () => void;
    ended: () => void;
    pause: () => void;
    unpause: () => void;
    favorite: () => void;
    unfavorite: () => void;
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

    public track?: Track;
    public playlist?: Playlist;

    public constructor(config: PlayerConfig = {}) {
        super();
        this.streamConfig = config.streamConfig || {};
        this._stallDetectionEnabled = config.enableStallDetection ?? true;
    }

    public streamConfig: PlayerStreamConfig;

    // Start backup stall detection timer to catch unreported stalls.
    private _startStallDetection(): void {
        if (!this._stallDetectionEnabled) {
            return;
        }
        this._stopStallDetection();
        this._lastStallCheck = { timeMs: Date.now(), seekPos: this.track?.currentTime() || 0 };
        this._stallDetectionTimer = window.setInterval(() => {
            this._restartIfStalled();
        }, this._stallDetectionIntervalMs);
    }

    private _stopStallDetection(): void {
        if (this._stallDetectionTimer !== undefined) {
            clearInterval(this._stallDetectionTimer);
            this._stallDetectionTimer = undefined;
        }
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
                console.log(
                    new Date().toISOString(),
                    `Player: Playback stalled for ${stallDuration / 1000}s; restarting`,
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

    private async _play(url: string, startTime?: number): Promise<void> {
        console.log(new Date().toISOString(), "Player._play", url, startTime);
        this._stopStallDetection();

        let streamConfig: StreamConfig;
        if (this.streamConfig.replayGain === "auto") {
            streamConfig = copyJson(this.streamConfig);
            streamConfig.replayGain = this._replayGainHint;
        } else {
            streamConfig = this.streamConfig;
        }

        const track = await Track.fetch(url, streamConfig, startTime, this.track);
        this.track = track;

        let wasPaused = false;

        track.addEventListener("pause", () => {
            this._stopStallDetection();
            wasPaused = true;
            this.dispatchEvent("pause");
        });
        track.addEventListener("play", () => {
            this._startStallDetection();
            // Don't dispatch "unpause" in response to initial "play" event
            if (wasPaused) {
                this.dispatchEvent("unpause");
            }
            wasPaused = false;
        });

        track.addEventListener("progress", () => {
            this.dispatchEvent("progress");
        });
        track.addEventListener("timeupdate", () => {
            this._lastStallCheck = { timeMs: Date.now(), seekPos: this.track?.currentTime() || 0 };
            this.dispatchEvent("timeupdate");
        });

        track.addEventListener("ended", async () => {
            this._stopStallDetection();
            if (!(await this.next())) {
                if (this.track !== undefined) {
                    this.track.destroy();
                    delete this.track;
                }
                this.dispatchEvent("ended");
            }
        });

        track.addEventListener("stalled", () => {
            if (this.track === undefined || this.track.isPaused()) {
                return;
            }
            this._play(this.track.url, this.track.currentTime());
        });

        await track.play();
        this._startStallDetection();
        this.dispatchEvent("play");
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
        },
    ): Promise<boolean> {
        const config = {
            random: false,
            startPos: 0,
            replayGainHint: ReplayGainMode.Track,
            ...configOverride,
        };

        if (typeof playlistUrlOrTrackUrls === "string") {
            const url = playlistUrlOrTrackUrls;
            this.playlist = await RemotePlaylist.fetch(url);
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
        let item = this._history.next();
        if (item === undefined) {
            if (this.playlist === undefined || this.playlist.length() < 1) {
                return false;
            }

            if (this._random) {
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
        this.dispatchEvent("pause");
    }

    public unpause(): void {
        if (this.track === undefined || !this.track.isPaused()) {
            return;
        }
        this.track.play();
        this._startStallDetection();
        this.dispatchEvent("unpause");
    }

    public async favorite(): Promise<void> {
        if (this.track === undefined) {
            return;
        }
        await this.track.favorite();
        this.dispatchEvent("favorite");
    }

    public async unfavorite(): Promise<void> {
        if (this.track === undefined) {
            return;
        }
        await this.track.unfavorite();
        this.dispatchEvent("unfavorite");
    }
}
