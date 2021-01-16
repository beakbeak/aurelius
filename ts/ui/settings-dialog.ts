import { Settings, getSettings, saveSettings, newSettings } from "./settings";
import { StreamCodec, ReplayGainMode } from "../core/track";
import { hideModalDialog, showModalDialog } from "./modal";
import { Class } from "./class";
import { toggleClass } from "./dom";

interface SettingsElement {
    fromSettings(settings: Settings): void;
    toSettings(settings: Settings): void;
}

let settingsDialog: HTMLElement;
let saveButton: HTMLButtonElement;

const settingsElements: SettingsElement[] = [];

export function showSettingsDialog(onApply: (settings: Settings) => void): void {
    populateElements(getSettings());
    showModalDialog(settingsDialog);

    saveButton.onclick = () => {
        hideModalDialog();

        const settings = gatherSettings();
        saveSettings(settings);
        onApply(settings);
    };
}

function populateElements(settings: Settings): void {
    ensureElements();

    for (const element of settingsElements) {
        element.fromSettings(settings);
    }
}

function gatherSettings(): Settings {
    const settings = newSettings();

    for (const element of settingsElements) {
        element.toSettings(settings);
    }
    return settings;
}

function ensureElements() {
    if (settingsElements.length !== 0) {
        return;
    }

    settingsDialog = document.getElementById("settings-dialog")!;
    saveButton = document.getElementById("settings-save") as HTMLButtonElement;

    const codecElement = new CodecElement();

    settingsElements.push(codecElement);
    settingsElements.push(new TargetMetricElement(codecElement));
    settingsElements.push(new ReplayGainElement());
    settingsElements.push(new PreventClippingElement());
}

function createOption(value: string, text = value): HTMLOptionElement {
    const option = document.createElement("option");
    option.value = value;
    option.innerText = text;
    return option;
}

function populateSelectWithEnumValues<EnumType>(
    select: HTMLSelectElement,
    enumObject: EnumType,
): void {
    for (const keyString of Object.keys(enumObject)) {
        const key = keyString as keyof EnumType;
        const valueString = (enumObject[key] as unknown) as string;

        const option = createOption(valueString);
        select.appendChild(option);
    }
}

class CodecElement implements SettingsElement {
    public readonly element = document.getElementById("settings-codec") as HTMLSelectElement;
    private readonly _targetMetricRow = document.getElementById("settings-target-metric-row")!;

    public constructor() {
        populateSelectWithEnumValues(this.element, StreamCodec);

        this.element.oninput = () => {
            this._onUpdate();
        };
    }

    private _onUpdate(): void {
        const codec = this.value();
        toggleClass(
            this._targetMetricRow,
            Class.Hidden,
            !(codec === StreamCodec.Vorbis || codec === StreamCodec.Mp3),
        );
    }

    public value(): StreamCodec {
        return this.element.value as StreamCodec;
    }

    public fromSettings(settings: Settings): void {
        if (settings.streamConfig.codec !== undefined) {
            this.element.value = settings.streamConfig.codec;
        }
        this._onUpdate();
    }

    public toSettings(settings: Settings): void {
        settings.streamConfig.codec = this.value();
    }
}

class ReplayGainElement implements SettingsElement {
    public readonly element = document.getElementById("settings-replay-gain") as HTMLSelectElement;
    private readonly _preventClippingRow = document.getElementById(
        "settings-prevent-clipping-row",
    )!;

    public constructor() {
        this.element.appendChild(createOption("auto"));
        populateSelectWithEnumValues(this.element, ReplayGainMode);

        this.element.oninput = () => {
            this._onUpdate();
        };
    }

    private _onUpdate(): void {
        toggleClass(this._preventClippingRow, Class.Hidden, this.value() === ReplayGainMode.Off);
    }

    public value(): ReplayGainMode | "auto" {
        return this.element.value as ReplayGainMode | "auto";
    }

