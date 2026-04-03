<script lang="ts">
    import type { Snippet } from "svelte";

    const {
        children,
        contentKey,
    }: {
        children: Snippet;
        /** When this value changes, the scroll animation is recalculated. */
        contentKey?: unknown;
    } = $props();

    let element: HTMLDivElement | undefined = $state(undefined);
    let styleElement: HTMLStyleElement | undefined;

    function updateAnimation(): void {
        if (!element) {
            return;
        }

        // Remove previous dynamic style
        if (styleElement) {
            styleElement.remove();
            styleElement = undefined;
        }
        element.style.animation = "";

        if (element.clientWidth >= element.scrollWidth) {
            return;
        }

        const scrollLength = element.scrollWidth - element.clientWidth;
        const scrollTime = scrollLength / 50; // px/second
        const waitTime = 2; // seconds
        const totalTime = 2 * (scrollTime + waitTime);
        const scrollPercent = 100 * (scrollTime / totalTime);
        const waitPercent = 100 * (waitTime / totalTime);

        const style = document.createElement("style");
        style.textContent = `@keyframes marquee {
            ${scrollPercent}% {
                transform: translateX(-${scrollLength}px);
            }
            ${scrollPercent + waitPercent}% {
                transform: translateX(-${scrollLength}px);
            }
            ${2 * scrollPercent + waitPercent}% {
                transform: translateX(0px);
            }
        }`;
        document.head.appendChild(style);
        styleElement = style;
        element.style.animation = `marquee ${totalTime}s infinite linear`;
    }

    $effect(() => {
        // Track key changes to trigger re-calculation
        void contentKey;
        // Use a microtask to ensure DOM has updated
        queueMicrotask(() => {
            updateAnimation();
        });
        return () => {
            if (styleElement) {
                styleElement.remove();
                styleElement = undefined;
                element?.style.setProperty("animation", "");
            }
        };
    });
</script>

<svelte:window onresize={updateAnimation} />

<div bind:this={element} class="marquee">
    {@render children()}
</div>

<style>
    .marquee {
        display: block;
        max-width: 100%;
        max-height: 100%;
        white-space: nowrap;
        text-align: center;
    }
</style>
