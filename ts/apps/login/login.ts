import { mount } from 'svelte';
import LoginApp from './LoginApp.svelte';
import '../tailwind.css';
import '../global.css';

mount(LoginApp, { target: document.getElementById('app')! });
