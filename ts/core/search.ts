import { fetchJson as _fetchJson } from "./json";
import type { TrackInfo } from "./track";

export const deps = { fetchJson: _fetchJson as <T>(url: string) => Promise<T> };

export interface SearchResult {
    path: string;
    type: "dir" | "track";
    url: string;
    track?: TrackInfo;
}

export interface SearchResponse {
    results: SearchResult[];
}

export async function searchMedia(query: string): Promise<SearchResponse> {
    const url = `/media/search?q=${encodeURIComponent(query)}`;
    return deps.fetchJson<SearchResponse>(url);
}
