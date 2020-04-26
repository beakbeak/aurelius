import typescript from "@rollup/plugin-typescript";
import { terser } from "rollup-plugin-terser";

const tsExclude = [
    "ts/**/*.spec.ts",
    "ts/testing.ts"
];

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
                exclude: tsExclude
            }),
        ] : [
            typescript({ exclude: tsExclude }),
            terser(),
        ]
    };
};
