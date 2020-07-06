import { Class } from "./ui/class";

window.onload = () => {
    const form = document.getElementById("login-form") as HTMLFormElement;
    const query = window.location.search;

    // Pass "from" argument to login, which indicates which URL to load afterward
    form.action += query;

    if (query.match(/^\?failed/)) {
        const error = document.getElementById("login-error")!;
        const passwordInput = document.getElementById("password-input")!;

        error.classList.remove(Class.Hidden);
        passwordInput.classList.add(Class.UiEntryInput_Error);
    }
};
