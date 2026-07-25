import { mount } from 'svelte';
import '../lib/styles.css';
import App from './App.svelte';

/**
 * Host apps pass diagnostics in on the mounting element as data attributes, so
 * the overlay works in a bare webview with no JavaScript bridge.
 */
const root = document.getElementById('app');
const data = root.dataset;

mount(App, {
  target: root,
  props: {
    appInfo: {
      appName: data.appName ?? '',
      appVersion: data.appVersion ?? '',
      os: data.os ?? '',
      platform: data.platform ?? '',
      deviceModel: data.deviceModel ?? '',
      logs: data.logs ?? '',
      logsDurationMin: Number(data.logsDurationMin ?? 0) || 0,
    },
  },
});
