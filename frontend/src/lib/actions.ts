// User actions shared by context menus, buttons and pages.
import { App, libraryVersion, saved, toast } from './api.svelte'
import { confirm, editDetails } from './dialog.svelte'
import { refresh } from './player.svelte'
import { current, navigate, back } from './router.svelte'
import { shareUrl } from './menu.svelte'

export async function addToQueue(uris: string[]) {
  try {
    await App.AddToQueue(uris)
    toast(uris.length === 1 ? 'Added to queue' : `Added ${uris.length} songs to queue`)
    refresh()
  } catch (e) {
    toast(`Couldn't add to queue: ${e}`)
  }
}

// Queue a whole album or playlist (first 100 tracks of a playlist).
export async function queueContext(uri: string) {
  const type = uri.split(':')[1]
  try {
    const tracks =
      type === 'album' ? (await App.Album(uri)).tracks : type === 'playlist' ? (await App.Playlist(uri, 0, 100)).tracks.items : []
    await addToQueue(tracks.filter((t) => t.playable).map((t) => t.uri))
  } catch (e) {
    toast(`Couldn't add to queue: ${e}`)
  }
}

export async function addToPlaylist(playlist: { uri: string; name: string }, uris: string[]) {
  try {
    await App.AddToPlaylist(playlist.uri, uris)
    toast(`Added to ${playlist.name}`)
  } catch (e) {
    toast(`Couldn't add to playlist: ${e}`)
  }
}

export async function createPlaylist(withUris: string[] = []) {
  try {
    const uri = await App.CreatePlaylist(`My Playlist #${(await countOwnPlaylists()) + 1}`)
    if (withUris.length) await App.AddToPlaylist(uri, withUris)
    libraryVersion.n++
    navigate(`/playlist/${uri.split(':')[2]}`)
  } catch (e) {
    toast(`Couldn't create playlist: ${e}`)
  }
}

async function countOwnPlaylists(): Promise<number> {
  try {
    return (await App.EditablePlaylists([])).length
  } catch {
    return 0
  }
}

export async function editPlaylist(uri: string, name: string, description: string, onDone?: () => void) {
  const res = await editDetails(name, description)
  if (!res) return
  try {
    await App.RenamePlaylist(uri, res.name, res.description)
    libraryVersion.n++
    onDone?.()
  } catch (e) {
    toast(`Couldn't save: ${e}`)
  }
}

export async function deletePlaylist(uri: string, name: string) {
  if (!(await confirm('Delete from Your Library?', `This will delete ${name} from Your Library.`))) return
  try {
    await App.DeletePlaylist(uri)
    libraryVersion.n++
    toast('Removed from Your Library')
    if (current().path.endsWith(uri.split(':')[2])) back()
  } catch (e) {
    toast(`Couldn't delete: ${e}`)
  }
}

export async function setPinned(uri: string, pinned: boolean) {
  try {
    await App.SetPinned(uri, pinned)
    libraryVersion.n++
  } catch (e) {
    toast(`Couldn't ${pinned ? 'pin' : 'unpin'}: ${e}`)
  }
}

export function copyLink(uri: string) {
  App.CopyText(shareUrl(uri))
  toast('Link copied to clipboard')
}

export async function toggleLibrary(uri: string) {
  const next = !saved[uri]
  saved[uri] = next
  try {
    await App.SetSaved([uri], next)
    libraryVersion.n++
    toast(next ? 'Added to Your Library' : 'Removed from Your Library')
  } catch (e) {
    saved[uri] = !next
    toast(`Couldn't update Your Library: ${e}`)
  }
}
