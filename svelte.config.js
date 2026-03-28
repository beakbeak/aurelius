import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

export default {
    preprocess: vitePreprocess(),
    compilerOptions: {
        warningFilter: (warning) => {
            const ignore = [
                "a11y_click_events_have_key_events",
                "a11y_invalid_attribute",
                "a11y_no_noninteractive_element_interactions",
                "a11y_no_noninteractive_tabindex",
                "a11y_no_static_element_interactions",
            ];
            return !ignore.includes(warning.code);
        },
    },
};
