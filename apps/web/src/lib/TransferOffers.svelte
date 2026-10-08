<script lang="ts">
  // The open Transfer offers to the signed-in Member, under the breadcrumb bar
  // of every page: nothing changes until they accept one here.
  import { api } from './api'
  import { Button } from '@bakery/ui/components/ui/button'
  import { session, type Offer } from './session.svelte'
  import InlineBanner from './ui/InlineBanner.svelte'

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

{#if session.offers.length > 0}
  <div class="chrome mb-4 flex flex-col gap-2">
    {#each session.offers as o (o.id)}
      <InlineBanner title="Become the Guild Master of {o.guild?.name}?" data-testid="transfer-offer">
        {o.from?.name ?? 'The Guild Master'} offers you the Guild Master of <strong>{o.guild?.name}</strong>. Nothing changes
        until you accept; they keep their Roles, and you keep yours. The offer expires on
        {until.format(new Date(o.expires_at))}.
        {#if error}<p class="mt-1 text-destructive">{error}</p>{/if}
        {#snippet actions()}
          <Button size="sm" disabled={busy} onclick={() => answer(o, 'accept')}>Accept</Button>
          <Button size="sm" variant="outline" disabled={busy} onclick={() => answer(o, 'decline')}>Decline</Button>
        {/snippet}
      </InlineBanner>
    {/each}
  </div>
{/if}
