/**
 * EventDispatcher provides type-safe methods for listening for and dispatching
 * events.
 */
export default class EventDispatcher<
    EventMap extends Record<keyof EventMap, (...args: any[]) => any>
> {
    private readonly _listeners: {[key: string]: ((...args: any[]) => any)[] | undefined} = {};

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

    protected _dispatchEvent<K extends keyof EventMap>(
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
}
