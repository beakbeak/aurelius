<script lang="ts">
    import type { PlayerState } from "./PlayerState.svelte";
    import { onDrag } from "./dom";

    let {
        playerState,
        seekTime = $bindable(undefined),
    }: {
        playerState: PlayerState;
        seekTime?: number;
    } = $props();

    const player = $derived(playerState.player);
    let seekSliderPosition = $state<number | undefined>(undefined);
    let progressBarEmpty: HTMLElement | undefined = $state(undefined);

    const ariaValueNow = $derived.by(() => {
        const track = playerState.track;
        if (!track || playerState.duration <= 0) {
            return 0;
        }
        const currentTime =
            seekSliderPosition !== undefined
                ? seekSliderPosition * playerState.duration
                : playerState.currentTime;
        return Math.round((currentTime / playerState.duration) * 100);
    });

    function handleKeyDown(e: KeyboardEvent): void {
        if (!playerState.track) {
            return;
        }
        const step = 5;
        if (e.key === "ArrowLeft") {
            e.preventDefault();
            player.seekTo(Math.max(0, playerState.currentTime - step));
        } else if (e.key === "ArrowRight") {
            e.preventDefault();
            player.seekTo(Math.min(playerState.duration, playerState.currentTime + step));
        }
    }

    const seekLeft = $derived.by(() => {
        const track = playerState.track;
        if (!track) {
            return "0";
        }
        const duration = playerState.duration;
        const currentTime =
            seekSliderPosition !== undefined
                ? seekSliderPosition * duration
                : playerState.currentTime;
        if (duration > 0) {
            return `${(currentTime / duration) * 100}%`;
        }
        return "0";
    });

    const bufferLeft = $derived.by(() => {
        playerState.updateOnBufferProgress();
        const track = playerState.track;
        if (!track) {
            return "0";
        }
        const ranges = track.buffered();
        if (ranges.length > 0 && track.info.duration > 0) {
            const startTime = track.startTime + ranges.start(0);
            const left = startTime / track.info.duration;
            return `${Math.max(0, Math.min(100, left * 100))}%`;
        }
        return "0";
    });

    const bufferWidth = $derived.by(() => {
        playerState.updateOnBufferProgress();
        const track = playerState.track;
        if (!track) {
            return "0";
        }
        const ranges = track.buffered();
        if (ranges.length > 0 && track.info.duration > 0) {
            const startTime = track.startTime + ranges.start(0);
            const endTime = track.startTime + ranges.end(ranges.length - 1);
            const left = startTime / track.info.duration;
            const width = (endTime - startTime) / track.info.duration;
            const leftPercent = Math.max(0, Math.min(100, left * 100));
            const widthPercent = Math.max(0, Math.min(100 - leftPercent, width * 100));
            return `${widthPercent}%`;
        }
        return "0";
    });

    function startSeekSliderDrag(
        anchorClientX: number,
        anchorScreenX: number,
        touchId?: number,
    ): void {
        if (!playerState.track || !progressBarEmpty) {
            return;
        }
        const rect = progressBarEmpty.getBoundingClientRect();
        const anchorClientXOffset = anchorClientX - rect.left;

        const getPosition = (screenX: number): number => {
            let clientXOffset = anchorClientXOffset + (screenX - anchorScreenX);
            if (clientXOffset < 0) clientXOffset = 0;
            else if (clientXOffset > rect.width) clientXOffset = rect.width;
            return clientXOffset / rect.width;
        };

        seekSliderPosition = getPosition(anchorScreenX);
        seekTime = seekSliderPosition * playerState.duration;

        onDrag(
            (screenX) => {
                seekSliderPosition = getPosition(screenX);
                seekTime = seekSliderPosition * playerState.duration;
            },
            async (screenX) => {
                if (playerState.track !== undefined) {
                    // Wait for seek to complete before clearing slider/timestamp overrides.
                    // This prevents jumping back and forth between the seek time and current time.
                    try {
                        await player.seekTo(getPosition(screenX) * playerState.track.info.duration);
                    } finally {
                        seekSliderPosition = undefined;
                        seekTime = undefined;
                    }
                }
            },
            touchId,
        );
    }

    function handleMouseDown(event: MouseEvent): void {
        event.preventDefault();
        startSeekSliderDrag(event.clientX, event.screenX);
    }

    function handleTouchStart(event: TouchEvent): void {
        event.preventDefault();
        if (event.changedTouches.length > 0) {
            const touch = event.changedTouches[0];
            startSeekSliderDrag(touch.clientX, touch.screenX, touch.identifier);
        }
    }
</script>

<div class="controls__group" class:controls--disabled={!playerState.track}>
    <div
        bind:this={progressBarEmpty}
        class="controls__progress-trough"
        role="slider"
        aria-label="Seek"
        aria-valuemin="0"
        aria-valuemax="100"
        aria-valuenow={ariaValueNow}
        tabindex="0"
        onmousedown={handleMouseDown}
        ontouchstart={handleTouchStart}
        onkeydown={handleKeyDown}
    >
        <span class="controls__progress-fill" style:left={bufferLeft} style:width={bufferWidth}
        ></span>
        <span class="controls__slider-range">
            <span class="controls__slider" style:left={seekLeft}></span>
        </span>
    </div>
</div>

<style>
    .controls__progress-trough {
        cursor: pointer;
        flex: 1;
        position: relative;
        height: 0.5rem;
        box-shadow: inset 0 0 3px black;
        margin: 0 0.5rem;
    }
    .controls--disabled .controls__progress-trough {
        cursor: default;
    }

    .controls__progress-fill {
        position: absolute;
        top: 0;
        left: 0;
        width: 0;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.27);
    }

    .controls__slider-range {
        position: absolute;
        left: 0;
        width: calc(100% - 3rem);
        height: 100%;
    }
    .controls__slider {
        cursor: pointer;
        position: absolute;
        width: 3rem;
        height: 200%;
        top: -50%;
        background-color: hsl(0, 0%, 10%);
        border-radius: 0.25em;
    }
    .controls--disabled .controls__slider {
        cursor: default;
        background-color: hsl(0, 0%, 24%);
    }
</style>
