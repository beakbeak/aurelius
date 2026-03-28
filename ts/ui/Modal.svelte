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
    <div class="modal-overlay" onclick={handleOverlayClick}></div>
    <div class="ui modal dialog {dialogClass}">
        {@render children()}
    </div>
{/if}

<style>
    .modal-overlay {
        display: block;
        position: fixed;
        top: 0;
        left: 0;
        bottom: 0;
        right: 0;
        background-color: rgba(0, 0, 0, 0.5);
        z-index: 99;
    }

    .modal {
        display: block;
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        z-index: 100;
    }

    .dialog {
        padding: 0.75rem;
        min-width: 15rem;
        box-shadow: 0px 0px 1.5rem rgba(0, 0, 0, 0.5);
        border-radius: 0.25rem;
    }
</style>
