// Minimal history router: a stack of paths with back/forward like the
// official client's arrows. Scroll position is remembered per entry.

type Entry = { path: string; scroll: number }

export const route = $state({
  stack: [{ path: '/', scroll: 0 }] as Entry[],
  index: 0,
})

export function current(): Entry {
  return route.stack[route.index]
}

export function navigate(path: string) {
  if (current().path === path) return
  route.stack = [...route.stack.slice(0, route.index + 1), { path, scroll: 0 }]
  route.index = route.stack.length - 1
}

export const canBack = () => route.index > 0
export const canForward = () => route.index < route.stack.length - 1
export function back() {
  if (canBack()) route.index--
}
export function forward() {
  if (canForward()) route.index++
}

export function saveScroll(y: number) {
  current().scroll = y
}

// spotify:playlist:abc -> /playlist/abc; spotify:user:x:collection -> /collection/tracks
export function uriToPath(uri: string): string {
  const p = uri.split(':')
  if (p[1] === 'collection' || p[p.length - 1] === 'collection') {
    return p[2] === 'your-episodes' ? '/collection/episodes' : '/collection/tracks'
  }
  if (p[1] === 'user' && p.length === 3) return `/user/${p[2]}`
  return `/${p[1]}/${p[2]}`
}

export function openUri(uri: string) {
  navigate(uriToPath(uri))
}
