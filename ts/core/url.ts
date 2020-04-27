export function stripQueryString(urlString: string): string {
    const url = document.createElement("a");
    url.href = urlString;
    return `${url.protocol}//${url.host}${url.pathname}`;
}

export function stripLastPathElement(url: string): string {
    return url.split("/").slice(0, -1).join("/");
}
