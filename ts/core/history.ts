import { PlaylistItem } from "./playlist";

export class PlayHistory {
    private static readonly _maxLength = 1024;

    private _items: PlaylistItem[] = [];
    private _index = 0;

    public push(item: PlaylistItem): void {
        this._items.splice(this._index + 1, this._items.length - (this._index + 1), item);
        if (this._items.length > PlayHistory._maxLength) {
            this._items.shift();
        } else if (this._items.length > 1) {
            ++this._index;
        }
    }

    public pushFront(item: PlaylistItem): void {
        this._items.splice(this._index, 0, item);
        if (this._items.length > PlayHistory._maxLength) {
            this._items.pop();
        }
    }

    public hasPrevious(): boolean {
        return this._items.length > 1 && this._index > 0;
    }

    public hasNext(): boolean {
        return this._index < this._items.length - 1;
    }

    public previous(): PlaylistItem | undefined {
        if (this._index === 0) {
            return undefined;
        }
        --this._index;
        return this._items[this._index];
    }

    public next(): PlaylistItem | undefined {
        if (this._index >= this._items.length - 1) {
            return undefined;
        }
        ++this._index;
        return this._items[this._index];
    }
}
