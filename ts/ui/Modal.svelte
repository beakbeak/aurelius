<script lang="ts">
    import type { Snippet } from "svelte";

    let {
        open = $bindable(false),
        hideCloseFocusRing = false,
        children,
    }: {
        open: boolean;
        hideCloseFocusRing?: boolean;
        children: Snippet;
    } = $props();

    let dialogEl: HTMLDialogElement | undefined = $state(undefined);

    $effect(() => {
        if (!dialogEl) return;
        if (open && !dialogEl.open) {
            dialogEl.showModal();
        } else if (!open && dialogEl.open) {
            dialogEl.close();
        }
    });
</script>

<dialog bind:this={dialogEl} class="modal" onclose={() => (open = false)}>
    {@render children()}
    <form method="dialog" class="modal-backdrop">
        <button class:hide-focus-ring={hideCloseFocusRing}>close</button>
    </form>
</dialog>

<style>
    .hide-focus-ring {
        outline: none;
    }
</style>
