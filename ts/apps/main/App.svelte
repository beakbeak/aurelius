<script lang="ts">
    import { Player } from "../../core/player";
    import { getSettings } from "../../ui/settings";
    import { LogLevel, serverLog } from "../../core/log";
    import { createPlayerState } from "../../ui/state/playerState.svelte";
    import { createDirState } from "../../ui/state/dirState.svelte";
    import PlayerControls from "../../ui/PlayerControls.svelte";
    import DirectoryBrowser from "../../ui/DirectoryBrowser.svelte";
    import Modal from "../../ui/Modal.svelte";
    import SettingsDialog from "../../ui/SettingsDialog.svelte";
    import SearchDialog from "../../ui/SearchDialog.svelte";
    import KeyboardShortcutsDialog from "../../ui/KeyboardShortcutsDialog.svelte";
    import AboutDialog from "../../ui/AboutDialog.svelte";

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

    const playerState = createPlayerState(player);
    const dirState = createDirState(player);

    let showSettings = $state(false);
    let showSearch = $state(false);
    let showShortcuts = $state(false);
    let showAbout = $state(false);

    function handleMenuKeydown(e: KeyboardEvent, action: () => void): void {
        if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            action();
        }
    }

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
                    window.open(playerState.track.info.attachedImages[0].url, "_blank");
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

<PlayerControls
    {player}
    {playerState}
    onAbout={() => (showAbout = true)}
    onNavigateToDir={(url) => dirState.loadDir(url)}
/>

<main class="dir main__dir">
    <DirectoryBrowser {player} {playerState} {dirState} />
</main>

<aside class="menu top-right__menu">
    <i
        class="menu__button material-icons"
        title="Settings"
        role="button"
        tabindex="0"
        onclick={() => (showSettings = true)}
        onkeydown={(e) => handleMenuKeydown(e, () => (showSettings = true))}
    >
        settings
    </i>
    <i
        class="menu__button material-icons"
        title="Search"
        role="button"
        tabindex="0"
        onclick={() => (showSearch = true)}
        onkeydown={(e) => handleMenuKeydown(e, () => (showSearch = true))}
    >
        search
    </i>
</aside>

<Modal bind:open={showSettings}>
    <SettingsDialog
        onSave={(newSettings) => {
            player.streamConfig = newSettings.streamConfig;
            showSettings = false;
        }}
    />
</Modal>

<Modal bind:open={showSearch} dialogClass="search-dialog">
    <SearchDialog {dirState} onClose={() => (showSearch = false)} />
</Modal>

<Modal bind:open={showShortcuts}>
    <KeyboardShortcutsDialog />
</Modal>

<Modal bind:open={showAbout}>
    <AboutDialog />
</Modal>

<style>
    :global(.main__controls) {
        position: fixed;
        bottom: 0;
        left: 50%;
        transform: translateX(-50%);
        width: 100%;
        max-width: 1200px;
        box-shadow: 0px 0px 1rem rgba(0, 0, 0, 0.75);
    }

    .main__dir {
        max-width: 960px;
        margin-left: auto;
        margin-right: auto;
        margin-bottom: 10rem;
    }

    :global(body) {
        background: hsl(39, 0%, 11.8%);
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
        cursor: pointer;
        font-size: 3rem;
        color: black;
        text-shadow: 0 0 2px white;
    }

    @media (min-width: 960px) {
        :global(body) {
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
        :global(body) {
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

        :global(html) {
            margin-left: calc(100vw - 100%);
            margin-right: 0;
        }

        :global(.main__controls) {
            left: calc(50% + (100vw - 100%) / 2);
            bottom: inherit;
            top: 0;
        }

        .main__dir {
            margin-bottom: inherit;
            margin-top: 8rem;
        }

        .top-right__menu {
            top: inherit;
            bottom: 0;
            flex-direction: column-reverse;
        }
    }
</style>
