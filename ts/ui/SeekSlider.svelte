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

<div class="flex h-12 items-center justify-center">
    <!-- Progress trough -->
    <div
        bind:this={progressBarEmpty}
        class={[
            "progress-trough flex-1 relative h-2 mx-2 rounded-lg",
            !disabled && "cursor-pointer",
        ]}
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
        <!-- Progress fill -->
        <span
            class="bg-base-200 absolute inset-0 h-full rounded-lg"
            style:left={`${bufferLeft * 100}%`}
            style:width={`${bufferWidth * 100}%`}
        ></span>
        <!-- Slider range (trough width - handle width) -->
        <span class="absolute inset-0 h-full" style:width="calc(100% - 3rem)">
            <!-- Slider handle -->
            <span
                class={[
                    "absolute w-12 h-[200%] -top-[50%] rounded-sm z-1",
                    disabled ? "bg-base-200" : "bg-neutral cursor-pointer",
                ]}
                style:left={seekLeft}
            ></span>
        </span>
    </div>
</div>

<style>
    /* Draw progress trough shadow above progress fill */
    .progress-trough::after {
        content: "";
        position: absolute;
        inset: 0;
        box-shadow: inset 0 0 3px black;
        border-radius: inherit;
        pointer-events: none;
    }
</style>
