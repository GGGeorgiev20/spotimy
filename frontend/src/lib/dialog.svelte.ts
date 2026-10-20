// Minimal modal dialogs: confirm and the playlist "Edit details" form.
type Confirm = { kind: 'confirm'; title: string; body: string; ok: string; resolve: (v: boolean) => void }
type Edit = { kind: 'edit'; name: string; description: string; resolve: (v: { name: string; description: string } | null) => void }

type Credits = { kind: 'credits'; track: string; uri: string; resolve: (v: null) => void }

export const dialog = $state({ current: null as Confirm | Edit | Credits | null })

export function showCredits(track: string, uri: string): Promise<null> {
  return new Promise((resolve) => (dialog.current = { kind: 'credits', track, uri, resolve }))
}

export function confirm(title: string, body: string, ok = 'Delete'): Promise<boolean> {
  return new Promise((resolve) => (dialog.current = { kind: 'confirm', title, body, ok, resolve }))
}

export function editDetails(name: string, description: string): Promise<{ name: string; description: string } | null> {
  return new Promise((resolve) => (dialog.current = { kind: 'edit', name, description, resolve }))
}
