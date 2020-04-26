import { env } from "process";
import { JSDOM } from "jsdom";

export let host = "http://localhost:9090";

if (env.AURELIUS_HOST !== undefined) {
    host = env.AURELIUS_HOST;
}

const { window } = new JSDOM("", { url: host });

type MockMedia = HTMLMediaElement & {
    _readyState?: number;
};

Object.defineProperty(window.HTMLMediaElement.prototype, "readyState", {
    get(this: MockMedia): number {
        return this._readyState !== undefined ? this._readyState : 0;
    }
});

function mediaLoad(this: MockMedia) {
    this._readyState = window.HTMLMediaElement.HAVE_FUTURE_DATA;
    this.dispatchEvent(new window.Event("canplay"));
}

function mediaPlay(this: MockMedia): Promise<void> {
    return Promise.resolve();
}

window.HTMLMediaElement.prototype.load = mediaLoad;
window.HTMLMediaElement.prototype.play = mediaPlay;
// TODO: window.HTMLMediaElement.prototype.pause = mediaPause;

// @ts-ignore
global["window"] = window;
// @ts-ignore
global["document"] = window.document;
// @ts-ignore
global["XMLHttpRequest"] = window.XMLHttpRequest;
// @ts-ignore
global["Audio"] = window.Audio;
// @ts-ignore
global["HTMLMediaElement"] = window.HTMLMediaElement;
