import { fetchJson } from "./json";
import { TrackInfo } from "./track";

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
    return fetchJson<SearchResponse>(url);
}
