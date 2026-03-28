import { mount } from "svelte";
import App from "./App.svelte";
import "../global.css";
import "../../ui/ui.css";

mount(App, { target: document.getElementById("app")! });
