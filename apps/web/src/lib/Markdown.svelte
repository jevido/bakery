<script lang="ts" module>
  import DOMPurify from 'dompurify'
  import { marked } from 'marked'

  // Every link a description or Comment holds opens beside the dashboard and
  // tells the other site nothing about where it came from. The hook is
  // DOMPurify's own, global to it, so it is added once here.
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (node.tagName === 'A' && node.hasAttribute('href')) {
      node.setAttribute('target', '_blank')
      node.setAttribute('rel', 'noreferrer')
    }
  })

  /** Markdown as sanitized HTML: GitHub's flavour, with single line breaks kept. */
  export function renderMarkdown(source: string): string {
    const html = marked.parse(source, { gfm: true, breaks: true, async: false })
    return DOMPurify.sanitize(html, { ADD_ATTR: ['target'] })
  }
</script>

<script lang="ts">
  // Paperclip's MarkdownBody (ui/src/components/MarkdownBody.tsx and the
  // .paperclip-markdown rules of ui/src/index.css; MIT, see NOTICE): a
  // description or Comment rendered from Markdown with the theme's tokens.
  // Rendered by marked and sanitized by DOMPurify in place of
  // react-markdown; its issue references, mention chips, Mermaid diagrams
  // and code-block actions are left out.
  let { source, class: className = '' }: { source: string; class?: string } = $props()

  const html = $derived(renderMarkdown(source))
</script>

<div class={['markdown min-w-0 max-w-full overflow-hidden break-words', className]}>
  <!-- Sanitized by DOMPurify in renderMarkdown. -->
  {@html html}
</div>

<style>
  .markdown {
    color: var(--foreground);
    font-size: 0.9375rem;
    line-height: 1.6;
  }
  .markdown :global {
    > :first-child {
      margin-top: 0;
    }
    > :last-child {
      margin-bottom: 0;
    }
    :where(p, ul, ol, blockquote, pre, table) {
      margin-top: 0.7rem;
      margin-bottom: 0.7rem;
    }
    :where(ul, ol) {
      padding-left: 1.5rem;
    }
    ul {
      list-style-type: disc;
    }
    ol {
      list-style-type: decimal;
      padding-left: 2.5rem;
    }
    li {
      margin: 0.14rem 0;
      padding-left: 0.2rem;
    }
    li > :where(p, ul, ol) {
      margin-top: 0.3rem;
      margin-bottom: 0.3rem;
    }
    li::marker {
      color: var(--muted-foreground);
    }
    :where(h1, h2, h3, h4) {
      margin-top: 1.75rem;
      margin-bottom: 0.45rem;
      color: var(--foreground);
      font-weight: 600;
      letter-spacing: -0.01em;
      line-height: 1.3;
    }
    h1 {
      font-size: 1.5rem;
    }
    h2 {
      font-size: 1.25rem;
    }
    h3 {
      font-size: 1.05rem;
    }
    h4 {
      font-size: 0.95rem;
    }
    :where(strong, b) {
      color: var(--foreground);
      font-weight: 600;
    }
    a {
      color: color-mix(in oklab, var(--foreground) 76%, #0969da 24%);
      text-decoration: underline;
      text-underline-offset: 0.15em;
    }
    blockquote {
      margin-left: 0;
      padding-left: 0.95rem;
      border-left: 0.24rem solid color-mix(in oklab, var(--border) 84%, var(--muted-foreground) 16%);
      color: var(--muted-foreground);
    }
    code {
      font-family: var(--font-mono);
      font-size: 1em;
    }
    :not(pre) > code {
      padding: 0.1rem 0.3rem;
      border-radius: var(--radius-sm);
      background-color: var(--muted);
    }
    pre {
      border: 1px solid var(--border);
      border-radius: var(--radius-lg);
      background-color: var(--muted);
      color: var(--foreground);
      padding: 0.5rem 0.65rem;
      overflow-x: auto;
      white-space: pre;
    }
    pre code {
      font-size: inherit;
      color: inherit;
      background: none;
    }
    hr {
      margin: 1.25rem 0;
      border-color: var(--border);
    }
    img {
      max-width: 100%;
      border-radius: calc(var(--radius) + 2px);
    }
    table {
      display: block;
      max-width: 100%;
      overflow-x: auto;
      border-collapse: collapse;
    }
    :where(th, td) {
      min-width: 8rem;
      max-width: 18rem;
      padding: 0.3rem 0.6rem;
      border: 1px solid var(--border);
      vertical-align: top;
    }
    th {
      font-weight: 600;
      text-align: left;
    }
  }
  :global(.dark) .markdown :global(a) {
    color: color-mix(in oklab, var(--foreground) 80%, #58a6ff 20%);
  }
</style>
