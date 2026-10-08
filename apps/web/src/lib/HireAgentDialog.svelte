<script lang="ts">
  // Paperclip's AgentBasicsDialog step 1 (ui/src/components/new-agent/
  // AgentBasicsDialog.tsx; MIT, see NOTICE) on the dashboard's Modal: the
  // centred "Meet your next agent" heading and the large name field, with
  // The Bakery's hire fields in place of its adapter steps: Job, Title, the
  // Agent icon (the large icon above the heading is the picker, where
  // Paperclip draws its character), Reports to, Capabilities and the Roles
  // the Agent gets. The Job starts at CEO for a Guild's first Agent, else
  // General, and Reports to at the CEO, as NewAgentSetup.tsx does. A hire
  // waits for its Approval, so success shows Paperclip's "Agent submitted
  // for approval" with a link to it.
  import { Check } from '@lucide/svelte'
  import { Input } from '@bakery/ui/components/ui/input'
  import { Textarea } from '@bakery/ui/components/ui/textarea'
  import AgentIcon from '@bakery/ui/AgentIcon.svelte'
  import AgentIconPicker from './AgentIconPicker.svelte'
  import { hireAgent, jobLabel, jobs, listAgents, type Agent } from './agents'
  import { api, ApiError } from './api'
  import { canGiveAgent } from './hierarchy'
  import { refreshBadges } from './inbox.svelte'
  import { href } from './router.svelte'
  import type { GuildRole } from './types'
  import Button from './ui/Button.svelte'
  import Checkbox from './ui/Checkbox.svelte'
  import FieldError from './ui/FieldError.svelte'
  import Modal from './ui/Modal.svelte'
  import Select from './ui/Select.svelte'

  let { open = $bindable(false), onhired }: { open?: boolean; onhired?: (agent: Agent) => void } = $props()

  let name = $state('')
  let job = $state('general')
  let title = $state('')
  let icon = $state('')
  let reportsTo = $state<number | null>(null)
  let capabilities = $state('')
  let roleIds = $state<number[]>([])
  let agents = $state.raw<Agent[]>([])
  let roles = $state.raw<GuildRole[]>([])
  let saving = $state(false)
  let error = $state('')
  let errors = $state<Record<string, string>>({})
  let hired = $state<{ agent: Agent; approvalId: number } | null>(null)

  const givable = $derived(roles.filter((r) => canGiveAgent(r, roles)).toSorted((a, b) => b.position - a.position))

  // Each opening starts empty, with the Job and Manager Paperclip preselects.
  $effect(() => {
    if (!open) return
    name = ''
    title = ''
    icon = ''
    capabilities = ''
    roleIds = []
    error = ''
    errors = {}
    hired = null
    job = 'general'
    reportsTo = null
    Promise.all([listAgents('all'), api<{ roles: GuildRole[] }>('GET', '/roles')])
      .then(([as, r]) => {
        agents = as
        roles = r.roles
        job = as.length === 0 ? 'ceo' : 'general'
        reportsTo = as.find((a) => a.job === 'ceo')?.id ?? null
      })
      .catch((e) => (error = e.message))
  })

  async function submit(e?: Event) {
    e?.preventDefault()
    if (!name.trim() || saving) return
    saving = true
    error = ''
    errors = {}
    try {
      const r = await hireAgent({ name: name.trim(), job, title: title.trim(), icon, capabilities: capabilities.trim(), reports_to: reportsTo, role_ids: roleIds })
      hired = { agent: r.agent, approvalId: r.approval_id }
      refreshBadges()
      onhired?.(r.agent)
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.errors).length) errors = e.errors
      else error = e instanceof Error ? e.message : String(e)
    } finally {
      saving = false
    }
  }

  const toggleRole = (id: number, on: boolean) => (roleIds = on ? [...roleIds, id] : roleIds.filter((r) => r !== id))
</script>

