<script lang="ts">
    import type { Snippet } from "svelte";

    let {
        open = $bindable(false),
        dialogClass = "",
        children,
    }: {
        open: boolean;
        dialogClass?: string;
        children: Snippet;
    } = $props();

    function handleOverlayClick(): void {
        open = false;
    }

    function handleKeydown(e: KeyboardEvent): void {
        if (e.key === "Escape" && open) {
            open = false;
        }
    }

    $effect(() => {
        if (open) {
            document.addEventListener("keydown", handleKeydown);
            return () => {
                document.removeEventListener("keydown", handleKeydown);
            };
        }
    });
</script>

{#if open}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal-overlay" onclick={handleOverlayClick}></div>
    <div class="ui modal dialog {dialogClass}">
        {@render children()}
    </div>
{/if}
