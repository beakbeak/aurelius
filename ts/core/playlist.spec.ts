import { host } from "../testing";

import { Playlist, LocalPlaylist, RemotePlaylist } from "./playlist";
import { fetchDirInfo } from "./dir";
import { fetchTrackInfo } from "./track";

import { ok, strictEqual } from "assert";

function makeCommonTests(
    suiteName: string,
    makePlaylist: () => Promise<Playlist>,
) {
    describe(suiteName, function () {
        it("can access all elements", async function () {
            const playlist = await makePlaylist();

            const length = playlist.length();
            ok(playlist.length() > 0);

            for (let i = 0; i < length; ++i) {
                const item = await playlist.at(i);
                if (item === undefined) {
                    throw new Error("item is undefined");
                }
                strictEqual(item.pos, i);

                await fetchTrackInfo(`${host}${item.path}`);
            }
        });

        it("returns a random element", async function () {
            const playlist = await makePlaylist();

            const item = await playlist.random();
            if (item === undefined) {
                throw new Error("item is undefined");
            }

            await fetchTrackInfo(`${host}${item.path}`);
        });
    });
}

async function makeLocalPlaylist(dbPath = ""): Promise<LocalPlaylist> {
    const dir = await fetchDirInfo(`${host}/db/${dbPath}`);
    return new LocalPlaylist(dir.tracks.map(pathUrl => pathUrl.url));
}

async function makeRemotePlaylist(dbPath = "test.m3u"): Promise<RemotePlaylist> {
    return RemotePlaylist.fetch(`${host}/db/${dbPath}`);
}

makeCommonTests("LocalPlaylist", makeLocalPlaylist);
makeCommonTests("RemotePlaylist", makeRemotePlaylist);
