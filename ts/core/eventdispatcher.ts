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
    private _listeners: {[eventName: string]: ((...args: any[]) => any)[] | undefined} = {};

    /** Set a function to be called when a particular event is dispatched. */
    public addEventListener<K extends keyof EventMap>(
        eventName: K,
        value: EventMap[K],
    ): void {
        let listeners = this._listeners[eventName as string];
        if (listeners === undefined) {
            listeners = [];
            this._listeners[eventName as string] = listeners;
        }
        listeners.push(value);
    }

    /** Call each listener of a given event with the supplied arguments. */
    protected dispatchEvent<K extends keyof EventMap>(
        eventName: K,
        ...args: Parameters<EventMap[K]>
    ): void {
        let listeners = this._listeners[eventName as string];
        if (listeners === undefined) {
            return;
        }

        for (const listener of listeners) {
            listener.apply(undefined, args);
        }
    }

    /** Execute `callback` once for each listener of each event. */
    protected forEachEventListener(
        callback: (eventName: string, listener: (...args: any[]) => any) => void,
    ): void {
        for (const eventName of Object.keys(this._listeners)) {
            const listenerArray = this._listeners[eventName]!;
            for (const listener of listenerArray) {
                callback(eventName, listener);
            }
        }
    }

    /** Remove all listeners. */
    protected clearEventListeners() {
        this._listeners = {};
    }
}
