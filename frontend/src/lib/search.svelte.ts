import { current, navigate, replace } from './router.svelte'

// The top bar's search text. Typing updates the /search/<query>/<tab> route,
// replacing the history entry so each keystroke isn't a "back" step.
export const search = $state({ q: '' })

let timer = 0
export function setQuery(q: string) {
  search.q = q
  clearTimeout(timer)
  timer = setTimeout(() => {
    const path = q.trim() ? `/search/${encodeURIComponent(q.trim())}` : '/search'
    if (current().path.startsWith('/search')) replace(path)
    else navigate(path)
  }, 250)
}
