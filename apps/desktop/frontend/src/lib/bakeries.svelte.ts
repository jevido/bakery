/**
 * The connected Bakeries, shared by the sidebar and the page: loaded once and
 * reloaded on every `bakeries` event the Go side sends after a connect,
 * disconnect, switch or a Bakery refusing its key.
 */
import { activate, bakeries, onEvent, type Bakery } from './desktop'
import { go, router } from './router.svelte'

class Bakeries {
  list = $state<Bakery[]>([])
  loaded = $state(false)
  error = $state('')
  active = $derived(this.list.find((b) => b.active))
  /** The Bakery the route shows, by its place in the list; the active one is only the default. */
  shownIndex = $derived(router.route.bakery ?? -1)
  shown = $derived(this.list[this.shownIndex] as Bakery | undefined)
  #listening = false

  async load() {
    try {
      this.list = await bakeries()
      this.error = ''
    } catch (e) {
      this.error = (e as Error).message
    }
    this.loaded = true
  }

  /** Loads the list and keeps it current; safe to call more than once. */
  start() {
    if (this.#listening) return
    this.#listening = true
    onEvent('bakeries', () => void this.load())
    void this.load()
  }

  /** Shows the Bakery at index and makes it the active one, which the app opens on next time. */
  async switchTo(index: number) {
    go(`/b/${index}`)
    await activate(this.list[index].address)
    await this.load()
  }
}

export const connected = new Bakeries()

/** Whether the Connect a Bakery dialog is open; the sidebar and the page both open it. */
export const connectDialog = $state({ open: false })
