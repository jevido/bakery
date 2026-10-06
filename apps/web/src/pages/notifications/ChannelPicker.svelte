<script lang="ts">
  // The Bakery's own: a Channel kind can have several named Notification
  // channels, where Coolify has one. With two or more, the picker sits above
  // the channel's page (a select of their names, Add channel and Delete);
  // with one, only Add channel and Delete remain, at the foot of the page,
  // so the page above looks as Coolify's does. A channel added here is
  // stored once its settings are saved or it is enabled.
  import Button from '../../lib/ui/Button.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import type { NotificationChannel } from '../../lib/types'

  let {
    channels,
    selected,
    pendingName,
    taken,
    onselect,
    onadd,
    ondelete,
  }: {
    channels: NotificationChannel[]
    /** The channel shown, 'new' while an added one is not saved yet. */
    selected: number | 'new'
    pendingName: string
    /** Names no new channel can take (every kind's channels). */
    taken: string[]
    onselect: (id: number | 'new') => void
    onadd: (name: string) => void
    ondelete: () => unknown
  } = $props()

  const picker = $derived(channels.length + (selected === 'new' ? 1 : 0) >= 2)
  const current = $derived(selected === 'new' ? null : channels.find((c) => c.id === selected))

  let adding = $state(false)
  let name = $state('')
  let nameError = $state('')

  function add(e: SubmitEvent) {
    e.preventDefault()
    const n = name.trim()
    if (taken.includes(n)) {
      nameError = 'Another channel has this name.'
      return
    }
    adding = false
    onadd(n)
  }
</script>

{#snippet buttons()}
  <Modal
    title="Add channel"
    subtitle="Another channel of this kind, with its own settings and events."
    buttonTitle="Add channel"
    bind:open={adding}
    onclose={() => (nameError = '')}
  >
    <form class="flex flex-col gap-4" onsubmit={add}>
      <Input label="Name" bind:value={name} error={nameError} required maxlength={63} data-testid="channel-name" />
      <Button type="submit" variant="highlighted">Continue</Button>
    </form>
  </Modal>
  {#if selected === 'new'}
    <Button onclick={() => onselect(channels[0]?.id ?? 'new')}>Discard</Button>
  {:else if current}
    <ConfirmationModal
      title="Delete channel?"
      buttonTitle="Delete"
      variant="error"
      actions={[`The channel ${current.name} and its deliveries will be permanently deleted. Nothing is sent to it any more.`]}
      confirmationText={current.name}
      confirmationLabel="Please confirm by entering the channel name below"
      shortConfirmationLabel="Channel name"
      onconfirm={ondelete}
    />
  {/if}
{/snippet}

{#if picker}
  <div class="flex flex-wrap items-end gap-2" data-testid="channel-picker">
    <div class="w-full max-w-72 min-w-0">
      <Select
        label="Channel"
        value={String(selected)}
        onchange={(e) => {
          const v = e.currentTarget.value
          onselect(v === 'new' ? 'new' : Number(v))
        }}
        data-testid="channel-select"
      >
        {#each channels as c (c.id)}
          <option value={String(c.id)}>{c.name}</option>
        {/each}
        {#if selected === 'new'}<option value="new">{pendingName} (not saved)</option>{/if}
      </Select>
    </div>
    {@render buttons()}
  </div>
{:else if channels.length === 1}
  <div class="flex flex-wrap items-center gap-2" data-testid="channel-more">
    {@render buttons()}
  </div>
{/if}
