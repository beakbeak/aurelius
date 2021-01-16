import { Player } from "./core/player";
import setupPlayerUi from "./ui/player";
import setupDirUi from "./ui/dir";
import { getSettings } from "./ui/settings";
import { showSettingsDialog } from "./ui/settings-dialog";

window.onload = () => {
    const player = new Player(getSettings().streamConfig);

    setupDirUi(player);
    setupPlayerUi(player);

    document.getElementById("settings-button")!.onclick = () => {
        showSettingsDialog((settings) => {
            player.streamConfig = settings.streamConfig;
        });
    };
};
