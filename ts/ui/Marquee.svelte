<script lang="ts">
    let {
        text,
        url,
        onNavigate,
    }: {
        text: string;
        url: string;
        onNavigate: (url: string) => void;
    } = $props();

    let element: HTMLAnchorElement | undefined = $state(undefined);
    let styleElement: HTMLStyleElement | undefined;

    let href = $derived(url ? `/media/tree/?path=${encodeURIComponent(url)}` : "#");

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

    function handleClick(e: MouseEvent): void {
        e.preventDefault();
        if (url) {
            onNavigate(url);
        }
    }

    $effect(() => {
        // Track text changes to trigger re-calculation
        void text;
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

<a
    bind:this={element}
    class="controls__marquee controls__link"
    {href}
    title="Jump to directory containing this track"
    onclick={handleClick}
>
    {text}
</a>

<style>
    .controls__link {
        cursor: pointer;
        font-style: italic;
        text-decoration: none;
        color: inherit;
    }
    .controls__link:hover {
        text-decoration: underline;
    }

    .controls__marquee {
        display: block;
        max-width: 100%;
        max-height: 100%;
        white-space: nowrap;
        text-align: center;
        font-size: 1.1em;
    }
</style>
