import { StreamConfig, StreamCodec } from "../core/track";
import { copyJson } from "../core/json";

const SettingsStorageKey = "settings";

export interface Settings {
    streamConfig: StreamConfig;
}

export function newSettings(): Settings {
    return {
        streamConfig: {},
    };
}

function defaultSettings(): Settings {
    return {
        streamConfig: {
            codec: StreamCodec.Vorbis,
            quality: 8,
            preventClipping: false,
        },
    };
}

let settingsObj: Settings | undefined;

export function getSettings(): Settings {
    if (settingsObj !== undefined) {
        return settingsObj;
    }
    settingsObj = loadSettings() || defaultSettings();
    return settingsObj;
}

function loadSettings(): Settings | undefined {
    const settingsJson = localStorage.getItem(SettingsStorageKey);
    if (settingsJson === null) {
        return undefined;
    }
    return JSON.parse(settingsJson);
}

export function saveSettings(settings: Settings): void {
    settingsObj = copyJson(settings);
    localStorage.setItem(SettingsStorageKey, JSON.stringify(settings));
}
