import { App } from './api.svelte'
import type { spotify } from '../../wailsjs/go/models'

export const me = $state({ user: null as spotify.Me | null })

export async function loadMe() {
  try {
    me.user = await App.Me()
  } catch {
    /* top bar shows a generic avatar */
  }
}
