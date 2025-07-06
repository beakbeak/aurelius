import { searchMedia } from "./search";
import * as jsonModule from "./json";
import { strictEqual, deepStrictEqual } from "assert";

describe("search", () => {
    let originalFetchJson: typeof jsonModule.fetchJson;

    beforeEach(() => {
        originalFetchJson = jsonModule.fetchJson;
    });

    afterEach(() => {
        // Restore original function
        (jsonModule as any).fetchJson = originalFetchJson;
    });

    describe("searchMedia", () => {
        it("calls fetchJson with correct URL for simple query", async () => {
            const mockResponse = {
                results: [
                    { path: "test.mp3", type: "track" as const, url: "/media/tracks/test.mp3" }
                ],
                total: 1
            };
            
            let calledUrl = "";
            (jsonModule as any).fetchJson = async (url: string) => {
                calledUrl = url;
                return mockResponse;
            };

            const result = await searchMedia("test");

            strictEqual(calledUrl, "/media/search?q=test");
            deepStrictEqual(result, mockResponse);
        });

        it("URL encodes query parameters", async () => {
            const mockResponse = { results: [], total: 0 };
            
            let calledUrl = "";
            (jsonModule as any).fetchJson = async (url: string) => {
                calledUrl = url;
                return mockResponse;
            };

            await searchMedia("test with spaces & special chars");

            strictEqual(calledUrl, "/media/search?q=test%20with%20spaces%20%26%20special%20chars");
        });

        it("handles empty query", async () => {
            const mockResponse = { results: [], total: 0 };
            
            let calledUrl = "";
            (jsonModule as any).fetchJson = async (url: string) => {
                calledUrl = url;
                return mockResponse;
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
                total: 2
            };
            
            (jsonModule as any).fetchJson = async () => mockResponse;

            const result = await searchMedia("query");

            strictEqual(result.results.length, 2);
            strictEqual(result.results[0].type, "dir");
            strictEqual(result.results[1].type, "track");
            strictEqual(result.total, 2);
        });

        it("propagates errors from fetchJson", async () => {
            const error = new Error("Network error");
            
            (jsonModule as any).fetchJson = async () => {
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