<Modal bind:open variant="none" title="Hire agent">
  {#if hired}
    <div class="space-y-6 rounded-lg border border-border p-6">
      <h2 class="flex items-center gap-3 text-lg font-semibold"><Check class="size-5" />Agent submitted for approval</h2>
      <dl class="grid grid-cols-2 gap-4 text-sm">
        <dt class="text-muted-foreground">Name</dt>
        <dd>{hired.agent.name}</dd>
        <dt class="text-muted-foreground">Job</dt>
        <dd>{hired.agent.job_label}</dd>
      </dl>
      <p class="text-sm text-muted-foreground">
        The Board decides on the hire. <a class="text-foreground underline underline-offset-4" href={href(`/approvals/${hired.approvalId}`)} onclick={() => (open = false)}>View approval</a>
      </p>
    </div>
  {:else}
    <form id="hire-agent" class="flex flex-col gap-7 sm:px-4" onsubmit={submit}>
      <div class="flex flex-col items-center gap-4 text-center">
        <AgentIconPicker value={icon} onchange={(v) => (icon = v)}>
          <span class="flex size-20 items-center justify-center rounded-2xl bg-accent text-foreground">
            <AgentIcon {icon} class="size-10" />
          </span>
        </AgentIconPicker>
        <div class="space-y-2">
          <h2 class="text-3xl font-semibold tracking-tight">Meet your next agent</h2>
          <p class="text-base text-muted-foreground">Start with a name. Make them your own.</p>
        </div>
      </div>
      <div class="space-y-2">
        <label for="hire-agent-name" class="text-sm font-medium">Agent name</label>
        <!-- svelte-ignore a11y_autofocus -->
        <Input id="hire-agent-name" autofocus maxlength={100} placeholder="e.g. Darnold" bind:value={name} class="h-12 text-base" />
        <FieldError error={errors.name} />
      </div>
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <Select label="Job" bind:value={job} error={errors.job}>
            {#each jobs as j (j)}<option value={j}>{jobLabel(j)}</option>{/each}
          </Select>
        </div>
        <label class="grid gap-1.5 text-sm font-medium">
          Title
          <Input bind:value={title} maxlength={200} placeholder="e.g. Backend lead" />
          <FieldError error={errors.title} />
        </label>
      </div>
      <div>
        <Select label="Reports to" value={reportsTo ?? ''} onchange={(e) => (reportsTo = e.currentTarget.value ? Number(e.currentTarget.value) : null)} error={errors.reports_to}>
          <option value="">No one</option>
          {#each agents as a (a.id)}<option value={a.id}>{a.name} ({a.job_label})</option>{/each}
        </Select>
      </div>
      <label class="grid gap-1.5 text-sm font-medium">
        Capabilities
        <Textarea bind:value={capabilities} rows={3} placeholder="What this agent can do" />
        <FieldError error={errors.capabilities} />
      </label>
      <fieldset class="space-y-2">
        <legend class="text-sm font-medium">Roles</legend>
        {#if givable.length === 0}
          <p class="text-sm text-muted-foreground">No Roles below yours to give. The agent holds only @everyone.</p>
        {:else}
          <div class="grid gap-1 sm:grid-cols-2">
            {#each givable as role (role.id)}
              <Checkbox label={role.name} checked={roleIds.includes(role.id)} instantSave={(on) => toggleRole(role.id, on)} />
            {/each}
          </div>
        {/if}
        <FieldError error={errors.role_ids} />
      </fieldset>
      {#if error}<p class="text-sm text-destructive">{error}</p>{/if}
    </form>
  {/if}
  {#snippet footer()}
    {#if hired}
      <Button onclick={() => (open = false)}>Done</Button>
    {:else}
      <Button onclick={() => (open = false)}>Cancel</Button>
      <Button variant="highlighted" type="submit" form="hire-agent" disabled={!name.trim()} loading={saving}>{saving ? 'Hiring…' : 'Hire'}</Button>
    {/if}
  {/snippet}
</Modal>
