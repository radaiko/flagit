import { mount } from 'svelte';
import '../lib/styles.css';
import App from './App.svelte';

mount(App, { target: document.getElementById('app') });
