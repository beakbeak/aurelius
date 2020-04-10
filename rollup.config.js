import typescript from "@rollup/plugin-typescript";
import { terser } from "rollup-plugin-terser";

export default args => {
    return {
        input: "ts/main.ts",
        output: {
            dir: "cmd/aurelius/static/js",
            format: "iife",
        },
        plugins: args.configDebug === true ? [
            typescript({
                target: "ES2017",
            }),
        ] : [
            typescript(),
            terser(),
        ]
    };
};
