import EventDispatcher from "./eventdispatcher";

import { ok, deepStrictEqual } from "assert";

interface TestEventMap {
    test: (x: number) => void;
}

class TestEventDispatcher extends EventDispatcher<TestEventMap> {
    public dispatchEvent<K extends keyof TestEventMap>(
        key: K,
        ...args: Parameters<TestEventMap[K]>
    ): void {
        super.dispatchEvent(key, ...args);
    }

    public forEachEventListener(
        callback: (key: string, listener: (...args: any[]) => any) => void
    ): void {
        super.forEachEventListener(callback);
    }

    public clearEventListeners() {
        super.clearEventListeners();
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

    it("dispatches an event with no listeners", function () {
        const ed = new TestEventDispatcher();
        ed.dispatchEvent("test", 1);
    });

    it("loops over and clears listeners", function () {
        const ed = new TestEventDispatcher();

        let expectedListeners: { [K in keyof TestEventMap]?: TestEventMap[K][] } = {
            test: [
                x => { console.log(1); },
                x => { console.log(2); },
                x => { console.log(3); },
            ]
        };

        for (const keyString of Object.keys(expectedListeners)) {
            const key = keyString as keyof TestEventMap;
            for (const value of expectedListeners[key]!) {
                ed.addEventListener(key, value);
            }
        }

        const compareListeners = () => {
            const actualListeners: { [key: string]: any } = {};

            ed.forEachEventListener((key, listener) => {
                let listenerArray = actualListeners[key];
                if (listenerArray === undefined) {
                    listenerArray = [];
                    actualListeners[key] = listenerArray;
                }
                listenerArray.push(listener);
            });

            deepStrictEqual(expectedListeners, actualListeners);
        };

        compareListeners();

        expectedListeners = {};
        ed.clearEventListeners();

        compareListeners();
    });
});
