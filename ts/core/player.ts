import { Track, StreamOptions } from "./track";
import { Playlist, LocalPlaylist, RemotePlaylist } from "./playlist";
import { PlayHistory } from "./history";
import EventDispatcher from "./eventdispatcher";

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

export class Player extends EventDispatcher<PlayerEventMap> {
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
            this.dispatchEvent("progress");
        });
        track.addEventListener("timeupdate", () => {
            this.dispatchEvent("timeupdate");
        });
        track.addEventListener("ended", async () => {
            if (!await this.next()) {
                this.pause();
                if (this.track !== undefined) {
                    this.track.rewind();
                }
                this.dispatchEvent("ended");
            }
        });

        await track.play();
        this.dispatchEvent("play");
    }

    public seekTo(seconds: number): Promise<void> {
        if (this.track === undefined) {
            return Promise.resolve();
        }
        if (this.track.seekTo(seconds)) {
            return Promise.resolve();
        }
        return this._play(this.track.url, seconds);
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
        configOverride?: {
            random?: boolean,
            startPos?: number,
        }
    ): Promise<boolean> {
        const config = {
            random: false,
            startPos: 0,
            ...configOverride
        };

        if (typeof playlistUrlOrTrackUrls === "string") {
            const url = playlistUrlOrTrackUrls;
            this.playlist = await RemotePlaylist.fetch(url);
        } else {
            const trackUrls = playlistUrlOrTrackUrls;
            this.playlist = new LocalPlaylist(trackUrls);
        }

        this.playlistPos = config.startPos - 1;
        this.history = new PlayHistory();
        this.random = config.random;

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
        await this._play(item.path);
        return true;
    }

    public async previous(): Promise<boolean> {
        let item = this.history.previous();
        if (item === undefined) {
            if (this.random
                || this.playlist === undefined || this.playlist.length() < 1
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
        await this._play(item.path);
        return true;
    }

    public pause(): void {
        if (this.track === undefined || this.track.isPaused()) {
            return;
        }
        this.track.pause();
        this.dispatchEvent("pause");
    }

    public unpause(): void {
        if (this.track === undefined || !this.track.isPaused()) {
            return;
        }
        this.track.play();
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

    public setStreamOptions(options: StreamOptions): void {
        this.streamOptions = options;
    }
}
