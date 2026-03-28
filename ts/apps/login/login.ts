import { mount } from 'svelte';
import LoginApp from './LoginApp.svelte';
import '../global.css';
import '../../ui/ui.css';

mount(LoginApp, { target: document.getElementById('app')! });
