<script lang="ts">
    import type { SearchResult } from "../core/search";
    import { searchMedia } from "../core/search";
    import { onMount } from "svelte";
    import "./dir.css";
    import type { DirState } from "./state/dirState.svelte";

    let {
        dirState,
        onClose,
    }: {
        dirState: DirState;
        onClose: () => void;
    } = $props();

    let query = $state("");
    let results = $state<SearchResult[]>([]);
    let focusedIndex = $state(-1);
    let errorMessage = $state("");
    let searchTimeout: ReturnType<typeof setTimeout> | undefined;
    let searchSequence = 0;
    let searchInput: HTMLInputElement | undefined = $state(undefined);
    let resultsContainer: HTMLElement | undefined = $state(undefined);

    function clearSearch(): void {
        query = "";
        results = [];
        focusedIndex = -1;
        errorMessage = "";
        if (searchTimeout) {
            clearTimeout(searchTimeout);
            searchTimeout = undefined;
        }
    }

    async function performSearch(q: string): Promise<void> {
        const seq = ++searchSequence;
        try {
            const response = await searchMedia(q);
            if (seq !== searchSequence) return;
            results = response.results;
            errorMessage = "";
            focusedIndex = -1;
        } catch (error) {
            if (seq !== searchSequence) return;
            console.error("Search failed:", error);
            results = [];
            errorMessage = "Search failed. Please try again.";
            focusedIndex = -1;
        }
    }

    function handleInput(): void {
        const q = query.trim();
        if (searchTimeout) {
            clearTimeout(searchTimeout);
        }
        if (q === "") {
            clearSearch();
            return;
        }
        searchTimeout = setTimeout(() => {
            performSearch(q);
        }, 300);
    }

    function handleKeydown(e: KeyboardEvent): void {
        if (e.key === "ArrowDown") {
            e.preventDefault();
            if (results.length > 0 && focusedIndex < results.length - 1) {
                focusedIndex++;
            }
        } else if (e.key === "ArrowUp") {
            e.preventDefault();
            if (focusedIndex > 0) {
                focusedIndex--;
            } else if (focusedIndex === 0) {
                focusedIndex = -1;
                searchInput?.focus();
            }
        } else if (e.key === "Enter") {
            e.preventDefault();
            if (focusedIndex >= 0 && focusedIndex < results.length) {
                activateResult(results[focusedIndex]);
            }
        }
    }

    function handleResultClick(index: number): void {
        if (index >= 0 && index < results.length) {
            activateResult(results[index]);
        }
    }

    async function activateResult(result: SearchResult): Promise<void> {
        onClose();
        if (result.type === "dir") {
            await dirState.loadDir(result.url);
        } else if (result.type === "track") {
            await dirState.playTrackByUrl(result.url);
        }
    }

    function getResultIcon(result: SearchResult): string {
        if (result.type === "dir") return "folder_open";
        if (result.track?.favorite) return "favorite_border";
        return "music_note";
    }

    function getResultTitle(result: SearchResult): string {
        if (result.track) {
            const title = result.track.tags["title"];
            if (title) return title;
        }
        return result.path;
    }

    function getResultDetail(result: SearchResult): string {
        if (result.track) {
            const title = result.track.tags["title"];
            const artist = result.track.tags["artist"];
            if (title && artist) {
                const album = result.track.tags["album"];
                return album ? ` \u00b7 ${artist} \u00b7 ${album}` : ` \u00b7 ${artist}`;
            }
        }
        return "";
    }

    function hasDetailText(result: SearchResult): boolean {
        if (!result.track) return false;
        const title = result.track.tags["title"];
        const artist = result.track.tags["artist"];
        return !!(title && artist);
    }

    onMount(() => {
        clearSearch();
        searchInput?.focus();
    });

    // Focus and scroll focused result into view
    $effect(() => {
        if (focusedIndex >= 0 && resultsContainer) {
            const el = resultsContainer.children[focusedIndex];
            if (el instanceof HTMLElement) {
                el.focus();
                el.scrollIntoView({ block: "nearest" });
            }
        }
    });
</script>

<div class="ui__section-header">Search</div>
<div class="ui__section-body">
    <input
        bind:this={searchInput}
        bind:value={query}
        class="search__input ui__entry-input"
        type="text"
        placeholder="Type to search..."
        autocomplete="off"
        oninput={handleInput}
        onkeydown={handleKeydown}
    />
</div>
<div class="ui__section-body ui__section-body--scroll">
    <div bind:this={resultsContainer} class="search__results dir__list" role="listbox">
        {#if errorMessage}
            <div class="search__error">{errorMessage}</div>
        {:else if query.trim() !== "" && results.length === 0}
            <div class="search__no-results">No results found</div>
        {:else}
            {#each results as result, i (result.url)}
                <div
                    class="search__result dir__row"
                    class:search__result--focused={i === focusedIndex}
                    role="option"
                    aria-selected={i === focusedIndex}
                    tabindex="0"
                    onclick={() => handleResultClick(i)}
                    onkeydown={handleKeydown}
                >
                    <i class="dir__icon material-icons">
                        {getResultIcon(result)}
                    </i>
                    <span class="dir__link">
                        {#if hasDetailText(result)}
                            {getResultTitle(result)}<span class="search__result-detail"
                                >{getResultDetail(result)}</span
                            >
                        {:else}
                            {result.path}
                        {/if}
                    </span>
                </div>
            {/each}
        {/if}
    </div>
</div>

<style>
    .search__result {
        cursor: pointer;
    }

    .search__result :global(.dir__link) {
        color: inherit;
    }

    .search__result-detail {
        font-style: italic;
        opacity: 0.7;
    }

    .ui__section-body {
        max-height: 30em;
    }
</style>
