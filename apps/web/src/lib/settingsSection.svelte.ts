// Coolify's scrollToSettingsSection (resources/js/app.js, Apache-2.0, see
// NOTICE): smooth-scroll a settings section into view, then flash its border
// for 500 ms once the scroll has settled. Coolify jumps to a section on
// another page with `route#section-id`; the hash is the router's here, so the
// section waits in `pending` until that page has rendered it.

let pending = $state<string | null>(null)

export function scrollToSettingsSection(id: string) {
  const el = document.getElementById(id)
  if (!el) return
  el.classList.remove('is-section-highlight')
  let done = false
  const finish = () => {
    if (done) return
    done = true
    // Force a reflow so the highlight re-runs on repeated clicks.
    void el.offsetWidth
    el.classList.add('is-section-highlight')
    setTimeout(() => el.classList.remove('is-section-highlight'), 500)
  }
  el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  window.addEventListener('scrollend', finish, { once: true })
  // No scrollend when the section is already in view.
  setTimeout(finish, 700)
}

/** Remember a section to scroll to once the page holding it has rendered. */
export function scrollToSettingsSectionLater(id: string) {
  pending = id
}

/** Call from a page's effect once its sections are in the DOM. */
export function scrollToPendingSettingsSection() {
  const id = pending
  if (!id || !document.getElementById(id)) return
  pending = null
  requestAnimationFrame(() => scrollToSettingsSection(id))
}
