<script lang="ts">
    import type { Snippet } from "svelte";

    let { children }: { children: Snippet } = $props();
</script>

<main>
    {@render children()}
</main>

<style>
    main {
        max-width: 960px;
        margin-left: auto;
        margin-right: auto;
        padding-left: 0.5rem;
        padding-right: 0.5rem;
    }

    :global(html) {
        background: hsl(39, 0%, 11.8%);
    }

    @media (min-width: 960px) {
        :global(html) {
            background:
                calc(50vw - (960px + 2rem) / 2) top / calc(960px + 2rem) fixed no-repeat
                    linear-gradient(hsl(39, 0%, 11.8%) 50%, hsl(39, 0%, 11.8%)),
                left top / calc((100vw - (960px + 2rem)) / 2) fixed no-repeat
                    linear-gradient(
                        to right,
                        rgba(0, 0, 0, 0.25) calc(100% - 150px),
                        rgba(0, 0, 0, 0.35) 100%
                    ),
                right top / calc((100vw - (960px + 2rem)) / 2) fixed no-repeat
                    linear-gradient(
                        to left,
                        rgba(0, 0, 0, 0.25) calc(100% - 150px),
                        rgba(0, 0, 0, 0.35) 100%
                    ),
                fixed url("/img/bg.svgz"),
                fixed linear-gradient(hsl(240, 2%, 30%) 50%, hsl(240, 2%, 20%));
        }
    }

    @media (min-width: 1200px) {
        :global(html) {
            background:
                calc(50vw - (960px + 2rem) / 2) top / calc(960px + 2rem) fixed no-repeat
                    linear-gradient(hsl(39, 0%, 11.8%) 50%, hsl(39, 0%, 11.8%)),
                left top / calc((100vw - (960px + 2rem)) / 2) fixed no-repeat
                    linear-gradient(
                        to right,
                        rgba(0, 0, 0, 0.25) calc(100% - 150px),
                        rgba(0, 0, 0, 0.35) 100%
                    ),
                right top / calc((100vw - (960px + 2rem)) / 2) fixed no-repeat
                    linear-gradient(
                        to left,
                        rgba(0, 0, 0, 0.25) calc(100% - 150px),
                        rgba(0, 0, 0, 0.35) 100%
                    ),
                fixed url("/img/bg.svgz"),
                fixed linear-gradient(hsl(240, 2%, 30%) 50%, hsl(240, 2%, 20%));
            margin-left: calc(100vw - 100%);
            margin-right: 0;
        }
    }
</style>
