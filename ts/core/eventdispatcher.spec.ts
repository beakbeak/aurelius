import EventDispatcher from "./eventdispatcher";

import { ok } from "assert";

interface TestEventMap {
    test: (x: number) => void;
}

class TestEventDispatcher extends EventDispatcher<TestEventMap> {
    public dispatchEvent<K extends keyof TestEventMap>(
        key: K,
        ...args: Parameters<TestEventMap[K]>
    ): void {
        this._dispatchEvent(key, ...args);
    }
}

describe("EventDispatcher", function () {
    it("dispatches an event to multiple listeners", function () {
        const ed = new TestEventDispatcher();

        let firstValue = 0;
        ed.addEventListener("test", (x) => { firstValue = x; })

        let secondValue = 0;
        ed.addEventListener("test", (x) => { secondValue = x; })

        const expectedValue = 1;
        ed.dispatchEvent("test", expectedValue);
        ok(firstValue === expectedValue && secondValue === expectedValue);
    });
});

describe("EventDispatcher", function () {
    it("dispatches an event with no listeners", function () {
        const ed = new TestEventDispatcher();
        ed.dispatchEvent("test", 1);
    });
});
