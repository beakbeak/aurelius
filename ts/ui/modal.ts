import { Class } from "./class";

let elementsEnsured = false;
let modalOverlay: HTMLElement;

let currentDialog: HTMLElement | undefined;

function ensureElements() {
    if (elementsEnsured) {
        return;
    }
    elementsEnsured = true;

    modalOverlay = document.createElement("div");
    modalOverlay.classList.add(Class.ModalOverlay);
    modalOverlay.onclick = hideModalDialog;
    document.body.appendChild(modalOverlay);
}

export function showModalDialog(dialog: HTMLElement): void {
    ensureElements();

    if (currentDialog !== undefined) {
        hideModalDialog();
    }
    currentDialog = dialog;

    for (const element of [dialog, modalOverlay]) {
        element.classList.add(Class.ModalIsVisible);
    }
}

export function hideModalDialog(): void {
    if (currentDialog === undefined) {
        return;
    }

    for (const element of [currentDialog, modalOverlay]) {
        element.classList.remove(Class.ModalIsVisible);
    }
    currentDialog = undefined;
}
