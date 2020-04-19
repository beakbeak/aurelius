import { env } from "process";
import { JSDOM } from "jsdom";

export let host = "http://localhost:9090";

if (env.AURELIUS_HOST !== undefined) {
    host = env.AURELIUS_HOST;
}

const { window } = new JSDOM("", { url: host });

// @ts-ignore
global["window"] = window;
// @ts-ignore
global["document"] = window.document;
// @ts-ignore
global["XMLHttpRequest"] = window.XMLHttpRequest;
// @ts-ignore
global["Audio"] = window.Audio;
