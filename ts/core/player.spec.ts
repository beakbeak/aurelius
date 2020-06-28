import { host, EventChecker } from "../testing";

import { Player } from "./player";
import { fetchTrackInfo } from "./track";
import { fetchDirInfo } from "./dir";

import { ok, strictEqual } from "assert";

const localPlaylistDir = `${host}/media/tree/`;
const remotePlaylistUrl = `${host}/media/tree/test.m3u`;
const trackUrl = `${host}/media/tree/test.flac`;

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

    it("seeks within a track", async function () {
        // HTMLMediaElement is not actually implemented in the testing context,
        // so some values are not checked and some expected behavior may be
        // unreliable if the test is run in a browser.

        const player = new Player();

        await player.seekTo(10);
        await player.playTrack(trackUrl);

        if (player.track === undefined) {
            throw new Error("player.track is undefined");
        }

        strictEqual(player.track.currentTime(), 0);
        await player.seekTo(10);
        strictEqual(player.track.currentTime(), 10);
    });

    function makePlaylistTests(
        playlistType: string,
        getUrls: () => Promise<string | string[]>
    ) {
        it(`cycles through a ${playlistType} playlist`, async function () {
            const player = new Player();

            const gotPlay = new EventChecker();
            player.addEventListener("play", () => { gotPlay.set(); });

            ok(await player.playList(await getUrls(), { startPos: 1 }));
            if (player.playlist === undefined) {
                throw new Error("player.playlist is undefined")
            }
            ok(gotPlay.consume());

            ok(player.hasPrevious());
            ok(await player.previous());
            ok(gotPlay.consume());

            ok(!player.hasPrevious());
            ok(!await player.previous());
            ok(!gotPlay.consume());

            for (let i = 1, end = player.playlist.length() >> 1; i < end; ++i) {
                ok(player.hasNext());
                ok(await player.next());
                ok(gotPlay.consume());
            }

            for (let i = 1, end = player.playlist.length() >> 1; i < end; ++i) {
                ok(player.hasPrevious());
                ok(await player.previous());
                ok(gotPlay.consume());
            }

            ok(!player.hasPrevious());
            ok(!await player.previous());
            ok(!gotPlay.consume());

            for (let i = 1, end = player.playlist.length(); i < end; ++i) {
                ok(player.hasNext());
                ok(await player.next());
                ok(gotPlay.consume());
            }

            ok(!player.hasNext());
            ok(!await player.next());
            ok(!gotPlay.consume());

            for (let i = 1, end = player.playlist.length(); i < end; ++i) {
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

        it(`plays a ${playlistType} playlist in random order`, async function () {
            const player = new Player();

            ok(await player.playList(await getUrls(), { random: true }));
            if (player.playlist === undefined) {
                throw new Error("player.playlist is undefined")
            }

            ok(!player.hasPrevious());
            ok(!await player.previous());

            const moreThanOriginalLength = player.playlist.length() + 2;
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
    }

    makePlaylistTests("remote", async () => remotePlaylistUrl);
    makePlaylistTests("local", async () => {
        const dirInfo = await fetchDirInfo(localPlaylistDir);
        return dirInfo.tracks.map(pathUrl => pathUrl.url);
    });
});
