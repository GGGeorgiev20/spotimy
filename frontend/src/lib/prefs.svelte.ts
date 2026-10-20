// Per-device UI preferences, remembered in localStorage when it's available.
const defaults = {
  libSort: 'Recents',
  libView: 'list' as 'list' | 'compact' | 'grid',
  libWidth: 340,
  libCollapsed: false,
  rightPanel: '' as '' | 'queue' | 'lyrics' | 'npv' | 'devices' | 'friends',
}

function load(): typeof defaults {
  try {
    return { ...defaults, ...JSON.parse(localStorage.getItem('prefs') ?? '{}') }
  } catch {
    return { ...defaults }
  }
}

export const prefs = $state(load())

$effect.root(() => {
  $effect(() => {
    const json = JSON.stringify(prefs)
    try {
      localStorage.setItem('prefs', json)
    } catch {
      /* not persisted */
    }
  })
})
