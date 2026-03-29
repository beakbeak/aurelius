import { searchMedia, deps } from "./search";
import { strictEqual, deepStrictEqual } from "assert";

describe("search", () => {
    let originalFetchJson: typeof deps.fetchJson;

    beforeEach(() => {
        originalFetchJson = deps.fetchJson;
    });

    afterEach(() => {
        deps.fetchJson = originalFetchJson;
    });

    describe("searchMedia", () => {
        it("calls fetchJson with correct URL for simple query", async () => {
            const mockResponse = {
                results: [
                    { path: "test.mp3", type: "track" as const, url: "/media/tracks/test.mp3" }
                ],
            };

            let calledUrl = "";
            deps.fetchJson = async (url: string) => {
                calledUrl = url;
                return mockResponse as any;
            };

            const result = await searchMedia("test");

            strictEqual(calledUrl, "/media/search?q=test");
            deepStrictEqual(result, mockResponse);
        });

        it("URL encodes query parameters", async () => {
            const mockResponse = { results: [] };

            let calledUrl = "";
            deps.fetchJson = async (url: string) => {
                calledUrl = url;
                return mockResponse as any;
            };

            await searchMedia("test with spaces & special chars");

            strictEqual(calledUrl, "/media/search?q=test%20with%20spaces%20%26%20special%20chars");
        });

        it("handles empty query", async () => {
            const mockResponse = { results: [] };

            let calledUrl = "";
            deps.fetchJson = async (url: string) => {
                calledUrl = url;
                return mockResponse as any;
            };

            const result = await searchMedia("");

            strictEqual(calledUrl, "/media/search?q=");
            deepStrictEqual(result, mockResponse);
        });

        it("returns search results with correct types", async () => {
            const mockResponse = {
                results: [
                    { path: "foo", type: "dir" as const, url: "/media/dirs/foo" },
                    { path: "bar.mp3", type: "track" as const, url: "/media/tracks/bar.mp3" }
                ],
            };

            deps.fetchJson = async () => mockResponse as any;

            const result = await searchMedia("query");

            strictEqual(result.results.length, 2);
            strictEqual(result.results[0].type, "dir");
            strictEqual(result.results[1].type, "track");
        });

        it("propagates errors from fetchJson", async () => {
            const error = new Error("Network error");

            deps.fetchJson = async () => {
                throw error;
            };

            try {
                await searchMedia("test");
                throw new Error("Expected error to be thrown");
            } catch (err: any) {
                strictEqual(err.message, "Network error");
            }
        });
    });
});
