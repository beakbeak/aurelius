import { Class } from "./class";

let elementsEnsured = false;
let overlay: HTMLElement;

let currentDialog: HTMLElement | undefined;

function ensureElements() {
    if (elementsEnsured) {
        return;
    }
    elementsEnsured = true;

    overlay = document.createElement("div");
    overlay.classList.add(Class.ModalOverlay, Class.Hidden);
    overlay.onclick = hideModalDialog;
    document.body.appendChild(overlay);

    document.addEventListener("keydown", (e) => {
        if (e.key === "Escape" && currentDialog !== undefined) {
            hideModalDialog();
        }
    });
}

export function showModalDialog(dialog: HTMLElement): void {
    ensureElements();

    if (currentDialog !== undefined) {
        hideModalDialog();
    }
    currentDialog = dialog;

    dialog.classList.remove(Class.Hidden);
    overlay.classList.remove(Class.Hidden);
}

export function hideModalDialog(): void {
    if (currentDialog === undefined) {
        return;
    }

    currentDialog.classList.add(Class.Hidden);
    overlay.classList.add(Class.Hidden);

    currentDialog = undefined;
}
