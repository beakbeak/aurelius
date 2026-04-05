<script lang="ts">
    import { Player } from "../../core/player";
    import { getSettings } from "../../ui/settings";
    import { LogLevel, serverLog } from "../../core/log";
    import { makePlayerState } from "../../ui/PlayerState.svelte";
    import { DirState } from "../../ui/DirState.svelte";
    import PlayerControls from "../../ui/PlayerControls.svelte";
    import DirectoryBrowser from "../../ui/DirectoryBrowser.svelte";
    import SettingsDialog from "../../ui/SettingsDialog.svelte";
    import SearchDialog from "../../ui/SearchDialog.svelte";
    import KeyboardShortcutsDialog from "../../ui/KeyboardShortcutsDialog.svelte";
    import CenteredLayout from "../../ui/CenteredLayout.svelte";
    import AboutDialog from "../../ui/AboutDialog.svelte";
    import ImageGalleryDialog from "../../ui/ImageGalleryDialog.svelte";

    const settings = getSettings();
    const player = new Player({ streamConfig: settings.streamConfig });

    const eventNames = [
        "play",
        "ended",
        "pause",
        "unpause",
        "favorite",
        "unfavorite",
        "autoNext",
    ] as const;
    eventNames.forEach((eventName) => {
        player.addEventListener(eventName, (track) =>
            serverLog(LogLevel.Info, `player: ${eventName}`, { track: track.info.name }),
        );
    });

    const playerState = makePlayerState(player);
    const dirState = new DirState(player);

    let showSettings = $state(false);
    let showSearch = $state(false);
    let showShortcuts = $state(false);
    let showAbout = $state(false);
    let showImageGallery = $state(false);

    function isTypingInInput(target: EventTarget | null): boolean {
        if (!target || !(target instanceof HTMLElement)) {
            return false;
        }
        const tagName = target.tagName.toLowerCase();
        return tagName === "input" || tagName === "textarea" || target.isContentEditable;
    }

    function handleKeydown(e: KeyboardEvent) {
        if (isTypingInInput(e.target) || e.metaKey || e.ctrlKey || e.altKey) {
            return;
        }
        if (showSettings || showSearch || showShortcuts || showAbout || showImageGallery) {
            return;
        }

        switch (e.key) {
            case "/":
                e.preventDefault();
                showSearch = true;
                break;
            case " ":
                e.preventDefault();
                if (player.track && player.track.isPaused()) {
                    player.unpause();
                } else {
                    player.pause();
                }
                break;
            case "]":
            case "w":
                e.preventDefault();
                player.next();
                break;
            case "[":
            case "q":
                e.preventDefault();
                player.previous();
                break;
            case "Backspace":
                e.preventDefault();
                dirState.navigateToTopLevel();
                break;
            case "<":
                e.preventDefault();
                window.history.back();
                break;
            case ">":
                e.preventDefault();
                window.history.forward();
                break;
            case "\\":
                e.preventDefault();
                dirState.navigateToParent();
                break;
            case "1":
            case "2":
            case "3":
            case "4":
            case "5":
            case "6":
            case "7":
            case "8":
            case "9":
            case "0":
                e.preventDefault();
                dirState.playTrackByIndex(e.key === "0" ? 9 : parseInt(e.key) - 1);
                break;
            case "f":
                e.preventDefault();
                if (player.track) {
                    if (player.track.info.favorite) {
                        player.unfavorite();
                    } else {
                        player.favorite();
                    }
                }
                break;
            case "?":
                e.preventDefault();
                showShortcuts = true;
                break;
            case "t":
                e.preventDefault();
                showSettings = true;
                break;
            case "'":
            case '"':
            case "s":
            case "S":
                e.preventDefault();
                if (player.track) {
                    const currentTime = player.track.currentTime();
                    const seekAmount = e.shiftKey ? 30 : 10;
                    const targetTime = currentTime + seekAmount;
                    if (targetTime < player.track.info.duration) {
                        player.seekTo(targetTime);
                    }
                }
                break;
            case ";":
            case ":":
            case "a":
            case "A":
                e.preventDefault();
                if (player.track) {
                    const currentTime = player.track.currentTime();
                    const seekAmount = e.shiftKey ? 30 : 10;
                    player.seekTo(Math.max(0, currentTime - seekAmount));
                }
                break;
            case "=":
            case "`":
                e.preventDefault();
                dirState.playFavorites();
                break;
            case "c":
                e.preventDefault();
                if (playerState.track?.info.attachedImages?.length) {
                    showImageGallery = true;
                }
                break;
            case "g":
                e.preventDefault();
                if (player.track) {
                    dirState.loadDir(player.track.info.dir);
                }
                break;
        }
    }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="controls">
    <PlayerControls
        {playerState}
        onAbout={() => (showAbout = true)}
        onNavigateToDir={(url) => dirState.loadDir(url)}
        onShowImageGallery={() => (showImageGallery = true)}
    />
</div>

<CenteredLayout>
    <main class="dir-browser">
        <DirectoryBrowser {playerState} {dirState} />
    </main>
</CenteredLayout>

<aside class="menu top-right__menu">
    <button
        class="menu__button material-icons"
        title="Settings"
        type="button"
        onclick={() => (showSettings = true)}
    >
        settings
    </button>
    <button
        class="menu__button material-icons"
        title="Search"
        type="button"
        onclick={() => (showSearch = true)}
    >
        search
    </button>
</aside>

<SettingsDialog
    bind:open={showSettings}
    onSave={(newSettings) => {
        player.streamConfig = newSettings.streamConfig;
        showSettings = false;
    }}
/>

<SearchDialog bind:open={showSearch} {dirState} />

<KeyboardShortcutsDialog bind:open={showShortcuts} />

<AboutDialog bind:open={showAbout} />

<ImageGalleryDialog
    bind:open={showImageGallery}
    images={playerState.track?.info.attachedImages ?? []}
/>

<style>
    .controls {
        position: fixed;
        bottom: 0;
        left: 50%;
        transform: translateX(-50%);
        width: 100%;
        max-width: 1200px;
        box-shadow: 0px 0px 1rem rgba(0, 0, 0, 0.75);
    }

    .dir-browser {
        margin-bottom: 10rem;
    }

    .top-right__menu {
        position: fixed;
        z-index: 1;
        right: 0;
        top: 0;
        margin: 0.5rem;
        display: flex;
        flex-direction: column;
    }

    .menu__button {
        font-size: 3rem;
        color: black;
        text-shadow: 0 0 2px white;
        cursor: pointer;
    }

    @media (min-width: 1200px) {
        .controls {
            left: calc(50% + (100vw - 100%) / 2);
            bottom: inherit;
            top: 0;
        }

        .dir-browser {
            margin-bottom: 0;
            margin-top: 8rem;
        }

        .top-right__menu {
            top: inherit;
            bottom: 0;
            flex-direction: column-reverse;
        }
    }
</style>
