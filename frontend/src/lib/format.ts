export function fmtDuration(ms: number): string {
  const s = Math.floor(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = String(s % 60).padStart(2, '0')
  return h ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}

// "1 hr 23 min" / "about 5 hr" style totals used in page headers.
export function fmtTotal(ms: number): string {
  const min = Math.floor(ms / 60000)
  if (min < 60) return `${min} min ${Math.floor((ms / 1000) % 60)} sec`
  const h = Math.floor(min / 60)
  return h >= 10 ? `about ${h} hr` : `${h} hr ${min % 60} min`
}

// Length of a long list: exact once every row is loaded, otherwise estimated
// from the loaded rows, which is what the official client's "about" means.
export function fmtListLength(loaded: { durationMs: number }[], total: number): string {
  if (!loaded.length) return ''
  const ms = loaded.reduce((s, t) => s + t.durationMs, 0)
  if (loaded.length >= total) return fmtTotal(ms)
  const est = fmtTotal((ms / loaded.length) * total)
  return est.startsWith('about') ? est : `about ${est}`
}

const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

// Date added: relative for the last few weeks, otherwise "Feb 12, 2026".
export function fmtAdded(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  const days = Math.floor((Date.now() - d.getTime()) / 86400000)
  if (days < 1) {
    const h = Math.floor((Date.now() - d.getTime()) / 3600000)
    return h < 1 ? 'just now' : `${h} hour${h > 1 ? 's' : ''} ago`
  }
  if (days < 7) return `${days} day${days > 1 ? 's' : ''} ago`
  if (days < 28) {
    const w = Math.floor(days / 7)
    return `${w} week${w > 1 ? 's' : ''} ago`
  }
  return `${months[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`
}

export function fmtNumber(n: number | string): string {
  return Number(n).toLocaleString('en-US')
}

// Darken/limit a header color so white text stays readable, like Spotify's
// extracted "dark" variant.
export function headerColor(hex: string): string {
  const n = parseInt(hex.slice(1), 16)
  let r = (n >> 16) & 255,
    g = (n >> 8) & 255,
    b = n & 255
  const lum = 0.299 * r + 0.587 * g + 0.114 * b
  const f = lum > 120 ? 120 / lum : 1
  r = Math.round(r * f * 0.85)
  g = Math.round(g * f * 0.85)
  b = Math.round(b * f * 0.85)
  return `rgb(${r}, ${g}, ${b})`
}

// Lyrics background: keep the cover's hue vivid, just bring luminance into a
// mid band so both the dark upcoming lines and white active line read well.
export function lyricsColor(hex: string): string {
  const n = parseInt(hex.slice(1), 16)
  const [r, g, b] = [(n >> 16) & 255, (n >> 8) & 255, n & 255]
  const lum = 0.299 * r + 0.587 * g + 0.114 * b || 1
  const f = Math.min(Math.max(lum, 90), 130) / lum
  const c = (v: number) => Math.min(255, Math.round(v * f))
  return `rgb(${c(r)}, ${c(g)}, ${c(b)})`
}
