/**
 * EventDispatcher provides type-safe methods for listening for and dispatching
 * events described by a TypeScript `interface`.
 *
 * Example:
 *
 *      interface Foo {
 *          fooEvent: (x: Bar) => number;
 *          barEvent: (y: Baz) => void;
 *      }
 *      class FooDispatcher extends EventDispatcher<Foo> {
 *          // ...
 *      }
 */
export default class EventDispatcher<
    EventMap extends Record<keyof EventMap, (...args: any[]) => any>
> {
    private _listeners: {[key: string]: ((...args: any[]) => any)[] | undefined} = {};

    public addEventListener<K extends keyof EventMap>(
        key: K,
        value: EventMap[K],
    ): void {
        let listeners = this._listeners[key as string];
        if (listeners === undefined) {
            listeners = [];
            this._listeners[key as string] = listeners;
        }
        listeners.push(value);
    }

    protected dispatchEvent<K extends keyof EventMap>(
        key: K,
        ...args: Parameters<EventMap[K]>
    ): void {
        let listeners = this._listeners[key as string];
        if (listeners === undefined) {
            return;
        }

        for (const listener of listeners) {
            listener.apply(undefined, args);
        }
    }

    protected forEachEventListener(
        callback: (key: string, listener: (...args: any[]) => any) => void
    ): void {
        for (const key of Object.keys(this._listeners)) {
            const listenerArray = this._listeners[key];
            if (listenerArray !== undefined) {
                for (const listener of listenerArray) {
                    callback(key, listener);
                }
            }
        }
    }

    protected clearEventListeners() {
        this._listeners = {};
    }
}
