// @ts-ignore
import { XMLHttpRequest } from "xmlhttprequest";
// @ts-ignore
global["XMLHttpRequest"] = XMLHttpRequest;

import { env } from "process";

export let host = "http://localhost:9090/db";

if (env.AURELIUS_HOST !== undefined) {
    host = env.AURELIUS_HOST;
}
