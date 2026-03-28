<script lang="ts">
    import type { TrackInfo } from "../core/track";
    import type { PlayerState } from "./state/playerState.svelte";
    import { formatTrackTitle, formatTrackArtist, formatTrackMeta, formatDuration } from "./format";

    let {
        tracks,
        playerState,
        dirState,
    }: {
        tracks: TrackInfo[];
        playerState: PlayerState;
        dirState: ReturnType<typeof import("./state/dirState.svelte").createDirState>;
    } = $props();

    interface DiscGroup {
        disc: string | undefined;
        startIndex: number;
        tracks: { track: TrackInfo; index: number }[];
    }

    let discGroups = $derived.by(() => {
        const groups: DiscGroup[] = [];
        let currentDisc: string | undefined = undefined;
        let currentGroup: DiscGroup | undefined = undefined;

        for (let i = 0; i < tracks.length; i++) {
            const track = tracks[i];
            const disc = track.tags["disc"];

            if (disc !== currentDisc || !currentGroup) {
                currentGroup = {
                    disc: disc !== currentDisc ? disc : undefined,
                    startIndex: i,
                    tracks: [],
                };
                groups.push(currentGroup);
                currentDisc = disc;
            }
            currentGroup.tracks.push({ track, index: i });
        }

        return groups;
    });

    function isPlaying(trackUrl: string): boolean {
        if (!playerState.track) return false;
        return playerState.track.url.endsWith(trackUrl);
    }

    function handleTrackClick(e: MouseEvent, index: number): void {
        if (e.metaKey || e.ctrlKey) return;
        e.preventDefault();
        dirState.playTrackByIndex(index);
    }
</script>

<ul class="dir__list dir__track-list">
    {#each discGroups as group (group.startIndex)}
        {#if group.disc}
            <li class="dir__disc-header">Disc {group.disc}</li>
        {/if}
        {#each group.tracks as { track, index } (track.url)}
            <li class="dir__row" class:dir__row--playing={isPlaying(track.url)}>
                <i class="dir__icon material-icons">
                    {track.favorite ? "favorite_border" : "music_note"}
                </i>
                <i class="dir__icon dir__icon--playing material-icons"> play_arrow </i>
                <span class="dir__cell-num">{index + 1}</span>
                <a
                    class="dir__link"
                    href="/media/tree/?play={encodeURIComponent(track.url)}"
                    data-url={track.url}
                    onclick={(e: MouseEvent) => handleTrackClick(e, index)}
                >
                    {formatTrackTitle(track)}
                </a>
                <span class="dir__cell-duration">
                    {formatDuration(track.duration)}
                </span>
                <span class="dir__cell-artist">
                    {formatTrackArtist(track)}
                </span>
                <span class="dir__cell-detail">
                    {formatTrackMeta(track)}
                </span>
            </li>
        {/each}
    {/each}
</ul>

<style>
    .dir__track-list {
        display: grid;
        grid-template-columns: auto auto minmax(0, 1fr) auto auto auto;
        column-gap: 0.75em;
    }
    .dir__track-list > :global(.dir__row) {
        display: grid;
        grid-column: 1 / -1;
        grid-template-columns: subgrid;
        align-items: center;
    }

    /* Dot leaders: subtle dots connecting track titles to right-aligned metadata */
    .dir__track-list :global(.dir__link) {
        display: flex;
        overflow: hidden;
    }
    .dir__track-list :global(.dir__link)::after {
        content: "";
        flex: 1;
        margin-left: 1em;
        margin-bottom: 0.15em;
        background-image: radial-gradient(circle, hsla(0, 0%, 100%, 0.07) 1px, transparent 1px);
        background-size: 10px 3px;
        background-position: bottom;
        background-repeat: repeat-x;
    }

    .dir__cell-num {
        white-space: nowrap;
        color: hsl(0, 0%, 50%);
        margin-left: -0.5em;
    }
    .dir__cell-duration {
        white-space: nowrap;
        color: hsl(0, 0%, 50%);
        font-size: 0.85em;
    }
    .dir__cell-artist {
        color: hsl(0, 0%, 50%);
        font-size: 0.85em;
        font-style: italic;
        margin-right: 0.75em;
    }
    .dir__cell-detail {
        white-space: nowrap;
        color: hsl(0, 0%, 50%);
        font-size: 0.85em;
    }
    .dir__disc-header {
        grid-column: 1 / -1;
        color: hsl(0, 0%, 50%);
        font-size: 0.85em;
        font-style: italic;
        padding: 0.5em 0 0.25em;
    }
    :global(.dir__row--playing) .dir__cell-num,
    :global(.dir__row--playing) .dir__cell-duration,
    :global(.dir__row--playing) .dir__cell-artist,
    :global(.dir__row--playing) .dir__cell-detail {
        color: white;
    }

    @media (max-width: 959px) {
        .dir__track-list {
            grid-template-columns: auto auto minmax(0, 1fr) auto auto;
        }
        .dir__cell-detail {
            display: none;
        }
        :global(.dir__row--playing) .dir__cell-detail {
            display: block;
            grid-column: 3 / -1;
        }
    }

    @media (max-width: 530px) {
        .dir__track-list {
            grid-template-columns: auto auto minmax(0, 1fr) auto;
        }
        .dir__cell-artist {
            grid-column: 3 / -1;
        }
        :global(.dir__row--playing) .dir__cell-detail {
            grid-column: 3 / -1;
        }
    }
</style>
