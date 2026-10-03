// Moves an element to <body>, as Alpine's x-teleport does for Coolify's
// modals, so a transformed or blurred ancestor cannot trap `position: fixed`.
// Attach it to an element that is never the first or last node of its
// component: Svelte removes a component by walking its sibling nodes.
export function portal(node: HTMLElement) {
  document.body.appendChild(node)
  return () => node.remove()
}
