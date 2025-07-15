import { postJson } from "./json";

export const enum LogLevel {
    Debug = "debug",
    Info = "info",
    Warn = "warn",
    Error = "error",
}

export async function serverLog(
    level: LogLevel,
    message: string,
    fields?: Record<string, any>,
): Promise<void> {
    const timestamp = new Date().toISOString();
    const fieldsStr = JSON.stringify(fields);
    switch (level) {
        case LogLevel.Debug:
            console.debug(timestamp, message, fieldsStr);
            break;
        case LogLevel.Info:
            console.log(timestamp, message, fieldsStr);
            break;
        case LogLevel.Warn:
            console.warn(timestamp, message, fieldsStr);
            break;
        case LogLevel.Error:
            console.error(timestamp, message, fieldsStr);
            break;
    }
    const body = {
        level,
        msg: message,
        time: timestamp,
        ...fields,
    };
    try {
        await postJson("/log", body);
    } catch (error) {
        console.error("server log failed:", error);
    }
}
