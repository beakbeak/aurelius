import * as util from "./util.js";
import { Track, StreamOptions } from "./track.js";
import { Playlist, LocalPlaylist, RemotePlaylist }  from "./playlist.js";
import { PlayHistory } from "./history.js";

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

export class Player extends util.EventDispatcher<PlayerEventMap> {
    public track?: Track;
    public playlist?: Playlist;
    public history = new PlayHistory();
    public playlistPos = -1;
    public random = false;
    public streamOptions: StreamOptions = { codec: "vorbis", quality: 8 };

    private async _play(
        url: string,
        startTime?: number,
    ): Promise<void> {
        const track = await Track.fetch(url, this.streamOptions, startTime, this.track);
        this.track = track;

        track.addEventListener("progress", () => {
            this._dispatchEvent("progress");
        });
        track.addEventListener("timeupdate", () => {
            this._dispatchEvent("timeupdate");
        });
        track.addEventListener("ended", async () => {
            if (!await this.next()) {
                this.pause();
                if (this.track !== undefined) {
                    this.track.audio.currentTime = 0;
                }
                this._dispatchEvent("ended");
            }
        });

        track.audio.play();
        this._dispatchEvent("play");
    }

    private async _playOr(
        url: string,
        startTime: number | undefined,
        canProceed: () => boolean,
        proceed: () => Promise<boolean>,
    ): Promise<boolean> {
        try {
            await this._play(url, startTime);
            return true;
        } catch(e) {
            console.log("playback failed:", url, startTime, e);
        }

        while (canProceed()) {
            try {
                return await proceed();
            } catch(e) {
                console.log("playback failed:", e);
            }
        }
        return false;
    }

    private _playOrNext(
        url: string,
        startTime?: number,
    ): Promise<boolean> {
        return this._playOr(
            url, startTime,
            () => { return this.hasNext(); },
            () => { return this.next(); });
    }

    private _playOrPrevious(
        url: string,
        startTime?: number,
    ): Promise<boolean> {
        return this._playOr(
            url, startTime,
            () => { return this.hasPrevious(); },
            () => { return this.previous(); });
    }

    public async seekTo(seconds: number): Promise<void> {
        if (this.track === undefined) {
            return;
        }
        await this._playOrNext(this.track.url, seconds);
    }

    public playTrack(url: string): Promise<void> {
        if (this.playlist !== undefined) {
            this.playlist = undefined;
            this.playlistPos = -1;
            this.history = new PlayHistory();
        }
        this.history.push({ path: url, pos: 0 });
        return this._play(url);
    }

    public async playList(
        playlistUrlOrTrackUrls: string | string[],
        random = false,
        startPos = 0,
    ): Promise<boolean> {
        if (typeof playlistUrlOrTrackUrls === "string") {
            const url = playlistUrlOrTrackUrls;
            this.playlist = await RemotePlaylist.fetch(url);
        } else {
            const trackUrls = playlistUrlOrTrackUrls;
            this.playlist = new LocalPlaylist(trackUrls);
        }

        this.playlistPos = startPos - 1;
        this.history = new PlayHistory();
        this.random = random;

        return this.next();
    }

    public hasNext(): boolean {
        if (this.history.hasNext()) {
            return true;
        }
        if (this.playlist === undefined || this.playlist.length() < 1) {
            return false;
        }
        return this.random || this.playlistPos < (this.playlist.length() - 1);
    }

    public hasPrevious(): boolean {
        if (this.history.hasPrevious()) {
            return true;
        }
        if (this.random || this.playlist === undefined || this.playlist.length() < 1) {
            return false;
        }
        return this.playlistPos > 0;
    }

    public async next(): Promise<boolean> {
        let item = this.history.next();
        if (item === undefined) {
            if (this.playlist === undefined || this.playlist.length() < 1) {
                return false;
            }

            if (this.random) {
                item = await this.playlist.random();
            } else if (this.playlistPos < (this.playlist.length() - 1)) {
                item = await this.playlist.at(this.playlistPos + 1);
            }

            if (item === undefined) {
                return false;
            }
            this.history.push(item);
        }
        this.playlistPos = item.pos;
        return this._playOrNext(item.path);
    }

    public async previous(): Promise<boolean> {
        let item = this.history.previous();
        if (item === undefined) {
            if (this.playlist === undefined || this.playlist.length() < 1
                || this.playlistPos <= 0)
            {
                return false;
            }

            item = await this.playlist.at(this.playlistPos - 1);
            if (item === undefined) {
                return false;
            }
            this.history.pushFront(item);
        }
        this.playlistPos = item.pos;
        return this._playOrPrevious(item.path);
    }

    public pause(): void {
        if (this.track === undefined || this.track.audio.paused) {
            return;
        }
        this.track.audio.pause();
        this._dispatchEvent("pause");
    }

    public unpause(): void {
        if (this.track === undefined || !this.track.audio.paused) {
            return;
        }
        this.track.audio.play();
        this._dispatchEvent("unpause");
    }

    public async favorite(): Promise<void> {
        if (this.track === undefined) {
            return;
        }
        await this.track.favorite();
        this._dispatchEvent("favorite");
    }

    public async unfavorite(): Promise<void> {
        if (this.track === undefined) {
            return;
        }
        await this.track.unfavorite();
        this._dispatchEvent("unfavorite");
    }

    public setStreamOptions(options: StreamOptions): void {
        this.streamOptions = options;
    }
}