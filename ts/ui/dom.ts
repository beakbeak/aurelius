/**
 * Set up temporary event listeners to handle a single touch- or mouse-based
 * drag operation.
 *
 * @param onMove A function to be called when the mouse or touch position moves.
 * @param onStop A function to be called when the drag operation stops.
 * @param touchId The identifier of the touch that started the drag operation.
 * If `undefined`, it is a mouse-based drag operation.
 */
export function onDrag(
    onMove: (x: number, y: number) => void,
    onStop: (x: number, y: number) => void,
    touchId?: number,
): void {
    if (touchId !== undefined) {
        const onTouchMove = (e: TouchEvent): void => {
            for (const touch of e.changedTouches) {
                if (touch.identifier === touchId) {
                    onMove(touch.screenX, touch.screenY);
                    break;
                }
            }
        };
        const onTouchEnd = (e: TouchEvent): void => {
            for (const touch of e.changedTouches) {
                if (touch.identifier === touchId) {
                    onStop(touch.screenX, touch.screenY);

                    document.removeEventListener("touchmove", onTouchMove);
                    document.removeEventListener("touchend", onTouchEnd);
                    document.removeEventListener("touchcancel", onTouchEnd);
                    break;
                }
            }
        };

        document.addEventListener("touchmove", onTouchMove);
        document.addEventListener("touchend", onTouchEnd);
        document.addEventListener("touchcancel", onTouchEnd);
    } else {
        const onMouseMove = (e: MouseEvent): void => {
            onMove(e.screenX, e.screenY);
        };
        const onMouseUp = (e: MouseEvent): void => {
            onStop(e.screenX, e.screenY);

            document.removeEventListener("mousemove", onMouseMove);
            document.removeEventListener("mouseup", onMouseUp);
        };

        document.addEventListener("mousemove", onMouseMove);
        document.addEventListener("mouseup", onMouseUp);
    }
}
