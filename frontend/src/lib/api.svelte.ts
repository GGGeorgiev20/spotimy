import { untrack } from 'svelte'
import * as App from '../../wailsjs/go/main/App'
import type { spotify } from '../../wailsjs/go/models'

export { App }
export type Track = spotify.Track
export type Image = spotify.Image

// Images go through the Go image cache (disk-backed, size-capped).
export function img(url: string | undefined | null): string {
  return url ? `/img?u=${encodeURIComponent(url)}` : ''
}

// URL of the smallest image at least `min` px wide, else the largest.
export function pickURL(images: Image[] | undefined | null, min = 64): string {
  if (!images?.length) return ''
  const sorted = [...images].sort((a, b) => (a.width || 0) - (b.width || 0))
  const fit = sorted.find((i) => (i.width || 640) >= min) ?? sorted[sorted.length - 1]
  return fit.url
}

export const pick = (images: Image[] | undefined | null, min = 64) => img(pickURL(images, min))

// Header color: dominant color of the cover, computed by the Go side.
export async function coverColor(images: Image[] | undefined | null): Promise<string> {
  const url = pickURL(images, 64)
  if (!url) return '#535353'
  try {
    return await App.ImageColor(url)
  } catch {
    return '#535353'
  }
}

// Saved-state of library items, shared by every heart/check button.
export const saved = $state<Record<string, boolean>>({})

// The URI whose saved state a track row shows: a relinked track is liked
// under its original ID.
export const likeKey = (t: { uri: string; linkedUri?: string }) => t.linkedUri || t.uri

export async function loadSaved(uris: string[]) {
  // untrack: callers run this inside effects, which must not re-run when the
  // saved map fills in.
  const missing = untrack(() => uris.filter((u) => u && !(u in saved)))
  for (let i = 0; i < missing.length; i += 50) {
    const chunk = missing.slice(i, i + 50)
    try {
      const res = await App.AreSaved(chunk)
      chunk.forEach((u, j) => (saved[u] = res[j]))
    } catch {
      /* leave unknown */
    }
  }
}

export async function toggleSaved(uri: string) {
  const next = !saved[uri]
  saved[uri] = next
  try {
    await App.SetSaved([uri], next)
    libraryVersion.n++
  } catch (e) {
    saved[uri] = !next
    toast(`Couldn't ${next ? 'save' : 'remove'}: ${e}`)
  }
}

// Bumped whenever the library changes so the left panel reloads.
export const libraryVersion = $state({ n: 0 })

export const toasts = $state<{ id: number; text: string }[]>([])
let toastId = 0
export function toast(text: string) {
  const id = ++toastId
  toasts.push({ id, text })
  setTimeout(() => {
    const i = toasts.findIndex((t) => t.id === id)
    if (i >= 0) toasts.splice(i, 1)
  }, 3000)
}
