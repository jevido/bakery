<script lang="ts">
  // Paperclip's RoutineVariablesEditor and RoutineVariablesHint
  // (ui/src/components/RoutineVariablesEditor.tsx; MIT, see NOTICE): one
  // row per `{{name}}` placeholder in the title and description, found as
  // they are typed, with its label, type, default, Required and, for a
  // select, its options; then the hint naming the built-in variables. The
  // definitions are kept in step with the placeholders as the API keeps
  // them, so a placeholder that is typed shows at once and one that is
  // deleted drops out. A 422 on `variables.<name>` shows under its row.
  import { ChevronDown, ChevronRight } from '@lucide/svelte'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import { Input } from '@bakery/ui/components/ui/input'
  import * as Select from '@bakery/ui/components/ui/select'
  import { Switch } from '@bakery/ui/components/ui/switch'
  import { builtinVariables, syncVariables, variableTypes, type RoutineVariable, type VariableType } from '../../lib/routines'
  import VariableValueField from '../../lib/VariableValueField.svelte'

  let {
    title,
    description,
    variables = $bindable(),
    errors = {},
  }: { title: string; description: string; variables: RoutineVariable[]; errors?: Record<string, string> } = $props()

  let open = $state(true)
  const synced = $derived(syncVariables(title, description, variables))

  function update(name: string, change: (v: RoutineVariable) => Partial<RoutineVariable>) {
    variables = synced.map((v) => (v.name === name ? { ...v, ...change(v) } : v))
  }

  /** Paperclip's defaultValueForType: a default that the new type cannot hold goes. */
  function retype(v: RoutineVariable, type: VariableType): Partial<RoutineVariable> {
    const keeps =
      type === 'date'
        ? typeof v.default_value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v.default_value)
        : type === 'number'
          ? typeof v.default_value === 'number'
          : type === 'text' || type === 'textarea'
            ? typeof v.default_value === 'string'
            : false
    return { type, default_value: keeps ? v.default_value : null, options: type === 'select' ? v.options : [] }
  }

  function setOptions(v: RoutineVariable, raw: string): Partial<RoutineVariable> {
    const options = raw
      .split(',')
      .map((o) => o.trim())
      .filter(Boolean)
    return { options, default_value: typeof v.default_value === 'string' && options.includes(v.default_value) ? v.default_value : null }
  }
</script>

<div class="space-y-3">
  {#if synced.length > 0}
    <div class="overflow-hidden rounded-lg border border-border/70" data-routine-variables>
      <button type="button" class="flex w-full items-center justify-between px-3 py-2 text-left" aria-expanded={open} onclick={() => (open = !open)}>
        <span>
          <span class="block text-sm font-medium">Variables</span>
          <span class="block text-xs text-muted-foreground">Detected from <code>{'{{name}}'}</code> placeholders in the title and description.</span>
        </span>
        {#if open}<ChevronDown class="size-4 text-muted-foreground" />{:else}<ChevronRight class="size-4 text-muted-foreground" />{/if}
      </button>
      {#if errors.variables}<p class="px-3 pb-2 text-xs text-destructive">{errors.variables}</p>{/if}
      {#if open}
        <div class="divide-y divide-border/70 border-t border-border/70">
          {#each synced as v (v.name)}
            {@const error = errors[`variables.${v.name}`]}
            <div class="p-4" data-routine-variable={v.name}>
              <div class="mb-3 flex flex-wrap items-center gap-2">
                <Badge variant="outline" class="font-mono text-xs">{`{{${v.name}}}`}</Badge>
                <span class="text-xs text-muted-foreground">Asked for before each manual run.</span>
              </div>
              <div class="grid gap-3 md:grid-cols-2">
                <div class="space-y-1.5">
                  <label for="variable-{v.name}-label" class="text-xs font-medium">Label</label>
                  <Input
                    id="variable-{v.name}-label"
                    value={v.label ?? ''}
                    placeholder={v.name.replaceAll('_', ' ')}
                    oninput={(e) => {
                      const label = e.currentTarget.value
                      update(v.name, () => ({ label: label || null }))
                    }}
                  />
                </div>
                <div class="space-y-1.5">
                  <span class="text-xs font-medium">Type</span>
                  <Select.Root type="single" value={v.type} onValueChange={(t) => update(v.name, (c) => retype(c, t as VariableType))}>
                    <Select.Trigger class="w-full" aria-label="Type of {v.name}">{v.type}</Select.Trigger>
                    <Select.Content>
                      {#each variableTypes as t (t)}
                        <Select.Item value={t} label={t} />
                      {/each}
                    </Select.Content>
                  </Select.Root>
                </div>
                {#if v.type === 'select'}
                  <div class="space-y-1.5">
                    <label for="variable-{v.name}-options" class="text-xs font-medium">Options</label>
                    <!-- Read on change, not on every key, so a trailing comma survives typing. -->
                    <Input
                      id="variable-{v.name}-options"
                      value={v.options.join(', ')}
                      placeholder="high, medium, low"
                      onchange={(e) => {
                        const raw = e.currentTarget.value
                        update(v.name, (c) => setOptions(c, raw))
                      }}
                    />
                  </div>
                {/if}
                <div class={['space-y-1.5', v.type !== 'select' && 'md:col-span-2']}>
                  <div class="flex items-center justify-between gap-3">
                    <label for="variable-{v.name}-default" class="text-xs font-medium">{v.type === 'select' ? 'Default option' : 'Default value'}</label>
                    <label class="flex items-center gap-2 text-xs text-muted-foreground">
                      <Switch checked={v.required} onCheckedChange={(required) => update(v.name, () => ({ required }))} aria-label="{v.name} is required" />
                      Required
                    </label>
                  </div>
                  <VariableValueField
                    variable={v}
                    id="variable-{v.name}-default"
                    label="Default of {v.name}"
                    unset="No default"
                    invalid={!!error}
                    bind:value={() => v.default_value, (d) => update(v.name, () => ({ default_value: d }))}
                  />
                </div>
              </div>
              {#if error}<p class="mt-2 text-xs text-destructive">{error}</p>{/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
  <div class="rounded-lg border border-dashed border-border/70 px-3 py-2 text-xs text-muted-foreground">
    Use <code>{'{{variable_name}}'}</code> placeholders in the title or description to ask for values when the routine runs. Built in, filled in by
    every run:
    {#each builtinVariables as b, n (b.name)}
      <span title="{b.help} For example {b.example}."><code class="text-foreground">{`{{${b.name}}}`}</code></span>{n < builtinVariables.length - 1 ? ' and ' : '.'}
    {/each}
  </div>
</div>