    public fromSettings(settings: Settings): void {
        if (settings.streamConfig.replayGain !== undefined) {
            this.element.value = settings.streamConfig.replayGain;
        }
        this._onUpdate();
    }

    public toSettings(settings: Settings): void {
        settings.streamConfig.replayGain = this.value();
    }
}

class PreventClippingElement implements SettingsElement {
    public readonly element = document.getElementById(
        "settings-prevent-clipping",
    ) as HTMLInputElement;

    public value(): boolean {
        return this.element.checked.valueOf();
    }

    public fromSettings(settings: Settings): void {
        this.element.checked = !!settings.streamConfig.preventClipping;
    }
    public toSettings(settings: Settings): void {
        settings.streamConfig.preventClipping = this.value();
    }
}

enum TargetMetricType {
    Quality = "quality",
    BitRate = "bit-rate",
}

class TargetMetricElement implements SettingsElement {
    public readonly typeElement = document.getElementById(
        "settings-target-metric-type",
    ) as HTMLSelectElement;
    public readonly valueElement = document.getElementById(
        "settings-target-metric-value",
    ) as HTMLInputElement;

    public constructor(private readonly _codecElement: CodecElement) {
        const update = () => {
            this._updateBounds();
            this._updateHelp();
        };

        this.typeElement.oninput = update;
        this._codecElement.element.addEventListener("input", update);

        update();
    }

    private _bounds(): { min: number; max: number } | undefined {
        const codec = this._codecElement.value();

        if (this.typeElement.value === TargetMetricType.Quality) {
            if (codec === StreamCodec.Vorbis) {
                return { min: -1, max: 10 };
            } else if (codec === StreamCodec.Mp3) {
                return { min: 0, max: 9.999 };
            }
        } else if (this.typeElement.value === TargetMetricType.BitRate) {
            if (codec === StreamCodec.Vorbis || codec === StreamCodec.Mp3) {
                return { min: 1, max: 1000 };
            }
        }
        return undefined;
    }

    private _updateBounds(): void {
        const bounds = this._bounds();
        if (bounds === undefined) {
            this.valueElement.min = "";
            this.valueElement.max = "";
            return;
        }
        this.valueElement.min = "" + bounds.min;
        this.valueElement.max = "" + bounds.max;
    }

    private _updateHelp(): void {
        const helpElements = settingsDialog.querySelectorAll(".settings-target-metric-help");
        for (let i = 0; i < helpElements.length; ++i) {
            const element = helpElements[i];
            if (element instanceof HTMLElement) {
                element.classList.add(Class.Hidden);
            }
        }

        const codec = this._codecElement.value();
        const metricType = this.typeElement.value;
        const helpElement = document.getElementById(`settings-${codec}-${metricType}-help`);

        if (helpElement !== null) {
            helpElement.classList.remove(Class.Hidden);
        }
    }

    public fromSettings(settings: Settings): void {
        if (settings.streamConfig.quality !== undefined) {
            this.typeElement.value = TargetMetricType.Quality;
            this.valueElement.value = "" + settings.streamConfig.quality;
        } else if (settings.streamConfig.kbitRate !== undefined) {
            this.typeElement.value = TargetMetricType.BitRate;
            this.valueElement.value = "" + settings.streamConfig.kbitRate;
        } else {
            this.typeElement.value = TargetMetricType.Quality;
            this.valueElement.value = "";
        }
    }

    public toSettings(settings: Settings): void {
        const bounds = this._bounds();
        if (bounds === undefined) {
            return;
        }

        const { min, max } = bounds;
        const value = parseFloat(this.valueElement.value);

        if (this.typeElement.value === TargetMetricType.Quality) {
            if (value >= min && value <= max) {
                settings.streamConfig.quality = value;
            }
        } else if (this.typeElement.value === TargetMetricType.BitRate) {
            if (value >= min && value <= max) {
                settings.streamConfig.kbitRate = value;
            }
        }
    }
}
