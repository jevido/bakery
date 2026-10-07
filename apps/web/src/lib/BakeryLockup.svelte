<script lang="ts">
  // Paperclip's PaperclipLockup (ui/src/components/PaperclipLockup.tsx; MIT,
  // see NOTICE) with The Bakery's mark (public/logo.svg) and name: one SVG in
  // currentColor, sized by a height class (`h-5 w-auto`). textLength pins the
  // wordmark's width so the viewBox never clips it whatever font loads.
  let {
    decorative = false,
    title = 'The Bakery',
    class: className,
  }: { decorative?: boolean; title?: string; class?: string } = $props()
</script>

<svg
  class={className}
  viewBox="2 4 112 17"
  fill="none"
  role={decorative ? undefined : 'img'}
  aria-hidden={decorative ? true : undefined}
  aria-label={decorative ? undefined : title}
  focusable="false"
>
  <g stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
    <path d="M3.5 19.5v-6.25C3.5 8.4 7.3 5 12 5s8.5 3.4 8.5 8.25v6.25z" />
    <path d="M8 10.5l1.75 3M11.75 9.25l1.75 3M15.5 10.5l1.5 2.75" />
  </g>
  <text
    x="27"
    y="18"
    fill="currentColor"
    font-size="15"
    font-weight="600"
    letter-spacing="-0.01em"
    textLength="86"
    lengthAdjust="spacingAndGlyphs">The Bakery</text
  >
</svg>
