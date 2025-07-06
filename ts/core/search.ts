import { fetchJson } from "./json";

export interface SearchResult {
    path: string;
    type: "dir" | "track";
    url: string;
}

export interface SearchResponse {
    results: SearchResult[];
    total: number;
}

export async function searchMedia(query: string): Promise<SearchResponse> {
    const url = `/media/search?q=${encodeURIComponent(query)}`;
    return fetchJson<SearchResponse>(url);
}
