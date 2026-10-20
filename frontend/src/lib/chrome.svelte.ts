// What the main view's sticky top bar shows for the current page: it fades in
// with the page color, title and a play button once the header scrolls away.
export const chrome = $state({
  title: '',
  color: '#333',
  contextUri: '',
  onplay: null as null | (() => void),
  threshold: 280, // scrollTop at which the bar is fully shown
})

export function setChrome(c: Partial<typeof chrome>) {
  Object.assign(chrome, { title: '', contextUri: '', onplay: null, threshold: 280 }, c)
}
