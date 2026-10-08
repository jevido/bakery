/**
 * The shown Bakery's Guilds and the person it knows this desktop as, shared by
 * the guild rail, the sidebar and the pages: loaded when the shown Bakery
 * changes and replaced by each `guilds` event, which the Go side sends when
 * its 15-second refresh finds them changed. The Guild picked last is
 * remembered per Bakery, so the app opens on it again.
 */
import { guilds as listGuilds, me, onEvent, type Guild, type GuildsEvent, type Member } from './desktop'

const rememberKey = (address: string) => `bakery-desktop.guild.${address}`

class Guilds {
  address = $state('')
  list = $state.raw<Guild[] | null>(null)
  member = $state.raw<Member | null>(null)
  error = $state('')
  #listening = false

  /** Loads the Guilds of the Bakery at address, once per Bakery shown. */
  show(address: string) {
    if (!this.#listening) {
      this.#listening = true
      onEvent<GuildsEvent>('guilds', (e) => {
        if (e.address === this.address) this.list = e.guilds ?? []
      })
    }
    if (address === this.address) return
    this.address = address
    this.list = null
    this.member = null
    this.error = ''
    void this.load()
  }

  async load() {
    const address = this.address
    try {
      const [gs, m] = await Promise.all([listGuilds(address), me(address)])
      if (address !== this.address) return
      this.list = gs
      this.member = m
      this.error = ''
    } catch (e) {
      if (address === this.address) this.error = (e as Error).message
    }
  }

  /** Forgets what was shown, so the next show loads again (after connecting anew). */
  reset() {
    this.address = ''
    this.list = null
    this.member = null
  }

  remembered(address: string): number | null {
    const v = Number(localStorage.getItem(rememberKey(address)))
    return Number.isInteger(v) && v > 0 ? v : null
  }

  remember(address: string, guildID: number) {
    localStorage.setItem(rememberKey(address), String(guildID))
  }
}

export const shownGuilds = new Guilds()
