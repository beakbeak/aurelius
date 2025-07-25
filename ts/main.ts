import { Player } from "./core/player";
import { setupPlayerUi, openTrackImageInNewTab } from "./ui/player";
import {
    setupDirUi,
    playTrackByIndex,
    navigateToParent,
    navigateToTopLevel,
    playFavorites,
    loadDir,
} from "./ui/dir";
import { getSettings } from "./ui/settings";
import { showSettingsDialog } from "./ui/settings-dialog";
import { showModalDialog } from "./ui/modal";

window.onload = () => {
    const player = new Player({ streamConfig: getSettings().streamConfig });

    setupDirUi(player);
    setupPlayerUi(player);

    const showAndApplySettings = () => {
        showSettingsDialog((settings) => {
            player.streamConfig = settings.streamConfig;
        });
    };

    document.getElementById("settings-button")!.onclick = showAndApplySettings;

    document.addEventListener("keydown", (e) => {
        if (isTypingInInput(e.target) || e.metaKey || e.ctrlKey || e.altKey) {
            return;
        }

        switch (e.key) {
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
                navigateToTopLevel();
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
                navigateToParent();
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
                playTrackByIndex(e.key === "0" ? 9 : parseInt(e.key) - 1);
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
                showModalDialog(document.getElementById("keyboard-shortcuts-dialog")!);
                break;
            case "t":
                e.preventDefault();
                showAndApplySettings();
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
                playFavorites();
                break;
            case "c":
                e.preventDefault();
                openTrackImageInNewTab();
                break;
            case "g":
                e.preventDefault();
                if (player.track) {
                    loadDir(player.track.info.dir);
                }
                break;
        }
    });
};

function isTypingInInput(target: EventTarget | null): boolean {
    if (!target || !(target instanceof HTMLElement)) {
        return false;
    }
    const tagName = target.tagName.toLowerCase();
    return tagName === "input" || tagName === "textarea" || target.isContentEditable;
}
