// Moves an element to <body>, as Alpine's x-teleport does for Coolify's
// modals, so a transformed or blurred ancestor cannot trap `position: fixed`.
// Attach it to an element that is never the first or last node of its
// component: Svelte removes a component by walking its sibling nodes.
export function portal(node: HTMLElement) {
  document.body.appendChild(node)
  return () => node.remove()
}

/**
 * Moves an element into the element `selector` finds, as Livewire's
 * @teleport does for the Resource headings' actions (into the top bar's
 * `#resource-action-hud-slot`). The same sibling rule as portal applies.
 */
export function portalTo(selector: string) {
  return (node: HTMLElement) => {
    const target = document.querySelector(selector)
    if (!target) return
    target.appendChild(node)
    return () => node.remove()
  }
}
