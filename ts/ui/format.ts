import type { TrackInfo } from "../core/track";

function fileExtension(name: string): string {
    const dot = name.lastIndexOf(".");
    return dot >= 0 ? name.substring(dot + 1).toLowerCase() : "";
}

function formatBitRate(bps: number): string {
    return bps ? `${Math.round(bps / 1000)} kbps` : "";
}

export function formatDuration(totalSeconds: number): string {
    const minutes = (totalSeconds / 60) | 0;
    const seconds = (totalSeconds - minutes * 60) | 0;
    return seconds < 10 ? `${minutes}:0${seconds}` : `${minutes}:${seconds}`;
}

function formatSampleRate(hz: number): string {
    if (!hz) return "";
    const khz = hz / 1000;
    return Number.isInteger(khz) ? `${khz} kHz` : `${khz.toFixed(1)} kHz`;
}

export function formatTrackTitle(track: TrackInfo): string {
    if (track.tags["title"] !== undefined) {
        return track.tags["title"];
    }
    const dot = track.name.lastIndexOf(".");
    return dot >= 0 ? track.name.substring(0, dot) : track.name;
}

export function formatTrackArtist(track: TrackInfo): string {
    return track.tags["artist"] ?? "";
}

function formatCodec(track: TrackInfo): string {
    return track.codec || fileExtension(track.name);
}

export function formatTrackMeta(track: TrackInfo): string {
    return [
        formatCodec(track),
        formatBitRate(track.bitRate),
        track.sampleFormat,
        formatSampleRate(track.sampleRate),
    ].filter(Boolean).join(" \u00b7 ");
}
