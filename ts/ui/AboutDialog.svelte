<script lang="ts">
    import Modal from "./Modal.svelte";

    let {
        open = $bindable(false),
    }: {
        open: boolean;
    } = $props();
</script>

<Modal bind:open>
    <div class="modal-box">
        <div class="aurelius-image"></div>

        <div class="divider font-bold">About</div>
        <div class="ml-4">
            <p>
                Aurelius is &copy; Philip Lafleur.
                <br />Licensed under the Apache License Version 2.0.
                <br /><a class="link-primary" href="https://github.com/beakbeak/aurelius"
                    >https://github.com/beakbeak/aurelius</a
                >
            </p>
        </div>

        <div class="divider font-bold">Attributions</div>
        <div class="ml-4">
            <p>
                Background image by Steve Schoger.
                <br />Licensed under Creative Commons
                <a class="link-primary" href="https://creativecommons.org/licenses/by/4.0/">CC BY 4.0</a>.
                <br /><a class="link-primary" href="https://www.heropatterns.com/"
                    >https://www.heropatterns.com/</a
                >
            </p>
            <p class="mt-2">
                <a
                    class="link-primary"
                    href="https://commons.wikimedia.org/wiki/File:Roman_-_Portrait_of_the_Emperor_Marcus_Aurelius_-_Walters_23215.jpg"
                    >Image of Marcus Aurelius sculpture</a
                >
                by Walters Art Museum.
                <br />Licensed under Creative Commons
                <a class="link-primary" href="https://creativecommons.org/licenses/by-sa/3.0/"
                    >CC BY-SA 3.0</a
                >.
                <br />Presented here with modifications.
            </p>
            <p class="mt-2">
                Material design icons licensed from Google under the Apache 2.0 license.
                <br /><a class="link-primary" href="https://github.com/google/material-design-icons"
                    >https://github.com/google/material-design-icons</a
                >
            </p>
            <p class="mt-2">
                This software uses libraries from the FFmpeg project under the LGPLv2.1.
                <br /><a class="link-primary" href="https://ffmpeg.org/">https://ffmpeg.org/</a>
            </p>
        </div>
    </div>
</Modal>

<style>
    .aurelius-image {
        background:
            center center / contain url("/img/aurelius.svgz") no-repeat,
            linear-gradient(hsl(240, 2%, 22%) 50%, hsl(240, 2%, 15%));
        border-radius: 0.5rem;
        box-shadow: inset 0 0 1rem 0rem rgba(0, 0, 0, 0.5);
        display: block;
        min-width: 10rem;
        max-width: 16rem;
        height: 12rem;
        margin: 1rem auto;
    }
</style>
