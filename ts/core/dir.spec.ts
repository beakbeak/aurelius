import { host } from "../testing";

import { fetchDirInfo, dirUrlFromTreeUrl, treeUrlFromDirInfo, DirInfo } from "./dir";

import { ok, strictEqual } from "assert";

function dirInfoFromPath(path: string): DirInfo {
    return {
        path,
        topLevel: "",
        parent: "",
        dirs: [],
        playlists: [],
        tracks: [],
    };
}

describe("directory listing", function () {
    it("succeeds and contains 'test.flac'", async function () {
        const dir = await fetchDirInfo(dirUrlFromTreeUrl(`${host}/media/tree/`));

        let found = false;
        for (const track of dir.tracks) {
            if (track.name === "test.flac") {
                found = true;
            }
        }
        ok(found);
    });
});

describe("URL conversion functions", function () {
    it("converts tree URL to dir URL", function () {
        strictEqual(dirUrlFromTreeUrl(`${host}/media/tree/`), `/media/dirs/at:`);
        strictEqual(
            dirUrlFromTreeUrl(`${host}/media/tree/foo/bar%20baz/`),
            `/media/dirs/at:foo%2Fbar%20baz`,
        );
    });

    it("converts DirInfo to tree URL", function () {
        const rootDirInfo = dirInfoFromPath("");
        const subDirInfo = dirInfoFromPath("foo/bar");

        strictEqual(treeUrlFromDirInfo(rootDirInfo), `/media/tree/`);
        strictEqual(treeUrlFromDirInfo(subDirInfo), `/media/tree/foo/bar`);
    });
});
