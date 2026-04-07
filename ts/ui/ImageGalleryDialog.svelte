<script lang="ts">
    import type { ImageInfo } from "../core/track";
    import Modal from "./Modal.svelte";

    let {
        open = $bindable(false),
        images,
    }: {
        open: boolean;
        images: ImageInfo[];
    } = $props();

    let index = $state(0);

    $effect(() => {
        if (open) {
            index = 0;
        }
    });

    let current = $derived(images[index]);

    function prev(): void {
        index = (index - 1 + images.length) % images.length;
    }

    function next(): void {
        index = (index + 1) % images.length;
    }

    function handleKeydown(e: KeyboardEvent): void {
        if (images.length <= 1) {
            return;
        }
        switch (e.key) {
            case "ArrowLeft":
            case "[":
            case "q":
                e.preventDefault();
                prev();
                break;
            case "ArrowRight":
            case "]":
            case "w":
                e.preventDefault();
                next();
                break;
        }
    }
</script>

<svelte:window onkeydown={open ? handleKeydown : undefined} />

<Modal bind:open hideCloseFocusRing>
    <div class="modal-box w-fit min-w-1/2 max-w-3/4 h-3/4 flex flex-col items-center gap-2">
        {#if current}
            <img
                class="gallery-image"
                src={current.url}
                alt="Track image {index + 1} of {images.length}"
            />
        {/if}
        {#if images.length > 1}
            <div class="flex items-center gap-2">
                <button class="btn btn-circle btn-ghost" type="button" onclick={prev}>
                    <i class="material-icons">chevron_left</i>
                </button>
                <span>{index + 1} / {images.length}</span>
                <button class="btn btn-circle btn-ghost" type="button" onclick={next}>
                    <i class="material-icons">chevron_right</i>
                </button>
            </div>
        {/if}
    </div>
</Modal>

<style>
    .gallery-image {
        flex: 1;
        min-height: 0;
        max-width: 100%;
        object-fit: contain;
    }
</style>
