import { loadSaved, type Track } from './api.svelte'

export type Page<T> = { total: number; items: T[] }

// A long list loaded page by page as a virtual list scrolls. Track lists also
// look up saved state for each loaded page.
export class Paged<T = Track> {
  items: (T | undefined)[] = $state([])
  total = $state(0)
  error = $state('')
  #inflight = new Set<number>()

  constructor(
    private fetch: (offset: number, limit: number) => Promise<Page<T>>,
    private size = 100,
    private onPage: (items: T[]) => void = (items) => loadSaved((items as Track[]).map((t) => t.uri)),
  ) {}

  // Seed with the first page that came with the page header.
  seed(page: Page<T>) {
    this.total = page.total
    this.items = Array.from({ length: page.total }, (_, i) => page.items[i])
    this.onPage(page.items)
  }

  ensure(start: number, end: number) {
    for (let p = Math.floor(start / this.size); p * this.size < Math.min(end, this.total); p++) {
      const off = p * this.size
      if (this.items[off] === undefined && !this.#inflight.has(p)) this.#load(p)
    }
  }

  async #load(p: number) {
    this.#inflight.add(p)
    try {
      const page = await this.fetch(p * this.size, this.size)
      page.items.forEach((t, i) => (this.items[p * this.size + i] = t))
      this.onPage(page.items)
    } catch (e) {
      this.error = String(e)
    } finally {
      this.#inflight.delete(p)
    }
  }

  get loaded(): T[] {
    return this.items.filter((t): t is T => t !== undefined)
  }
}
