<script lang="ts">
  // Paperclip's NotFoundPage (ui/src/pages/NotFound.tsx; MIT, see NOTICE):
  // a card with what was asked for and the way back. A page that knows what
  // is missing (an Issue, a Goal) says so in `title` and `description`.
  import { AlertTriangle, Compass } from '@lucide/svelte'
  import { Button } from '$lib/components/ui/button'
  import * as Card from '$lib/components/ui/card'
  import { href } from '../lib/router.svelte'

  let { title = 'Page not found', description = 'This route does not exist.' }: { title?: string; description?: string } = $props()
</script>

<div class="mx-auto max-w-2xl py-10" data-testid="not-found">
  <Card.Root class="block p-6">
    <div class="flex items-center gap-3">
      <div class="rounded-md border border-destructive/20 bg-destructive/10 p-2">
        <AlertTriangle class="size-5 text-destructive" />
      </div>
      <div>
        <h1 class="text-xl font-semibold">{title}</h1>
        <p class="text-sm text-muted-foreground">{description}</p>
      </div>
    </div>
    <div class="mt-4 rounded-md border border-border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
      Requested path: <code class="font-mono">{location.hash || '#/'}</code>
    </div>
    <div class="mt-5 flex flex-wrap gap-2">
      <Button href={href('/')}><Compass class="mr-1.5 size-4" />Open dashboard</Button>
      <Button variant="outline" href={href('/projects')}>Go to projects</Button>
    </div>
  </Card.Root>
</div>
