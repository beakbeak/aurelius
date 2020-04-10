import { host } from "../test-setup";
import { strict as assert } from "assert";
import { fetchDirInfo } from "./dir";

describe("directory listing", function () {
    it("succeeds and contains 'test.flac'", async function () {
        const dir = await fetchDirInfo(`${host}/`);

        let found = false;
        for (const track of dir.tracks) {
            if (track.name === "test.flac") {
                found = true;
            }
        }
        assert.ok(found);
    });
});
