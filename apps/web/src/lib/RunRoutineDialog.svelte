<script lang="ts">
  // Paperclip's RoutineRunVariablesDialog (ui/src/components/
  // RoutineRunVariablesDialog.tsx; MIT, see NOTICE): one field per Routine
  // variable, typed as its definition, prefilled with its default and the
  // required ones marked, with what is still missing named beside Run. Run
  // posts the values; a 422 shows under its field, and the Routine run goes
  // to `onrun`, which does what Run does without the dialog. Left out: its
  // Agent and Project pickers and workspace choice, since a Bakery run
  // always goes to the Routine's own Agent and Project.
  import { ApiError } from './api'
  import { runRoutine, type Routine, type RoutineRun, type RoutineVariable } from './routines'
  import Button from './ui/Button.svelte'
  import Modal from './ui/Modal.svelte'
  import { toast } from './ui/toast.svelte'
  import VariableValueField from './VariableValueField.svelte'

  let {
    open = $bindable(false),
    routine,
    onrun,
  }: { open?: boolean; routine: Pick<Routine, 'id' | 'title' | 'variables'>; onrun: (run: RoutineRun) => void | Promise<void> } = $props()

  let values = $state<Record<string, RoutineVariable['default_value']>>({})
  let errors = $state<Record<string, string>>({})
  let running = $state(false)

  // Each opening starts from the defaults.
  $effect(() => {
    if (!open) return
    values = Object.fromEntries(routine.variables.map((v) => [v.name, v.default_value]))
    errors = {}
  })

  const blank = (v: unknown) => v == null || (typeof v === 'string' && v.trim() === '')
  const missing = $derived(routine.variables.filter((v) => v.required && blank(values[v.name])).map((v) => v.label || v.name))

  async function submit() {
    if (running || missing.length > 0) return
    running = true
    errors = {}
    const given: Record<string, string | number | boolean> = {}
    for (const v of routine.variables) {
      const value = values[v.name]
      if (!blank(value)) given[v.name] = value as string | number | boolean
    }
    try {
      const run = await runRoutine(routine.id, undefined, given)
      open = false
      await onrun(run)
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.errors).some((k) => k.startsWith('variables'))) errors = e.errors
      else toast.error('Routine not run', e instanceof ApiError ? (Object.values(e.errors)[0] ?? e.message) : String(e))
    } finally {
      running = false
    }
  }
</script>

<Modal bind:open variant="none" title="Run routine" subtitle="Fill in this run's variables. The routine's defaults are prefilled and won't be changed.">
  <div class="space-y-4" data-run-routine-dialog>
    {#if errors.variables}<p class="text-sm text-destructive">{errors.variables}</p>{/if}
    {#each routine.variables as v (v.name)}
      {@const error = errors[`variables.${v.name}`]}
      <div class="space-y-1.5" data-run-variable={v.name}>
        <label for="run-variable-{v.name}" class="text-xs font-medium">{v.label || v.name}{v.required ? ' *' : ''}</label>
        <VariableValueField variable={v} id="run-variable-{v.name}" label={v.label || v.name} invalid={!!error} bind:value={values[v.name]} />
        {#if error}<p class="text-xs text-destructive">{error}</p>{/if}
      </div>
    {/each}
  </div>
  {#snippet footer()}
    {#if missing.length > 0}
      <p class="text-xs text-amber-600 sm:mr-auto">Missing: {missing.join(', ')}</p>
    {/if}
    <Button disabled={running} onclick={() => (open = false)}>Cancel</Button>
    <Button variant="highlighted" disabled={missing.length > 0} loading={running} onclick={submit}>
      {running ? 'Running…' : 'Run routine'}
    </Button>
  {/snippet}
</Modal>
