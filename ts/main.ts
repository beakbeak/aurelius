import { Player } from "./core/player";
import setupPlayerUi from "./ui/player";
import setupDirUi from "./ui/dir";

window.onload = () => {
    const player = new Player();

    setupDirUi(player, "content");
    setupPlayerUi(player, "header");
};
