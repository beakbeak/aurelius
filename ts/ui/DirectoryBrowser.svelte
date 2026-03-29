<script lang="ts">
    import type { Player } from "../core/player";
    import type { PlayerState } from "./state/playerState.svelte";
    import type { DirState } from "./state/dirState.svelte";
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
        dirState: DirState;
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

    function onNavigate(url: string): void {
        dirState.loadDir(url);
    }

    function onFavoritesClick(): void {
        dirState.playFavorites(favoritesPrefix);
    }

    function onPlaylistClick(url: string): void {
        player.playList(url);
    }

    function onPlaylistRandomClick(url: string): void {
        player.playList(url, { random: true });
    }

    onMount(() => {
        dirState.loadDirFromPageUrl();

        const reloadOnFavorite = () => dirState.reloadCurrentDir();
        player.addEventListener("favorite", reloadOnFavorite);
        player.addEventListener("unfavorite", reloadOnFavorite);
        return () => {
            player.removeEventListener("favorite", reloadOnFavorite);
            player.removeEventListener("unfavorite", reloadOnFavorite);
        };
    });
</script>

<div class="dir">
    <!-- Favorites -->
    <ul class="dir__list">
        <li class="dir__row">
            <i class="dir__icon material-icons">favorite_border</i>
            <button class="dir__link" type="button" onclick={onFavoritesClick}>
                {favoritesText}
            </button>
        </li>
    </ul>

    <!-- Navigation -->
    {#if info}
        <DirList items={navigationItems} {onNavigate} />
    {/if}

    <!-- Directories -->
    {#if info && info.dirs.length > 0}
        <DirList items={dirItems} {onNavigate} />
    {/if}

    <!-- Playlists -->
    {#if info && info.playlists.length > 0}
        <ul class="dir__list">
            {#each info.playlists as playlist (playlist.url)}
                <li class="dir__row">
                    <i class="dir__icon material-icons">playlist_play</i>
                    <button
                        class="dir__link"
                        type="button"
                        onclick={() => onPlaylistClick(playlist.url)}
                    >
                        {playlist.name}
                    </button>
                    <button
                        class="dir__link dir__link--aux"
                        type="button"
                        onclick={() => onPlaylistRandomClick(playlist.url)}
                    >
                        random
                    </button>
                </li>
            {/each}
        </ul>
    {/if}

    <!-- Tracks -->
    {#if info && info.tracks.length > 0}
        <TrackList tracks={info.tracks} {playerState} {dirState} />
    {/if}
</div>
