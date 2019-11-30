export default class EventDispatcher<M extends Record<keyof M, (...args: any[]) => any>> {
    private readonly _listeners: {[key: string]: ((...args: any[]) => any)[] | undefined} = {};

    public addEventListener<K extends keyof M>(
        key: K,
        value: M[K],
    ): void {
        let listeners = this._listeners[key as string];
        if (listeners === undefined) {
            listeners = [];
            this._listeners[key as string] = listeners;
        }
        listeners.push(value);
    }

    protected _dispatchEvent<K extends keyof M>(
        key: K,
        ...args: Parameters<M[K]>
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