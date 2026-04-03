import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
    plugins: [tailwindcss(), svelte()],
    publicDir: "cmd/aurelius/assets/static",
    base: "/static/",
    build: {
        outDir: "cmd/aurelius/assets/static",
        emptyOutDir: false,
        copyPublicDir: false,
        rollupOptions: {
            input: {
                main: "ts/apps/main/main.ts",
                login: "ts/apps/login/login.ts",
            },
            output: {
                entryFileNames: "js/[name].js",
                chunkFileNames: "js/chunks/[name]-[hash].js",
                assetFileNames: (assetInfo) => {
                    if (assetInfo.names?.[0]?.endsWith(".css")) {
                        return "css/[name][extname]";
                    }
                    return "assets/[name]-[hash][extname]";
                },
            },
        },
    },
});
