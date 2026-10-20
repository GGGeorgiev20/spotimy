import { mount } from 'svelte'
import './app.css'
import App from './App.svelte'
import { navigate } from './lib/router.svelte'

// Dev only: lets the browser console jump to a route.
if (import.meta.env.DEV) (window as any).nav = navigate

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
