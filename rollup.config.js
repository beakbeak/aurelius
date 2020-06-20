import typescript from "@rollup/plugin-typescript";
import { terser } from "rollup-plugin-terser";

const tsExclude = [
    "ts/**/*.spec.ts",
    "ts/testing.ts"
];

function createConfig(input, args) {
    return {
        input: input,
        output: {
            dir: "cmd/aurelius/assets/static/js",
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
}

export default args => [
    "ts/main.ts",
    "ts/login.ts",
].map(input => createConfig(input, args));
