<script lang="ts">
  // The Issue's Work products as Paperclip's artifact cards
  // (ui/src/components/artifacts/IssueArtifactCard.tsx and RichArtifactCards.tsx,
  // the pull_request and preview_url branches; MIT, see NOTICE): a Pull
  // request card with its number, title, git host and status pill, and a
  // Preview card with its link and state. The status pill is
  // RichWorkProductCard.tsx's Chip. Who opened it sits in the footer as the
  // Agent (with its icon) or the Member. The Preview card links to the
  // Application's Preview Deployments for people who may view resources.
  // Paperclip's diff, checks and review lines are left out: The Bakery does
  // not read them from the git host.
  import { ExternalLink, GitBranch, GitMerge, Globe, LoaderCircle } from '@lucide/svelte'
  import { buttonVariants } from '@bakery/ui/components/ui/button'
  import Actor from './Actor.svelte'
  import { ago } from './format'
  import { href } from './router.svelte'
  import type { WorkProduct } from './work'

  let {
    products,
    canViewApplication = false,
  }: {
    products: WorkProduct[]
    /** Whether the Preview card may link to the Application's Preview Deployments. */
    canViewApplication?: boolean
  } = $props()

  const providers: Record<string, string> = { github: 'GitHub', gitlab: 'GitLab', gitea: 'Gitea', forgejo: 'Forgejo' }

  // RichWorkProductCard.tsx's stateChipFor and its Chip: each status a
  // label and a status hue (progress, success, failure or neutral).
  const pills: Record<WorkProduct['status'], { label: string; hue: string }> = {
    open: { label: 'Open', hue: '--status-task-in_progress' },
    merged: { label: 'Merged', hue: '--status-task-done' },
    closed: { label: 'Closed', hue: '--status-task-cancelled' },
    deploying: { label: 'Deploying', hue: '--status-task-in_progress' },
    ready: { label: 'Ready', hue: '--status-task-done' },
    failed: { label: 'Failed', hue: '--status-task-blocked' },
    removed: { label: 'Removed', hue: '--status-task-cancelled' },
  }

  // The Pull request first, then its Preview.
  const sorted = $derived([...products].sort((a, b) => (a.type === b.type ? a.id - b.id : a.type === 'pull_request' ? -1 : 1)))
</script>

{#if products.length > 0}
  <section class="space-y-3" aria-label="Work products">
    <h3 class="text-sm font-medium text-muted-foreground">Work products</h3>
    <div class="grid gap-3 md:grid-cols-2">
      {#each sorted as p (p.id)}
        {@const pill = pills[p.status]}
        <article class="flex w-full flex-col overflow-hidden rounded-lg border border-border bg-card text-card-foreground" data-work-product={p.type} data-status={p.status}>
          <div class="flex flex-1 flex-col gap-4 p-5">
            <div class="flex items-center justify-between gap-2">
              <span class="flex items-center gap-2 text-xs text-muted-foreground">
                {#if p.type === 'pull_request'}
                  <GitBranch class="size-4" /> Pull request <span class="font-mono">#{p.external_id}</span>
                {:else}
                  <Globe class="size-4" /> Preview
                {/if}
              </span>
              <span class="status-chip inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-1 text-(length:--text-nano) leading-none font-medium" style:--sc="var({pill.hue})" data-pill>
                {#if p.status === 'merged'}<GitMerge class="size-3" />{/if}
                {#if p.status === 'deploying'}<LoaderCircle class="size-3 animate-spin motion-reduce:animate-none" />{/if}
                {pill.label}
              </span>
            </div>
            <h4 class="text-base leading-snug font-semibold break-words">
              {#if p.type === 'pull_request'}#{p.external_id} {p.title}{:else}{p.title}{/if}
            </h4>
            {#if p.type === 'pull_request'}
              <span class="text-xs text-muted-foreground">{providers[p.provider] ?? p.provider}</span>
            {:else if p.url}
              <a href={p.url} target="_blank" rel="noreferrer" class="font-mono text-xs break-all text-muted-foreground no-underline hover:underline" data-preview-link>{p.url}</a>
            {:else}
              <span class="text-xs text-muted-foreground">No link yet.</span>
            {/if}
          </div>
          <footer class="flex flex-wrap items-center justify-between gap-3 border-t border-border px-5 py-3">
            <span class="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
              {#if p.created_by_agent || p.created_by}
                <Actor member={p.created_by} agent={p.created_by_agent} size="sm" />
                <span>·</span>
              {/if}
              <span title={new Date(p.created_at).toLocaleString()}>{ago(p.created_at)}</span>
            </span>
            <span class="flex flex-wrap items-center gap-2">
              {#if p.type === 'preview_url' && canViewApplication}
                <a href={href(`/applications/${p.application_id}/preview-deployments`)} class={[buttonVariants({ variant: 'ghost', size: 'sm' }), 'no-underline']}>Preview deployments</a>
              {/if}
              {#if p.url}
                <a href={p.url} target="_blank" rel="noreferrer" class={[buttonVariants({ variant: 'outline', size: 'sm' }), 'no-underline']}>
                  {p.type === 'pull_request' ? 'Open pull request' : 'Open preview'}
                  <ExternalLink class="size-3" />
                </a>
              {/if}
            </span>
          </footer>
        </article>
      {/each}
    </div>
  </section>
{/if}
