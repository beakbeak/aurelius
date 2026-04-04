<script lang="ts">
    import type { SearchResult } from "../core/search";
    import { searchMedia } from "../core/search";
    import "./dir.css";
    import type { DirState } from "./DirState.svelte";
    import Modal from "./Modal.svelte";

    let {
        open = $bindable(false),
        dirState,
    }: {
        open: boolean;
        dirState: DirState;
    } = $props();

    let query = $state("");
    let results = $state<SearchResult[]>([]);
    let focusedIndex = $state(-1);
    let errorMessage = $state("");
    let loading = $state(false);
    let searchTimeout: ReturnType<typeof setTimeout> | undefined;
    let searchSequence = 0;
    let searchInput: HTMLInputElement | undefined = $state(undefined);
    let resultsContainer: HTMLElement | undefined = $state(undefined);

    function clearSearch(): void {
        query = "";
        results = [];
        focusedIndex = -1;
        errorMessage = "";
        loading = false;
        if (searchTimeout) {
            clearTimeout(searchTimeout);
            searchTimeout = undefined;
        }
    }

    async function performSearch(q: string): Promise<void> {
        const seq = ++searchSequence;
        try {
            const response = await searchMedia(q);
            if (seq !== searchSequence) {
                return;
            }
            results = response.results ?? [];
            errorMessage = "";
            focusedIndex = -1;
        } catch (error) {
            if (seq !== searchSequence) {
                return;
            }
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
        loading = true;
        searchTimeout = setTimeout(async () => {
            await performSearch(q);
            loading = false;
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
        open = false;
        if (result.type === "dir") {
            await dirState.loadDir(result.url);
        } else if (result.type === "track") {
            await dirState.playTrackByUrl(result.url);
        }
    }

    function getResultIcon(result: SearchResult): string {
        if (result.type === "dir") {
            return "folder_open";
        }
        if (result.track?.favorite) {
            return "favorite_border";
        }
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
        if (!result.track) {
            return false;
        }
        const title = result.track.tags["title"];
        const artist = result.track.tags["artist"];
        return !!(title && artist);
    }

    $effect(() => {
        if (open) {
            clearSearch();
            searchInput?.focus();
        }
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

<Modal bind:open>
    <div class="modal-box max-h-3/4 max-w-3xl w-auto">
        <h2 class="text-lg font-bold">Search</h2>

        <label class="input w-full my-2">
            <i class="material-icons">search</i>
            <input
                bind:this={searchInput}
                bind:value={query}
                type="text"
                placeholder="Type to search..."
                autocomplete="off"
                oninput={handleInput}
                onkeydown={handleKeydown}
            />
        </label>

        <div
            class="max-h-128 overflow-y-auto relative"
            class:search-results-container--loading={loading}
        >
            {#if loading}
                <div class="search-loading-overlay">
                    <span class="loading loading-spinner loading-md"></span>
                </div>
            {/if}
            <div
                bind:this={resultsContainer}
                class="search-results dir__list"
                class:opacity-50={loading}
                role="listbox"
            >
                {#if errorMessage}
                    <div class="text-error text-center">{errorMessage}</div>
                {:else if !loading && query.trim() !== "" && results.length === 0}
                    <div class="opacity-70 text-center">No results found</div>
                {:else}
                    {#each results as result, i (result.url)}
                        <div
                            class="search-result dir__row"
                            class:search-result--focused={i === focusedIndex}
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
                                    {getResultTitle(result)}<span class="search-result-detail"
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
    </div>
</Modal>

<style>
    .search-result {
        cursor: pointer;
    }

    .search-result :global(.dir__link) {
        color: inherit;
    }

    .search-result-detail {
        font-style: italic;
        opacity: 0.7;
    }

    .search-results-container--loading {
        min-height: 3rem;
    }

    .search-loading-overlay {
        position: absolute;
        inset: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        background: oklch(var(--b1) / 0.5);
        z-index: 1;
    }
</style>
