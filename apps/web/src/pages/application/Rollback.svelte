<script lang="ts">
  // Coolify's Rollback (resources/views/livewire/project/application/rollback.blade.php
  // and app/Livewire/Project/Application/Rollback.php, Apache-2.0, see
  // NOTICE): the Images the deployments context's Image retention keeps, the
  // running one marked, each other one rolled back to without a rebuild.
  // Coolify lists `docker images` by tag; The Bakery lists the finished
  // Deployments whose Image is still on their Server, so a Rollback names
  // the Deployment it starts again.
  import { api, ApiError } from '../../lib/api'
  import { ago } from '../../lib/format'
  import Icon from '../../lib/Icon.svelte'
  import { applicationPath, go } from '../../lib/router.svelte'
  import { projectAccess } from '../../lib/projectAccess.svelte'
  import type { Application, Deployment } from '../../lib/types'
  import Button from '../../lib/ui/Button.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import Spinner from '../../lib/ui/Spinner.svelte'
  import StatusBadge from '../../lib/ui/StatusBadge.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { application }: { application: Application } = $props()

  type RetainedImage = { image: string; current: boolean; deployment: Deployment }

  let images = $state.raw<RetainedImage[]>([])
  let loading = $state(true)
  let rollingBack = $state(0)

  async function load(showToast = false) {
    loading = true
    try {
      images = (await api<{ images: RetainedImage[] }>('GET', `/applications/${application.id}/images`)).images
      if (showToast) toast.success('Images loaded.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Images not loaded', err.message)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    void application.id
    load().catch(() => {})
  })

  async function rollback(d: Deployment) {
    rollingBack = d.id
    try {
      const r = await api<{ deployment: Deployment }>('POST', `/deployments/${d.id}/rollback`)
      go(`${applicationPath(application, 'deployment')}/${r.deployment.id}`)
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      toast.error('Rollback not started', err.message)
    } finally {
      rollingBack = 0
    }
  }

  const tag = (image: string) => image.slice(image.lastIndexOf(':') + 1)
  const builtAt = (d: Deployment) => d.finished_at ?? d.created_at
  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })
</script>

<div class="chrome flex flex-col gap-6">
  <SettingsSection
    id="rollback-images-section"
    title="Available images"
    helper="Rollback uses an existing local image without rebuilding the application."
    flush
  >
    {#snippet actions()}
      <Button onclick={() => load(true)} disabled={loading}>Reload images</Button>
    {/snippet}

    {#if loading}
      <div class="flex items-center justify-center gap-2 px-4 py-10 text-[13px] text-neutral-500 dark:text-fg-dim">
        <Spinner />
        Loading available images…
      </div>
    {:else}
      {#each images as image (image.deployment.id)}
        <div
          class="flex flex-col gap-3 border-b border-neutral-200 px-4 py-3.5 last:border-b-0 sm:flex-row sm:items-center dark:border-white/[0.07]"
          data-testid="rollback-image"
        >
          <div
            class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-neutral-100 text-neutral-500 ring-1 ring-neutral-200 dark:bg-white/[0.05] dark:text-fg-dim dark:ring-white/[0.07]"
          >
            <Icon name="layers" class="size-[18px]" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <code class="truncate font-mono text-[13px] font-semibold text-black dark:text-fg" title={image.image}>{tag(image.image)}</code>
              {#if image.current}
                <StatusBadge status="Running image" type="success" />
              {/if}
            </div>
            <p class="mt-1 text-xs text-neutral-500 dark:text-fg-dim">
              Built {ago(builtAt(image.deployment))}
              <span class="mx-1 text-neutral-300 dark:text-fg-faint">·</span>
              {when.format(new Date(builtAt(image.deployment)))}
              <span class="mx-1 text-neutral-300 dark:text-fg-faint">·</span>
              Deployment #{image.deployment.id}{#if image.deployment.commit_sha}
                ({image.deployment.commit_sha.slice(0, 7)}){/if}
            </p>
          </div>
          {#if projectAccess.can('deploy')}
            {#if image.current}
              <Button disabled title="This image is currently running.">Rollback</Button>
            {:else}
              <Button loading={rollingBack === image.deployment.id} disabled={rollingBack !== 0} onclick={() => rollback(image.deployment)}
                >Roll back to this image</Button
              >
            {/if}
          {/if}
        </div>
      {:else}
        <Empty title="No rollback images" description="No previous application images are currently stored on this server." icon="layers" />
      {/each}
    {/if}
  </SettingsSection>
</div>
