<script lang="ts">
  // The input for one Routine variable's value, matching its type, as
  // Paperclip's RoutineVariablesEditor and RoutineRunVariablesDialog draw
  // it (MIT, see NOTICE): a textarea, a True/False select, a select of its
  // options, a date input, or a number or text input. Empty is null.
  import { Input } from '@bakery/ui/components/ui/input'
  import * as Select from '@bakery/ui/components/ui/select'
  import { Textarea } from '@bakery/ui/components/ui/textarea'
  import type { RoutineVariable } from './routines'

  type Value = RoutineVariable['default_value']

  let {
    variable,
    value = $bindable(),
    id,
    label,
    unset = 'No value',
    invalid = false,
  }: { variable: RoutineVariable; value?: Value | undefined; id: string; label: string; unset?: string; invalid?: boolean } = $props()

  const UNSET = '__unset__'
  const text = $derived(value == null ? '' : String(value))
</script>

{#if variable.type === 'textarea'}
  <Textarea {id} rows={3} aria-label={label} aria-invalid={invalid} value={text} oninput={(e) => (value = e.currentTarget.value || null)} />
{:else if variable.type === 'boolean'}
  <Select.Root
    type="single"
    value={value === true ? 'true' : value === false ? 'false' : UNSET}
    onValueChange={(v) => (value = v === UNSET ? null : v === 'true')}
  >
    <Select.Trigger {id} class="w-full" aria-label={label} aria-invalid={invalid}>{value === true ? 'True' : value === false ? 'False' : unset}</Select.Trigger>
    <Select.Content>
      <Select.Item value={UNSET} label={unset} />
      <Select.Item value="true" label="True" />
      <Select.Item value="false" label="False" />
    </Select.Content>
  </Select.Root>
{:else if variable.type === 'select'}
  <Select.Root type="single" value={typeof value === 'string' && value ? value : UNSET} onValueChange={(v) => (value = v === UNSET ? null : v)}>
    <Select.Trigger {id} class="w-full" aria-label={label} aria-invalid={invalid}>{typeof value === 'string' && value ? value : unset}</Select.Trigger>
    <Select.Content>
      <Select.Item value={UNSET} label={unset} />
      {#each variable.options as option (option)}
        <Select.Item value={option} label={option} />
      {/each}
    </Select.Content>
  </Select.Root>
{:else}
  <Input
    {id}
    type={variable.type === 'number' ? 'number' : variable.type === 'date' ? 'date' : 'text'}
    aria-label={label}
    aria-invalid={invalid}
    value={text}
    placeholder={variable.type === 'number' ? '42' : variable.type === 'text' ? 'Value' : undefined}
    oninput={(e) => {
      const v = e.currentTarget.value
      value = v === '' ? null : variable.type === 'number' ? Number(v) : v
    }}
  />
{/if}
