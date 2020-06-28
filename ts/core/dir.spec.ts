import { host } from "../testing";

import { fetchDirInfo } from "./dir";

import { ok } from "assert";

describe("directory listing", function () {
    it("succeeds and contains 'test.flac'", async function () {
        const dir = await fetchDirInfo(`${host}/media/tree/`);

        let found = false;
        for (const track of dir.tracks) {
            if (track.name === "test.flac") {
                found = true;
            }
        }
        ok(found);
    });
});
