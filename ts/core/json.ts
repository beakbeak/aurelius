export function sendJsonRequest<Response>(
    method: string,
    url: string,
    data?: any,
): Promise<Response> {
    const req = new XMLHttpRequest();
    req.open(method, url);
    return new Promise((resolve, reject) => {
        req.onreadystatechange = () => {
            if (req.readyState !== req.DONE) {
                return;
            }
            if (req.status === 200) {
                resolve(JSON.parse(req.responseText));
            } else {
                reject(new Error(`request failed (${req.status}): ${url}`));
            }
        }

        if (data !== undefined) {
            req.setRequestHeader("Content-Type", "application/json");
            req.send(JSON.stringify(data));
        } else {
            req.send();
        }
    });
}

export function fetchJson<Response>(url: string): Promise<Response> {
    return sendJsonRequest<Response>("GET", url);
}

export function postJson<Response>(
    url: string,
    data?: any,
): Promise<Response> {
    return sendJsonRequest<Response>("POST", url, data);
}

export function copyJson(obj: any) {
    return JSON.parse(JSON.stringify(obj));
}

export function nullToUndefined<T>(value: T | null): T | undefined {
    return value !== null ? value : undefined;
}
