import type { spotify } from '../../wailsjs/go/models'
import type { Track } from './api.svelte'

export type MenuTarget =
  | { kind: 'track'; track: Track; playlist?: { uri: string; canEdit: boolean } }
  | { kind: 'card'; card: spotify.Card; pinned?: boolean; library?: boolean; own?: boolean; description?: string; onedit?: () => void }
  | { kind: 'queue'; track: Track; index: number }

export const menu = $state({ open: false, x: 0, y: 0, target: null as MenuTarget | null })

export function openMenu(e: MouseEvent, target: MenuTarget) {
  e.preventDefault()
  e.stopPropagation()
  menu.x = e.clientX
  menu.y = e.clientY
  menu.target = target
  menu.open = true
}

export function closeMenu() {
  menu.open = false
}

// open.spotify.com link for "Copy link".
export function shareUrl(uri: string): string {
  const p = uri.split(':')
  if (p[1] === 'user' && p.length === 3) return `https://open.spotify.com/user/${p[2]}`
  return `https://open.spotify.com/${p[1]}/${p[2]}`
}
