<script lang="ts">
    import type { Settings } from "./settings";
    import { StreamCodec, ReplayGainMode } from "../core/track";
    import { getSettings, saveSettings, newSettings } from "./settings";
    import Modal from "./Modal.svelte";

    let {
        open = $bindable(false),
        onSave,
    }: {
        open: boolean;
        onSave: (settings: Settings) => void;
    } = $props();

    let codec = $state<StreamCodec>(StreamCodec.Vorbis);
    let targetMetricType = $state<"quality" | "bit-rate">("quality");
    let targetMetricValue = $state("");
    let replayGainMode = $state<ReplayGainMode | "auto">("auto");
    let preventClipping = $state(false);
    let desktopNotifications = $state(false);
    let mediaSessionNotifications = $state(false);

    const showTargetMetric = $derived(codec === StreamCodec.Vorbis || codec === StreamCodec.Mp3);

    const targetMetricMin = $derived.by(() => {
        if (targetMetricType === "quality") {
            if (codec === StreamCodec.Vorbis) return -1;
            if (codec === StreamCodec.Mp3) return 0;
        } else if (targetMetricType === "bit-rate") {
            if (codec === StreamCodec.Vorbis || codec === StreamCodec.Mp3) return 1;
        }
        return undefined;
    });

    const targetMetricMax = $derived.by(() => {
        if (targetMetricType === "quality") {
            if (codec === StreamCodec.Vorbis) return 10;
            if (codec === StreamCodec.Mp3) return 9.999;
        } else if (targetMetricType === "bit-rate") {
            if (codec === StreamCodec.Vorbis || codec === StreamCodec.Mp3) return 1000;
        }
        return undefined;
    });

    function loadFromSettings(): void {
        const settings = getSettings();
        codec = settings.streamConfig.codec ?? StreamCodec.Vorbis;

        if (settings.streamConfig.quality !== undefined) {
            targetMetricType = "quality";
            targetMetricValue = "" + settings.streamConfig.quality;
        } else if (settings.streamConfig.kbitRate !== undefined) {
            targetMetricType = "bit-rate";
            targetMetricValue = "" + settings.streamConfig.kbitRate;
        } else {
            targetMetricType = "quality";
            targetMetricValue = "";
        }

        replayGainMode = settings.streamConfig.replayGain ?? "auto";
        preventClipping = !!settings.streamConfig.preventClipping;
        desktopNotifications = !!settings.desktopNotifications;
        mediaSessionNotifications = !!settings.mediaSessionNotifications;
    }

    function handleSave(): void {
        const settings = newSettings();
        settings.streamConfig.codec = codec;

        if (targetMetricMin !== undefined && targetMetricMax !== undefined) {
            const value = parseFloat(targetMetricValue);
            if (targetMetricType === "quality") {
                if (value >= targetMetricMin && value <= targetMetricMax) {
                    settings.streamConfig.quality = value;
                }
            } else if (targetMetricType === "bit-rate") {
                if (value >= targetMetricMin && value <= targetMetricMax) {
                    settings.streamConfig.kbitRate = value;
                }
            }
        }

        settings.streamConfig.replayGain = replayGainMode;
        settings.streamConfig.preventClipping = preventClipping;
        settings.desktopNotifications = desktopNotifications;
        settings.mediaSessionNotifications = mediaSessionNotifications;

        saveSettings(settings);

        if (
            desktopNotifications &&
            "Notification" in window &&
            Notification.permission !== "granted"
        ) {
            Notification.requestPermission();
        }

        onSave(settings);
    }

    $effect(() => {
        if (open) {
            loadFromSettings();
        }
    });
</script>

<Modal bind:open>
    <div class="modal-box bg-base-200">
        <h2 class="text-lg font-bold">Settings</h2>

        <fieldset class="fieldset bg-base-100 border border-base-300 rounded-box p-2">
            <legend class="fieldset-legend text-base">Stream Encoding</legend>
            <table class="table">
                <tbody>
                    <tr>
                        <td>Codec</td>
                        <td>
                            <select class="select" name="codec" bind:value={codec}>
                                {#each Object.values(StreamCodec) as c (c)}
                                    <option value={c}>{c}</option>
                                {/each}
                            </select>
                        </td>
                    </tr>
                    {#if showTargetMetric}
                        <tr>
                            <td>
                                <select
                                    class="select"
                                    name="metric-type"
                                    bind:value={targetMetricType}
                                >
                                    <option value="quality">Quality</option>
                                    <option value="bit-rate">Bit rate (kb/s)</option>
                                </select>
                            </td>
                            <td>
                                <fieldset class="fieldset">
                                    <input
                                        class="input"
                                        type="number"
                                        name="metric"
                                        bind:value={targetMetricValue}
                                        min={targetMetricMin}
                                        max={targetMetricMax}
                                    />
                                    {#if targetMetricType === "quality" && codec === StreamCodec.Vorbis}
                                        <p class="label">Range: 0 (best) to 9.999 (worst).</p>
                                    {/if}
                                    {#if targetMetricType === "quality" && codec === StreamCodec.Mp3}
                                        <p class="label">Range: -1 (worst) to 10 (best).</p>
                                    {/if}
                                </fieldset>
                            </td>
                        </tr>
                    {/if}
                </tbody>
            </table>
        </fieldset>

        <fieldset class="fieldset bg-base-100 border border-base-300 rounded-box p-2">
            <legend class="fieldset-legend text-base">ReplayGain</legend>
            <table class="table">
                <tbody>
                    <tr>
                        <td>Mode</td>
                        <td>
                            <select
                                class="select"
                                name="replaygain-mode"
                                bind:value={replayGainMode}
                            >
                                <option value="auto">auto</option>
                                {#each Object.values(ReplayGainMode) as mode (mode)}
                                    <option value={mode}>{mode}</option>
                                {/each}
                            </select>
                        </td>
                    </tr>
                    {#if replayGainMode !== ReplayGainMode.Off}
                        <tr>
                            <td colspan="2">
                                <label class="flex items-center gap-2">
                                    <input
                                        type="checkbox"
                                        class="toggle toggle-primary"
                                        bind:checked={preventClipping}
                                    />
                                    Prevent clipping
                                </label>
                            </td>
                        </tr>
                    {/if}
                </tbody>
            </table>
        </fieldset>

        <fieldset class="fieldset bg-base-100 border border-base-300 rounded-box p-2">
            <legend class="fieldset-legend text-base">Notifications</legend>
            <table class="table">
                <tbody>
                    <tr>
                        <td colspan="2">
                            <label class="flex items-center gap-2">
                                <input
                                    type="checkbox"
                                    class="toggle toggle-primary"
                                    bind:checked={desktopNotifications}
                                />
                                When next track plays
                            </label>
                        </td>
                    </tr>
                    <tr>
                        <td colspan="2">
                            <label class="flex items-center gap-2 pl-6">
                                <input
                                    type="checkbox"
                                    class="toggle toggle-primary"
                                    bind:checked={mediaSessionNotifications}
                                    disabled={!desktopNotifications}
                                />
                                When changing track via media controls
                            </label>
                        </td>
                    </tr>
                </tbody>
            </table>
        </fieldset>

        <button class="btn btn-soft btn-primary btn-block mt-5" type="button" onclick={handleSave}
            >Save</button
        >
    </div>
</Modal>
