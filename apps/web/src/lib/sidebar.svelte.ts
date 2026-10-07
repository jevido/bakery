/**
 * The shell's sidebar state, as Paperclip's SidebarContext and SidebarShell
 * keep it (ui/src/context/SidebarContext.tsx, ui/src/components/SidebarShell.tsx;
 * MIT, see NOTICE): the dragged width, the collapsed icon rail, its peek,
 * and the mobile drawer. Below 768px the sidebar is a drawer and never a rail.
 *
 * Paperclip's own release retired the rail; The Bakery keeps it, under the
 * `sidebarCollapsed` key the earlier shell stored, so nobody's choice is lost.
 */

export const SIDEBAR_RAIL_WIDTH = 64
export const DEFAULT_SIDEBAR_WIDTH = 240
export const MIN_SIDEBAR_WIDTH = 208
export const MAX_SIDEBAR_WIDTH = 420
export const SIDEBAR_WIDTH_STEP = 16

/**
 * A rail item's label: clipped to no width and transparent, but still in the
 * DOM as the link's accessible name, so a row is as tall collapsed as expanded
 * (Paperclip's SIDEBAR_RAIL_HIDDEN_LABEL).
 */
export const RAIL_HIDDEN_LABEL = 'block w-0 min-w-0 overflow-hidden whitespace-nowrap text-transparent select-none'

const WIDTH_KEY = 'bakery.sidebar.width'
const COLLAPSED_KEY = 'sidebarCollapsed'

const mobileQuery = matchMedia('(max-width: 767px)')

export function clampWidth(width: number): number {
  return Math.min(MAX_SIDEBAR_WIDTH, Math.max(MIN_SIDEBAR_WIDTH, width))
}

function storedWidth(): number {
  const parsed = Number.parseInt(localStorage.getItem(WIDTH_KEY) ?? '', 10)
  return Number.isFinite(parsed) ? clampWidth(parsed) : DEFAULT_SIDEBAR_WIDTH
}

class Sidebar {
  width = $state(storedWidth())
  collapsed = $state(localStorage.getItem(COLLAPSED_KEY) === 'true')
  peeking = $state(false)
  mobile = $state(mobileQuery.matches)
  /** The mobile drawer. */
  open = $state(false)

  /** Collapsed to the icon rail and not peeking open, on desktop. */
  get rail(): boolean {
    return !this.mobile && this.collapsed && !this.peeking
  }

  constructor() {
    mobileQuery.addEventListener('change', (e) => {
      this.mobile = e.matches
      this.open = false
    })
  }

  /** Commits a width (after a drag ends, or per arrow key) and stores it. */
  setWidth(width: number, store = true) {
    this.width = clampWidth(width)
    if (store) localStorage.setItem(WIDTH_KEY, String(this.width))
  }

  /** The toggle in the breadcrumb bar and Paperclip's `[` shortcut. */
  toggle() {
    if (this.mobile) {
      this.open = !this.open
      return
    }
    this.collapsed = !this.collapsed
    this.peeking = false
    localStorage.setItem(COLLAPSED_KEY, String(this.collapsed))
  }

  /** Closes the drawer after a choice in it (a nav item, a menu action). */
  closeDrawer() {
    if (this.mobile) this.open = false
  }
}

export const sidebar = new Sidebar()
