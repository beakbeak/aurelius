<script lang="ts">
    import type { Player } from "../core/player";
    import type { PlayerState } from "./state/playerState.svelte";
    import type { DirInfo } from "../core/dir";
    import { onMount } from "svelte";
    import TrackList from "./TrackList.svelte";
    import DirList from "./DirList.svelte";
    import "./dir.css";

    let {
        player,
        playerState,
        dirState,
    }: {
        player: Player;
        playerState: PlayerState;
        dirState: ReturnType<typeof import("./state/dirState.svelte").createDirState>;
    } = $props();

    let info = $derived(dirState.dirInfo);

    let favoritesText = $derived.by(() => {
        if (!info || info.path === "/") return "Favorites";
        const pathParts = info.path.replace(/\/$/, "").split("/");
        const dirName = pathParts[pathParts.length - 1];
        return `Favorites in ${dirName}/`;
    });

    let favoritesPrefix = $derived.by(() => {
        if (!info || info.path === "/") return undefined;
        return info.path;
    });

    let navigationItems = $derived.by(() => {
        if (!info) return [];
        return [
            {
                name: "Top level",
                url: info.topLevel,
                icon: "vertical_align_top",
                href: `/media/tree/?dir=${encodeURIComponent(info.topLevel)}`,
            },
            {
                name: "Parent directory",
                url: info.parent,
                icon: "arrow_back",
                href: `/media/tree/?dir=${encodeURIComponent(info.parent)}`,
            },
        ];
    });

    let dirItems = $derived.by(() => {
        if (!info) return [];
        return info.dirs.map((dir) => ({
            name: `${dir.name}/`,
            url: dir.url,
            icon: "folder_open",
            href: `/media/tree/?dir=${encodeURIComponent(dir.url)}`,
        }));
    });

    function handleNavigate(url: string): void {
        dirState.loadDir(url);
    }

    function handleFavoritesClick(): void {
        dirState.playFavorites(favoritesPrefix);
    }

    function handlePlaylistClick(url: string): void {
        player.playList(url);
    }

    function handlePlaylistRandomClick(url: string): void {
        player.playList(url, { random: true });
    }

    onMount(() => {
        dirState.loadDirFromPageUrl();
    });

    // Listen for favorite/unfavorite to reload
    $effect(() => {
        const reloadOnFav = () => dirState.reloadCurrentDir();
        player.addEventListener("favorite", reloadOnFav);
        player.addEventListener("unfavorite", reloadOnFav);
        return () => {
            // Note: Player doesn't have removeEventListener, so these persist
        };
    });
</script>

<main class="dir main__dir">
    <!-- Special list (Favorites) -->
    <ul class="dir__list">
        <li class="dir__row">
            <i class="dir__icon material-icons">favorite_border</i>
            <button class="dir__link" type="button" onclick={handleFavoritesClick}>
                {favoritesText}
            </button>
        </li>
    </ul>

    <!-- Navigation list -->
    {#if info}
        <DirList items={navigationItems} onNavigate={handleNavigate} />
    {/if}

    <!-- Directories list -->
    {#if info && info.dirs.length > 0}
        <DirList items={dirItems} onNavigate={handleNavigate} />
    {/if}

    <!-- Playlists list -->
    {#if info && info.playlists.length > 0}
        <ul class="dir__list">
            {#each info.playlists as playlist (playlist.url)}
                <li class="dir__row">
                    <i class="dir__icon material-icons">playlist_play</i>
                    <button
                        class="dir__link"
                        type="button"
                        onclick={() => handlePlaylistClick(playlist.url)}
                    >
                        {playlist.name}
                    </button>
                    <button
                        class="dir__link dir__link--aux"
                        type="button"
                        onclick={() => handlePlaylistRandomClick(playlist.url)}
                    >
                        random
                    </button>
                </li>
            {/each}
        </ul>
    {/if}

    <!-- Track list -->
    {#if info && info.tracks.length > 0}
        <TrackList
            tracks={info.tracks}
            {player}
            {playerState}
            {dirState}
        />
    {/if}
</main>
