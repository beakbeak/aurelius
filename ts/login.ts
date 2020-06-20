window.onload = () => {
    const form = document.getElementById("form") as HTMLFormElement;
    const query = window.location.search;

    // Pass "from" argument to login, which indicates which URL to load afterward
    form.action += query;

    if (query.match(/^\?failed/)) {
        form.classList.add("failed");
    }
};
