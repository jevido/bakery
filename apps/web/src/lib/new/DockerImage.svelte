<script lang="ts">
  // Coolify's docker-image page (resources/views/livewire/project/new/
  // docker-image.blade.php, app/Livewire/Project/New/DockerImage.php;
  // Apache-2.0, see NOTICE): a pasted full reference is split into the name
  // and its tag or SHA256 digest, which exclude each other. The Bakery adds
  // Port (Coolify's default, 80), since its proxy needs it before the first
  // deploy; registry credentials stay on the Application page.
  import { api } from '../api'
  import { go } from '../router.svelte'
  import type { Application } from '../types'
  import Button from '../ui/Button.svelte'
  import Input from '../ui/Input.svelte'
  import { attempt } from './create'
  import { dockerImage } from './parse'

  let { environmentId, server }: { environmentId: number; server: number } = $props()

  let imageName = $state('')
  let imageTag = $state('')
  let imageSha256 = $state('')
  let port = $state(80)
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  // updatedImageName(): only a reference with a tag or digest, and only while
  // both optional fields are still empty.
  function updatedImageName() {
    if (!imageName || imageTag || imageSha256) return
    if (!imageName.includes(':') && !imageName.includes('@')) return
    const parsed = dockerImage(imageName)
    if (parsed.name === imageName) return
    imageName = parsed.name
    imageTag = parsed.tag
    imageSha256 = parsed.digest
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    const name = imageName.trim()
    const tag = imageTag.trim()
    const sha256 = imageSha256.trim().replace(/^sha256:/i, '')
    if (tag && sha256) {
      const both = 'Provide either a tag or SHA256 digest, not both.'
      errors = { imageTag: both, imageSha256: both }
      return
    }
    if (sha256 && !/^[a-f0-9]{64}$/i.test(sha256)) {
      errors = { imageSha256: 'The SHA256 digest must be 64 hexadecimal characters.' }
      return
    }
    busy = true
    const created = await attempt(
      () =>
        api<{ application: Application }>('POST', `/environments/${environmentId}/applications`, {
          build_pack: 'dockerimage',
          docker_image: sha256 ? `${name}@sha256:${sha256}` : `${name}:${tag || 'latest'}`,
          port: Number(port),
          server_id: server,
        }),
      ['docker_image', 'port'],
      (e) => (errors = e),
    )
    busy = false
    if (created) go(`/applications/${created.application.id}`)
  }
</script>

<div class="chrome mt-8 w-full max-w-[920px] lg:mt-3">
  <!-- An edit clears the messages of the last attempt. -->
  <form onsubmit={submit} oninput={() => (errors = {})}>
    <section class="application-settings-section">
      <div class="application-settings-section-header">
        <div>
          <h2>Docker image</h2>
          <p>Deploy an existing image from Docker Hub or another OCI registry.</p>
        </div>
        <Button type="submit" variant="highlighted" loading={busy}>Create application</Button>
      </div>
      <div class="application-settings-section-body space-y-4">
        <!-- svelte-ignore a11y_autofocus -->
        <Input
          label="Image name"
          placeholder="nginx, ghcr.io/user/app:v1.2.3, or nginx:stable@sha256:…"
          helper="Paste a complete image reference, or enter a name and use one of the optional fields below."
          required
          autofocus
          bind:value={imageName}
          oninput={(e) => {
            if (e instanceof InputEvent && e.inputType === 'insertFromPaste') updatedImageName()
          }}
          onchange={updatedImageName}
          error={errors.imageName ?? errors.docker_image}
        />
        <div
          class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] sm:items-end"
          role="group"
          aria-label="Tag and SHA256 digest are mutually exclusive"
        >
          <Input label="Tag" placeholder="latest" helper="Use a mutable tag such as latest or v1.2.3." bind:value={imageTag} error={errors.imageTag} />
          <div class="flex items-center justify-center text-xs font-semibold text-neutral-400 sm:h-9 dark:text-fg-faint">
            <span>OR</span>
          </div>
          <Input
            label="SHA256 digest"
            placeholder="59e02939b1bf39f16c93138a28727aec…"
            helper="Use the 64-character digest without the sha256: prefix."
            bind:value={imageSha256}
            error={errors.imageSha256}
          />
        </div>
        <div class="sm:max-w-[calc(50%-0.375rem)]">
          <Input type="number" label="Port" required helper="Port the application listens on." bind:value={port} error={errors.port} />
        </div>
      </div>
    </section>
  </form>
</div>
