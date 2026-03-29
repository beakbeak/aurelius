import { mount } from 'svelte';
import LoginApp from './LoginApp.svelte';
import '../app.css';

mount(LoginApp, { target: document.getElementById('app')! });
