<script lang="ts">
  // The open Transfer offers to the signed-in Member, under the top bar of
  // every page: nothing changes until they accept one here.
  import { api } from './api'
  import { session, type Offer } from './session.svelte'
  import Callout from './ui/Callout.svelte'

  let busy = $state(false)
  let error = $state('')

  const until = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })

  async function answer(o: Offer, how: 'accept' | 'decline') {
    busy = true
    error = ''
    try {
      await api('POST', `/guild-master-offers/${o.id}/${how}`)
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      busy = false
    }
    await session.refresh()
  }
</script>

{#each session.offers as o (o.id)}
  <Callout type="info" title="Become the Guild Master of {o.guild?.name}?" class="mb-4">
    <div data-testid="transfer-offer">
      <p>
        {o.from?.name ?? 'The Guild Master'} offers you the Guild Master of <strong>{o.guild?.name}</strong>. Nothing
        changes until you accept; they keep their Roles, and you keep yours. The offer expires on
        {until.format(new Date(o.expires_at))}.
      </p>
      <div class="mt-2 flex gap-2">
        <button class="primary" disabled={busy} onclick={() => answer(o, 'accept')}>Accept</button>
        <button disabled={busy} onclick={() => answer(o, 'decline')}>Decline</button>
      </div>
      {#if error}<p class="error">{error}</p>{/if}
    </div>
  </Callout>
{/each}
