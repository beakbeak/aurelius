import { searchMedia, SearchResult } from "../core/search";
import { hideModalDialog, showModalDialog } from "./modal";
import { loadDir, playTrackByUrl } from "./dir";
import { Class } from "./class";

let searchDialog: HTMLElement;
let searchInput: HTMLInputElement;
let searchResults: HTMLElement;

let searchTimeout: ReturnType<typeof setTimeout> | undefined;
let searchSequence = 0;
let focusedResultIndex = -1;
let currentResults: SearchResult[] = [];

export function showSearchDialog(): void {
    ensureElements();
    clearSearch();
    showModalDialog(searchDialog);
    searchInput.focus();
}

function ensureElements(): void {
    if (searchDialog) {
        return;
    }

    searchDialog = document.getElementById("search-dialog")!;
    searchInput = document.getElementById("search-input") as HTMLInputElement;
    searchResults = document.getElementById("search-results")!;

    searchInput.addEventListener("input", () => {
        const query = searchInput.value.trim();

        if (searchTimeout) {
            clearTimeout(searchTimeout);
        }

        if (query === "") {
            clearSearch();
            return;
        }

        searchTimeout = setTimeout(() => {
            performSearch(query);
        }, 300);
    });

    searchInput.addEventListener("keydown", handleResultNavigation);
    searchResults.addEventListener("keydown", handleResultNavigation);

    searchResults.addEventListener("click", (e) => {
        const target = e.target as HTMLElement;
        const resultElement = target.closest(`.${Class.SearchResult}`);
        if (resultElement) {
            const index = Array.from(searchResults.children).indexOf(resultElement);
            if (index >= 0 && index < currentResults.length) {
                activateResult(currentResults[index]);
            }
        }
    });
}

function handleResultNavigation(e: KeyboardEvent): void {
    if (e.key === "ArrowDown") {
        e.preventDefault();
        moveResultFocus(1);
    } else if (e.key === "ArrowUp") {
        e.preventDefault();
        moveResultFocus(-1);
    } else if (e.key === "Enter") {
        e.preventDefault();
        if (focusedResultIndex >= 0 && focusedResultIndex < currentResults.length) {
            activateResult(currentResults[focusedResultIndex]);
        }
    }
}

function moveResultFocus(direction: number): void {
    if (currentResults.length === 0) {
        return;
    }

    const newIndex = focusedResultIndex + direction;
    if (newIndex >= 0 && newIndex < currentResults.length) {
        setFocusedResultIndex(newIndex);
    } else if (newIndex < 0) {
        // Move focus back to search input
        setFocusedResultIndex(-1);
        searchInput.focus();
    }
}

function setFocusedResultIndex(index: number): void {
    // Remove focus from previous result
    if (focusedResultIndex >= 0) {
        const prevResult = searchResults.children[focusedResultIndex] as HTMLElement;
        prevResult?.classList.remove(Class.SearchResult_Focused);
    }

    focusedResultIndex = index;

    // Add focus to new result
    if (focusedResultIndex >= 0) {
        const newResult = searchResults.children[focusedResultIndex] as HTMLElement;
        newResult?.classList.add(Class.SearchResult_Focused);
        newResult?.focus();
        newResult?.scrollIntoView({ block: "nearest" });
    }
}

async function performSearch(query: string): Promise<void> {
    const seq = ++searchSequence;
    try {
        const response = await searchMedia(query);
        if (seq !== searchSequence) return; // Discard stale response
        currentResults = response.results;
        displayResults(response.results);
    } catch (error) {
        if (seq !== searchSequence) return;
        console.error("Search failed:", error);
        displayError("Search failed. Please try again.");
    }
}

function displayResults(results: SearchResult[]): void {
    searchResults.replaceChildren();
    focusedResultIndex = -1;

    if (results.length === 0) {
        const noResults = document.createElement("div");
        noResults.className = "search__no-results";
        noResults.textContent = "No results found";
        searchResults.appendChild(noResults);
        return;
    }

    for (const result of results) {
        const resultElement = document.createElement("div");
        resultElement.className = `${Class.SearchResult} ${Class.DirRow}`;
        resultElement.tabIndex = 0; // Make focusable

        const icon = document.createElement("i");
        icon.className = `${Class.DirIcon} ${Class.MaterialIcons}`;
        if (result.type === "dir") {
            icon.textContent = "folder_open";
        } else if (result.track?.favorite) {
            icon.textContent = "favorite_border";
        } else {
            icon.textContent = "music_note";
        }

        const textElement = document.createElement("span");
        textElement.className = Class.DirLink;
        textElement.textContent = formatSearchResult(result);

        resultElement.appendChild(icon);
        resultElement.appendChild(textElement);
        searchResults.appendChild(resultElement);
    }
}

function formatSearchResult(result: SearchResult): string {
    if (result.track) {
        const title = result.track.tags["title"];
        const artist = result.track.tags["artist"];
        if (title && artist) {
            const album = result.track.tags["album"];
            return album ? `${title} \u00b7 ${artist} \u00b7 ${album}` : `${title} \u00b7 ${artist}`;
        }
    }
    return result.path;
}

function displayError(message: string): void {
    const errorDiv = document.createElement("div");
    errorDiv.className = "search__error";
    errorDiv.textContent = message;
    searchResults.replaceChildren(errorDiv);
    currentResults = [];
    focusedResultIndex = -1;
}

function clearSearch(): void {
    searchInput.value = "";
    searchResults.replaceChildren();
    currentResults = [];
    focusedResultIndex = -1;

    if (searchTimeout) {
        clearTimeout(searchTimeout);
        searchTimeout = undefined;
    }
}

async function activateResult(result: SearchResult): Promise<void> {
    hideModalDialog();

    if (result.type === "dir") {
        await loadDir(result.url);
    } else if (result.type === "track") {
        await playTrackByUrl(result.url);
    }
}
