import { host, EventChecker } from "../testing";

import { Player } from "./player";
import { fetchTrackInfo } from "./track";
import { RemotePlaylist } from "./playlist";

import { ok } from "assert";

const playlistUrl = `${host}/db/test.m3u`;
const trackUrl = `${host}/db/test.flac`;

describe("Player", function () {
    it("toggles favorite status", async function () {
        const player = new Player();

        const gotFavorite = new EventChecker();
        const gotUnfavorite = new EventChecker();
        player.addEventListener("favorite", () => { gotFavorite.set(); });
        player.addEventListener("unfavorite", () => { gotUnfavorite.set(); });

        await player.unfavorite();
        ok(!gotUnfavorite.consume());
        await player.favorite();
        ok(!gotFavorite.consume());

        await player.playTrack(trackUrl);

        await player.unfavorite();
        ok((await fetchTrackInfo(trackUrl)).favorite === false);
        ok(gotUnfavorite.consume());

        await player.favorite();
        ok((await fetchTrackInfo(trackUrl)).favorite === true);
        ok(gotFavorite.consume());

        await player.unfavorite();
        ok((await fetchTrackInfo(trackUrl)).favorite === false);
        ok(gotUnfavorite.consume());
    });

    it("cycles through a playlist", async function () {
        const playlist = await RemotePlaylist.fetch(playlistUrl);
        const player = new Player();

        const gotPlay = new EventChecker();
        player.addEventListener("play", () => { gotPlay.set(); });

        await player.playList(playlistUrl, { startPos: 1 });
        ok(gotPlay.consume());

        ok(player.hasPrevious());
        ok(await player.previous());
        ok(gotPlay.consume());

        ok(!player.hasPrevious());
        ok(!await player.previous());
        ok(!gotPlay.consume());

        for (let i = 1, end = playlist.length() >> 1; i < end; ++i) {
            ok(player.hasNext());
            ok(await player.next());
            ok(gotPlay.consume());
        }

        for (let i = 1, end = playlist.length() >> 1; i < end; ++i) {
            ok(player.hasPrevious());
            ok(await player.previous());
            ok(gotPlay.consume());
        }

        ok(!player.hasPrevious());
        ok(!await player.previous());
        ok(!gotPlay.consume());

        for (let i = 1, end = playlist.length(); i < end; ++i) {
            ok(player.hasNext());
            ok(await player.next());
            ok(gotPlay.consume());
        }

        ok(!player.hasNext());
        ok(!await player.next());
        ok(!gotPlay.consume());

        for (let i = 1, end = playlist.length(); i < end; ++i) {
            ok(player.hasPrevious());
            ok(await player.previous());
            ok(gotPlay.consume());
        }

        ok(!player.hasPrevious());
        ok(!await player.previous());
        ok(!gotPlay.consume());

        await player.playTrack(trackUrl);
        ok(gotPlay.consume());
        ok(!player.hasNext());
        ok(!player.hasPrevious());
        ok(!await player.next());
        ok(!await player.previous());
        ok(!gotPlay.consume());
    });

    it("pauses and unpauses a track", async function () {
        const player = new Player();

        const gotPause = new EventChecker();
        const gotUnpause = new EventChecker();
        player.addEventListener("pause", () => { gotPause.set(); });
        player.addEventListener("unpause", () => { gotUnpause.set(); });

        await player.playTrack(trackUrl);
        player.unpause();
        ok(!gotUnpause.consume());
        player.pause();
        ok(gotPause.consume());
        player.pause();
        ok(!gotPause.consume());
        player.unpause();
        ok(gotUnpause.consume());
    });

    it("plays a playlist in random order", async function () {
        const playlist = await RemotePlaylist.fetch(playlistUrl);
        const player = new Player();

        ok(await player.playList(playlistUrl, { random: true }));
        ok(!player.hasPrevious());
        ok(!await player.previous());

        const moreThanOriginalLength = playlist.length() + 2;
        for (let i = 0; i < moreThanOriginalLength; ++i) {
            ok(player.hasNext());
            ok(await player.next());
        }
        for (let i = 0; i < moreThanOriginalLength; ++i) {
            ok(player.hasPrevious());
            ok(await player.previous());
        }

        ok(!player.hasPrevious());
        ok(!await player.previous());
    });
});
