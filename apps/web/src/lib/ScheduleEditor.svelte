<script lang="ts">
  // Paperclip's ScheduleEditor (ui/src/components/ScheduleEditor.tsx; MIT,
  // see NOTICE): a frequency preset, then the hour, minute, weekday or day of
  // the month it needs, or a Custom cron expression; under it the line that
  // says it in words. The Bakery adds the Schedule's time zone, which
  // Paperclip keeps out of the editor; it defaults to the browser's.
  import { untrack } from 'svelte'
  import { Button } from '@bakery/ui/components/ui/button'
  import { Input } from '@bakery/ui/components/ui/input'
  import * as Select from '@bakery/ui/components/ui/select'
  import {
    buildCron,
    cronFieldsMessage,
    daysOfMonth,
    daysOfWeek,
    describeSchedule,
    hours,
    minutes,
    parseCronToPreset,
    presets,
    timeZones,
    type SchedulePreset,
  } from './schedule'

  let {
    cron = $bindable(''),
    timezone = $bindable(''),
    valid = $bindable(true),
  }: { cron?: string; timezone?: string; valid?: boolean } = $props()

  const start = untrack(() => parseCronToPreset(cron))
  let preset = $state<SchedulePreset>(start.preset)
  let hour = $state(start.hour)
  let minute = $state(start.minute)
  let dayOfWeek = $state(start.dayOfWeek)
  let dayOfMonth = $state(start.dayOfMonth)
  let custom = $state(untrack(() => (start.preset === 'custom' ? cron : '')))
  const zones = timeZones()

  const customMessage = $derived(preset === 'custom' ? cronFieldsMessage(custom) : null)

  // The picks write the expression; a Custom one only once it has five fields.
  $effect(() => {
    const next = preset === 'custom' ? (customMessage ? cron : custom.trim()) : buildCron({ preset, hour, minute, dayOfWeek, dayOfMonth })
    valid = customMessage === null
    if (next !== untrack(() => cron)) cron = next
  })

  const label = (options: { value: string; label: string }[], v: string) => options.find((o) => o.value === v)?.label ?? v
</script>

<div class="space-y-3">
  <Select.Root
    type="single"
    value={preset}
    onValueChange={(v) => {
      if (v === 'custom' && preset !== 'custom') custom = cron
      preset = v as SchedulePreset
    }}
  >
    <Select.Trigger class="w-full" aria-label="Schedule frequency">{label(presets, preset)}</Select.Trigger>
    <Select.Content>
      {#each presets as p (p.value)}
        <Select.Item value={p.value} label={p.label} />
      {/each}
    </Select.Content>
  </Select.Root>

  {#if preset === 'custom'}
    <div class="space-y-1.5">
      <Input bind:value={custom} placeholder="0 10 * * *" aria-label="Cron expression" aria-invalid={customMessage !== null} class="font-mono text-sm" />
      <p class="text-xs text-muted-foreground">Five fields: minute hour day-of-month month day-of-week</p>
      {#if customMessage}<p class="text-xs text-destructive" aria-live="polite">{customMessage}</p>{/if}
    </div>
  {:else}
    <div class="flex flex-wrap items-center gap-2">
      {#if preset !== 'every_minute' && preset !== 'every_hour'}
        <span class="text-sm text-muted-foreground">at</span>
        <Select.Root type="single" bind:value={hour}>
          <Select.Trigger class="w-30" aria-label="Hour">{label(hours, hour)}</Select.Trigger>
          <Select.Content>
            {#each hours as h (h.value)}
              <Select.Item value={h.value} label={h.label} />
            {/each}
          </Select.Content>
        </Select.Root>
        <span class="text-sm text-muted-foreground">:</span>
        <Select.Root type="single" bind:value={minute}>
          <Select.Trigger class="w-20" aria-label="Minute">{label(minutes, minute)}</Select.Trigger>
          <Select.Content>
            {#each minutes as m (m.value)}
              <Select.Item value={m.value} label={m.label} />
            {/each}
          </Select.Content>
        </Select.Root>
      {/if}
      {#if preset === 'every_hour'}
        <span class="text-sm text-muted-foreground">at minute</span>
        <Select.Root type="single" bind:value={minute}>
          <Select.Trigger class="w-20" aria-label="Minute">:{label(minutes, minute)}</Select.Trigger>
          <Select.Content>
            {#each minutes as m (m.value)}
              <Select.Item value={m.value} label=":{m.label}" />
            {/each}
          </Select.Content>
        </Select.Root>
      {/if}
      {#if preset === 'weekly'}
        <span class="text-sm text-muted-foreground">on</span>
        <div class="flex gap-1">
          {#each daysOfWeek as d (d.value)}
            <Button
              type="button"
              variant={dayOfWeek === d.value ? 'default' : 'outline'}
              size="sm"
              class="h-7 px-2 text-xs"
              aria-pressed={dayOfWeek === d.value}
              onclick={() => (dayOfWeek = d.value)}
            >
              {d.label}
            </Button>
          {/each}
        </div>
      {/if}
      {#if preset === 'monthly'}
        <span class="text-sm text-muted-foreground">on day</span>
        <Select.Root type="single" bind:value={dayOfMonth}>
          <Select.Trigger class="w-20" aria-label="Day of the month">{dayOfMonth}</Select.Trigger>
          <Select.Content>
            {#each daysOfMonth as d (d.value)}
              <Select.Item value={d.value} label={d.label} />
            {/each}
          </Select.Content>
        </Select.Root>
      {/if}
    </div>
  {/if}

  <div class="flex flex-wrap items-center gap-2">
    <span class="text-sm text-muted-foreground">in</span>
    <Select.Root type="single" bind:value={timezone}>
      <Select.Trigger class="w-64" aria-label="Time zone">{timezone || 'Time zone'}</Select.Trigger>
      <Select.Content class="max-h-72">
        {#each zones as z (z)}
          <Select.Item value={z} label={z} />
        {/each}
      </Select.Content>
    </Select.Root>
  </div>
  <p class="text-xs text-muted-foreground" data-testid="schedule-description">{describeSchedule(cron)}{timezone ? ` (${timezone})` : ''}</p>
</div>
