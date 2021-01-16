import { env } from "process";
import { JSDOM } from "jsdom";

export let host = "http://localhost:9090";

if (env.AURELIUS_HOST !== undefined) {
    host = env.AURELIUS_HOST;
}

const { window } = new JSDOM("", { url: host });

// Testing utilities ///////////////////////////////////////////////////////////

export class EventChecker {
    private _gotEvent = false;

    public set(): void {
        this._gotEvent = true;
    }

    public consume(): boolean {
        const out = this._gotEvent;
        this._gotEvent = false;
        return out;
    }
}

// HTMLMediaElement mocking ////////////////////////////////////////////////////

type MockMedia = HTMLMediaElement & {
    _readyState?: number;
    _paused?: boolean;
};

Object.defineProperty(window.HTMLMediaElement.prototype, "readyState", {
    get(this: MockMedia): number {
        return this._readyState !== undefined ? this._readyState : 0;
    },
});
Object.defineProperty(window.HTMLMediaElement.prototype, "paused", {
    get(this: MockMedia): boolean {
        return this._paused !== undefined ? this._paused : false;
    },
});

function mediaLoad(this: MockMedia) {
    this._readyState = window.HTMLMediaElement.HAVE_FUTURE_DATA;
    this.dispatchEvent(new window.Event("canplay"));
}

function mediaPlay(this: MockMedia): Promise<void> {
    if (this.paused) {
        this._paused = false;
        this.dispatchEvent(new window.Event("play"));
    }
    return Promise.resolve();
}

function mediaPause(this: MockMedia): void {
    if (this.paused) {
        return;
    }
    this._paused = true;
    this.dispatchEvent(new window.Event("pause"));
}

window.HTMLMediaElement.prototype.load = mediaLoad;
window.HTMLMediaElement.prototype.play = mediaPlay;
window.HTMLMediaElement.prototype.pause = mediaPause;

// Browser polyfill ////////////////////////////////////////////////////////////

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
