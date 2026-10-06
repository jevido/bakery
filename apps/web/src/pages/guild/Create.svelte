<script lang="ts">
  // Coolify's New Team form (resources/views/livewire/team/create.blade.php,
  // app/Livewire/Team/Create.php; Apache-2.0, see NOTICE): Name and
  // Description. The new Guild becomes the Current guild and its creator its
  // admin, and the Dashboard opens in it.
  import { api, ApiError } from '../../lib/api'
  import { go } from '../../lib/router.svelte'
  import { session } from '../../lib/session.svelte'
  import Button from '../../lib/ui/Button.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import { toast } from '../../lib/ui/toast.svelte'

  let { oncreated }: { oncreated?: () => void } = $props()

  let name = $state('')
  let description = $state('')
  let errors = $state<Record<string, string>>({})
  let busy = $state(false)

  async function create(e: SubmitEvent) {
    e.preventDefault()
    busy = true
    errors = {}
    try {
      await api('POST', '/guilds', { name, description })
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
      return
    } finally {
      busy = false
    }
    await session.refresh()
    toast.success('Guild created.')
    oncreated?.()
    go('/')
  }
</script>

<form class="application-settings-form flex w-full flex-col gap-4" onsubmit={create} data-testid="guild-create">
  <Input label="Name" bind:value={name} error={errors.name} required />
  <Input label="Description" bind:value={description} error={errors.description} />
  <div class="flex justify-end">
    <Button type="submit" variant="highlighted" loading={busy}>Create guild</Button>
  </div>
</form>
