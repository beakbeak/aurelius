<script lang="ts">
    import type { Settings } from "./settings";
    import { StreamCodec, ReplayGainMode } from "../core/track";
    import { getSettings, saveSettings, newSettings } from "./settings";

    let {
        onSave,
    }: {
        onSave: (settings: Settings) => void;
    } = $props();

    let codec = $state<StreamCodec>(StreamCodec.Vorbis);
    let targetMetricType = $state<"quality" | "bit-rate">("quality");
    let targetMetricValue = $state("");
    let replayGainMode = $state<ReplayGainMode | "auto">("auto");
    let preventClipping = $state(false);
    let desktopNotifications = $state(false);
    let mediaSessionNotifications = $state(false);

    let showTargetMetric = $derived(codec === StreamCodec.Vorbis || codec === StreamCodec.Mp3);
    let showPreventClipping = $derived(replayGainMode !== ReplayGainMode.Off);
    let mediaSessionDisabled = $derived(!desktopNotifications);

    let targetMetricMin = $derived.by(() => {
        if (targetMetricType === "quality") {
            if (codec === StreamCodec.Vorbis) return -1;
            if (codec === StreamCodec.Mp3) return 0;
        } else if (targetMetricType === "bit-rate") {
            if (codec === StreamCodec.Vorbis || codec === StreamCodec.Mp3) return 1;
        }
        return undefined;
    });

    let targetMetricMax = $derived.by(() => {
        if (targetMetricType === "quality") {
            if (codec === StreamCodec.Vorbis) return 10;
            if (codec === StreamCodec.Mp3) return 9.999;
        } else if (targetMetricType === "bit-rate") {
            if (codec === StreamCodec.Vorbis || codec === StreamCodec.Mp3) return 1000;
        }
        return undefined;
    });

    let showVorbisQualityHelp = $derived(
        codec === StreamCodec.Vorbis && targetMetricType === "quality",
    );
    let showMp3QualityHelp = $derived(codec === StreamCodec.Mp3 && targetMetricType === "quality");

    export function loadFromSettings(): void {
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

    // Load settings when component is first rendered
    loadFromSettings();
</script>

<div class="ui__section-header">Stream Encoding</div>
<div class="ui__section-body">
    <table class="ui__table">
        <tbody>
            <tr>
                <td>Codec</td>
                <td>
                    <select bind:value={codec}>
                        {#each Object.values(StreamCodec) as c}
                            <option value={c}>{c}</option>
                        {/each}
                    </select>
                </td>
            </tr>
            {#if showTargetMetric}
                <tr>
                    <td>
                        <select bind:value={targetMetricType}>
                            <option value="quality">Quality</option>
                            <option value="bit-rate">Bit rate (kb/s)</option>
                        </select>
                    </td>
                    <td>
                        <div>
                            <input
                                type="number"
                                bind:value={targetMetricValue}
                                min={targetMetricMin}
                                max={targetMetricMax}
                            />
                        </div>
                        {#if showMp3QualityHelp}
                            <div class="settings-target-metric-help">
                                Range: 0 (best) to 9.999 (worst).
                            </div>
                        {/if}
                        {#if showVorbisQualityHelp}
                            <div class="settings-target-metric-help">
                                Range: -1 (worst) to 10 (best).
                            </div>
                        {/if}
                    </td>
                </tr>
            {/if}
        </tbody>
    </table>
</div>

<div class="ui__section-header">ReplayGain</div>
<div class="ui__section-body">
    <table class="ui__table">
        <tbody>
            <tr>
                <td>Mode</td>
                <td>
                    <select bind:value={replayGainMode}>
                        <option value="auto">auto</option>
                        {#each Object.values(ReplayGainMode) as mode}
                            <option value={mode}>{mode}</option>
                        {/each}
                    </select>
                </td>
            </tr>
            {#if showPreventClipping}
                <tr>
                    <td>Prevent clipping</td>
                    <td>
                        <label>
                            <input type="checkbox" bind:checked={preventClipping} />
                            Enabled
                        </label>
                    </td>
                </tr>
            {/if}
        </tbody>
    </table>
</div>

<div class="ui__section-header">Notifications</div>
<div class="ui__section-body">
    <table class="ui__table">
        <tbody>
            <tr>
                <td>When next track plays</td>
                <td>
                    <label>
                        <input type="checkbox" bind:checked={desktopNotifications} />
                        Enabled
                    </label>
                </td>
            </tr>
            <tr>
                <td style="padding-left: 1.5em">When changing track via media controls</td>
                <td>
                    <label>
                        <input
                            type="checkbox"
                            bind:checked={mediaSessionNotifications}
                            disabled={mediaSessionDisabled}
                        />
                        Enabled
                    </label>
                </td>
            </tr>
        </tbody>
    </table>
</div>

<button class="ui__button" type="button" onclick={handleSave}>Save</button>
