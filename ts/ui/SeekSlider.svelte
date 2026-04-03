<script lang="ts">
    import { onDrag } from "./dom";

    let {
        value,
        disabled = false,
        bufferLeft = 0,
        bufferWidth = 0,
        seekValue = $bindable(undefined),
        onseek,
        keyboardStep = 0,
    }: {
        value: number;
        disabled?: boolean;
        bufferLeft?: number;
        bufferWidth?: number;
        seekValue?: number;
        onseek: (position: number) => Promise<void> | void;
        keyboardStep?: number;
    } = $props();

    let progressBarEmpty: HTMLElement | undefined = $state(undefined);

    const ariaValueNow = $derived(Math.round((seekValue ?? value) * 100));

    function handleKeyDown(e: KeyboardEvent): void {
        if (disabled) {
            return;
        }
        if (e.key === "ArrowLeft") {
            e.preventDefault();
            onseek(Math.max(0, value - keyboardStep));
        } else if (e.key === "ArrowRight") {
            e.preventDefault();
            onseek(Math.min(1, value + keyboardStep));
        }
    }

    const seekLeft = $derived.by(() => {
        const position = seekValue ?? value;
        return `${position * 100}%`;
    });

    function startSeekSliderDrag(
        anchorClientX: number,
        anchorScreenX: number,
        touchId?: number,
    ): void {
        if (disabled || !progressBarEmpty) {
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

        seekValue = getPosition(anchorScreenX);

        onDrag(
            (screenX) => {
                seekValue = getPosition(screenX);
            },
            async (screenX) => {
                try {
                    await onseek(getPosition(screenX));
                } finally {
                    seekValue = undefined;
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

<div class="controls__group" class:controls--disabled={disabled}>
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
        <span
            class="controls__progress-fill"
            style:left={`${bufferLeft * 100}%`}
            style:width={`${bufferWidth * 100}%`}
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
