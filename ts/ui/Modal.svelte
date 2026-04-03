<script lang="ts">
    import type { Snippet } from "svelte";

    let {
        open = $bindable(false),
        children,
    }: {
        open: boolean;
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
        <button>close</button>
    </form>
</dialog>
