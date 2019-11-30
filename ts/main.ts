import { Player } from "./core/player.js";
import setupPlayerUi from "./ui/player.js";
import setupDirUi from "./ui/dir.js";

window.onload = () => {
    const player = new Player();

    setupPlayerUi(player, "header");
    setupDirUi(player, "content");

    // XXX hack alert
    if (/([0-9]+\.){3}[0-9]+/.test(window.location.hostname)) {
        player.setStreamOptions({ codec: "wav" });
    }
};