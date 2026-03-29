<script lang="ts">
    import type { TrackInfo } from "../core/track";
    import type { Player } from "../core/player";
    import type { PlayerState } from "./state/playerState.svelte";
    import { formatTrackTitle, formatTrackArtist, formatTrackMeta, formatDuration } from "./format";

    let {
        tracks,
        player,
        playerState,
        dirState,
    }: {
        tracks: TrackInfo[];
        player: Player;
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
    {#each discGroups as group}
        {#if group.disc}
            <li class="dir__disc-header">Disc {group.disc}</li>
        {/if}
        {#each group.tracks as { track, index }}
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
