import { host } from "../testing";

import { Player } from "./player";
import { fetchTrackInfo } from "./track";

import { ok } from "assert";

describe("Player", function () {
    it("can favorite/unfavorite", async function () {
        const url = `${host}/db/test.flac`;

        const player = new Player();
        await player.playTrack(url);

        await player.unfavorite();
        ok((await fetchTrackInfo(url)).favorite === false);

        await player.favorite();
        ok((await fetchTrackInfo(url)).favorite === true);

        await player.unfavorite();
        ok((await fetchTrackInfo(url)).favorite === false);
    });
});
