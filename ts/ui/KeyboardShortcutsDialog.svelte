<script lang="ts">
    import Modal from "./Modal.svelte";

    let {
        open = $bindable(false),
    }: {
        open: boolean;
    } = $props();
</script>

<Modal bind:open>
    <div class="modal-box bg-base-200">
        <h2 class="text-lg font-bold">Keyboard Shortcuts</h2>

        <table>
            <tbody>
                <tr><th class="font-bold text-left pt-2" colspan="2">Track Selection</th></tr>
            </tbody>
            <tbody class="shortcut-group">
                <tr>
                    <td><kbd class="kbd">1</kbd> through <kbd class="kbd">0</kbd></td>
                    <td>Play track at index 1-10 in current directory</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">]</kbd> or <kbd class="kbd">w</kbd></td>
                    <td>Next track</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">[</kbd> or <kbd class="kbd">q</kbd></td>
                    <td>Previous track</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">=</kbd> or <kbd class="kbd">`</kbd></td>
                    <td>Play favorites under current directory</td>
                </tr>
            </tbody>

            <tbody>
                <tr><th class="font-bold text-left pt-4" colspan="2">Player Controls</th></tr>
            </tbody>
            <tbody class="shortcut-group">
                <tr>
                    <td><kbd class="kbd">Space</kbd></td>
                    <td>Pause/unpause playback</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">f</kbd></td>
                    <td>Toggle favorite status for currently playing track</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">'</kbd> or <kbd class="kbd">s</kbd></td>
                    <td>Seek forward 10s (30s with <kbd class="kbd">Shift</kbd>)</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">;</kbd> or <kbd class="kbd">a</kbd></td>
                    <td>Seek backward 10s (30s with <kbd class="kbd">Shift</kbd>)</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">c</kbd></td>
                    <td>Open cover image in new tab</td>
                </tr>
            </tbody>

            <tbody>
                <tr><th class="font-bold text-left pt-4" colspan="2">Navigation</th></tr>
            </tbody>
            <tbody class="shortcut-group">
                <tr>
                    <td><kbd class="kbd">Backspace</kbd></td>
                    <td>Go to top level directory</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">\</kbd></td>
                    <td>Go to parent directory</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">g</kbd></td>
                    <td>Go to current track directory</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">&lt;</kbd></td>
                    <td>Go back in browser history</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">&gt;</kbd></td>
                    <td>Go forward in browser history</td>
                </tr>
            </tbody>

            <tbody>
                <tr><th class="font-bold text-left pt-4" colspan="2">Other</th></tr>
            </tbody>
            <tbody class="shortcut-group">
                <tr>
                    <td><kbd class="kbd">/</kbd></td>
                    <td>Open search dialog</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">t</kbd></td>
                    <td>Open settings dialog</td>
                </tr>
                <tr>
                    <td><kbd class="kbd">?</kbd></td>
                    <td>Show this keyboard shortcuts dialog</td>
                </tr>
            </tbody>
        </table>
    </div>
</Modal>

<style>
    table {
        border-spacing: 0;
    }

    .shortcut-group {
        border-radius: var(--radius-box);
        box-shadow: 0 0 0 1px var(--color-base-300);
    }

    .shortcut-group td {
        background-color: var(--color-base-100);
        padding: 0.15em 0.5em;
    }

    .shortcut-group tr:first-child td:first-child {
        border-top-left-radius: var(--radius-box);
    }

    .shortcut-group tr:first-child td:last-child {
        border-top-right-radius: var(--radius-box);
    }

    .shortcut-group tr:last-child td:first-child {
        border-bottom-left-radius: var(--radius-box);
    }

    .shortcut-group tr:last-child td:last-child {
        border-bottom-right-radius: var(--radius-box);
    }
</style>
