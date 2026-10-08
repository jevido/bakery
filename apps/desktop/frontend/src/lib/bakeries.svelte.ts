/**
 * The connected Bakeries, shared by the sidebar and the page: loaded once and
 * reloaded on every `bakeries` event the Go side sends after a connect,
 * disconnect, switch or a Bakery refusing its key.
 */
import { activate, bakeries, onEvent, type Bakery } from './desktop'

class Bakeries {
  list = $state<Bakery[]>([])
  loaded = $state(false)
  error = $state('')
  active = $derived(this.list.find((b) => b.active))
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

  async switchTo(address: string) {
    await activate(address)
    await this.load()
  }
}

export const connected = new Bakeries()

/** Whether the Connect a Bakery dialog is open; the sidebar and the page both open it. */
export const connectDialog = $state({ open: false })
