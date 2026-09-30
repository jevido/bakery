<script lang="ts">
  import { session } from './session.svelte'
  import { size } from './format'
  import { untrack } from 'svelte'
  import { api, ApiError } from './api'
  import { href } from './router.svelte'
  import StatusBadge from './StatusBadge.svelte'
  import type { Backup, Database, S3Storage } from './types'

  // The Backups of one Database: its schedule, Back up now, and every
  // Backup with download, restore and delete. onchange hands back the
  // Database after the schedule was saved.
  let { database, onchange }: { database: Database; onchange: (d: Database) => void } = $props()

  const presets = [
    { label: 'Hourly', cron: '0 * * * *' },
    { label: 'Daily at 03:00', cron: '0 3 * * *' },
    { label: 'Weekly, Sunday 03:00', cron: '0 3 * * 0' },
  ]

  const start = untrack(() => database.backup_schedule)
  let enabled = $state(start.enabled)
  let frequency = $state(presets.find((p) => p.cron === start.cron)?.cron ?? 'custom')
  let customCron = $state(start.cron)
  let retention = $state(String(start.retention))
  let storageId = $state(start.s3_storage_id == null ? '' : String(start.s3_storage_id))
  let errors = $state<Record<string, string>>({})
  let scheduleMessage = $state('')
  let saved = $state(false)

  // null until loaded: the select only renders then, so its bound value is
  // not reset for lack of a matching option.
  let storages = $state.raw<S3Storage[] | null>(null)
  let backups = $state.raw<Backup[] | null>(null)
  let error = $state('')
  let busy = $state(false)

  async function loadBackups() {
    const r = await api<{ backups: Backup[] }>('GET', `/databases/${database.id}/backups`)
    backups = r.backups
  }
  loadBackups().catch((e) => (error = e.message))
  if (session.canSeeSecrets) {
    api<{ s3_storages: S3Storage[] }>('GET', '/s3-storages')
      .then((r) => (storages = r.s3_storages))
      .catch(() => (storages = []))
  }

  // Poll while a Backup or a Restore runs.
  let active = $derived(database.restoring || (backups ?? []).some((b) => b.status === 'running'))
  $effect(() => {
    if (!active) return
    const t = setInterval(() => loadBackups().catch(() => {}), 2000)
    return () => clearInterval(t)
  })

  async function saveSchedule(e: SubmitEvent) {
    e.preventDefault()
    errors = {}
    scheduleMessage = ''
    saved = false
    try {
      const r = await api<{ database: Database }>('PUT', `/databases/${database.id}/backup-schedule`, {
        enabled,
        cron: frequency === 'custom' ? customCron : frequency,
        retention: Number(retention),
        s3_storage_id: storageId === '' ? null : Number(storageId),
      })
      onchange(r.database)
      saved = true
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = err.errors
      if (Object.keys(err.errors).length === 0) scheduleMessage = err.message
    }
  }

  async function run(f: () => Promise<unknown>) {
    busy = true
    error = ''
    try {
      await f()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      error = err.message
    } finally {
      busy = false
      await loadBackups().catch(() => {})
    }
  }

  const backUp = () => run(() => api('POST', `/databases/${database.id}/backups`))

  function restore(b: Backup) {
    const ok = confirm(
      `Restore the backup of ${when.format(new Date(b.started_at))} into ${database.name}? It replaces the data in ${database.name} with this backup; anything written since is lost.`,
    )
    if (!ok) return
    run(async () => {
      await api('POST', `/backups/${b.id}/restore`)
      const r = await api<{ database: Database }>('GET', `/databases/${database.id}`)
      onchange(r.database)
    })
  }

  function remove(b: Backup) {
    if (!confirm('Delete this backup, on this server and in S3 storage?')) return
    run(() => api('DELETE', `/backups/${b.id}`))
  }

  function where(b: Backup): string {
    if (b.local && b.s3) return 'local + S3'
    if (b.s3) return 'S3'
    if (b.local) return 'local'
    return '—'
  }

  const when = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
  const utc = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' })
</script>

{#if !database.backups_supported}
  <p class="muted">
    Backups are not available for this engine: Redis and Valkey keep their data in an append-only file on the database's volume.
  </p>
{:else}
  <form class="form" onsubmit={saveSchedule}>
    <!-- A viewer sees the values and cannot change them. -->
    <fieldset class="contents" disabled={!session.canWrite}>
    <h2>Schedule</h2>
    <label class="check"><input type="checkbox" bind:checked={enabled} /> Back up on a schedule</label>
    <div class="row">
      <label class="field">
        <span>Frequency (UTC)</span>
        <select bind:value={frequency}>
          {#each presets as p (p.cron)}<option value={p.cron}>{p.label}</option>{/each}
          <option value="custom">Custom (cron)</option>
        </select>
      </label>
      {#if frequency === 'custom'}
        <label class="field">
          <span>Cron expression (minute hour day month weekday)</span>
          <input class="mono" bind:value={customCron} aria-invalid={errors['backup_schedule.cron'] ? 'true' : undefined} />
        </label>
      {/if}
      <label class="field">
        <span>Keep the newest</span>
        <input type="number" min="1" max="100" bind:value={() => retention, (v) => (retention = v == null ? '' : String(v))} />
      </label>
      {#if storages}
        <label class="field">
          <span>Upload to</span>
          <select bind:value={storageId}>
            <option value="">Local disk only</option>
            {#each storages as s (s.id)}<option value={String(s.id)}>{s.name} (and local disk)</option>{/each}
          </select>
        </label>
      {/if}
    </div>
    {#each Object.entries(errors) as [field, msg] (field)}<p class="error">{msg}</p>{/each}
    {#if scheduleMessage}<p class="error">{scheduleMessage}</p>{/if}
    {#if storages?.length === 0 && session.isAdmin}
      <p class="muted">To keep copies off this server, add an S3 storage under <a href={href('/settings')}>Settings</a>.</p>
    {/if}
    </fieldset>
    {#if session.canWrite}
    <div class="actions">
      {#if database.next_backup_at}
        <span class="muted" data-testid="next-backup">Next backup: {utc.format(new Date(database.next_backup_at))} UTC</span>
      {/if}
      {#if saved}<span class="ok">Saved.</span>{/if}
      <span class="spacer"></span>
      <button class="primary">Save schedule</button>
    </div>
    {/if}
  </form>

  <div class="head">
    <h2>Backups</h2>
    {#if session.canWrite}<button disabled={busy || active || database.status !== 'running'} onclick={backUp}>Back up now</button>{/if}
  </div>
  {#if database.status !== 'running'}<p class="muted">Start the database to back it up or restore it.</p>{/if}
  {#if database.restoring}<p class="muted" role="status">Restoring…</p>{/if}
  {#if database.last_restore && !database.restoring}
    {#if database.last_restore.error}
      <p class="error" role="status">The last restore failed: {database.last_restore.error}</p>
    {:else}
      <p class="ok" role="status">Restored at {when.format(new Date(database.last_restore.finished_at))}.</p>
    {/if}
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
  {#if backups === null}
    <p class="muted">Loading…</p>
  {:else if backups.length === 0}
    <p class="muted">No backups yet.</p>
  {:else}
    <table>
      <thead><tr><th>Started</th><th>By</th><th>Status</th><th>Size</th><th>Where</th><th></th></tr></thead>
      <tbody>
        {#each backups as b (b.id)}
          <tr>
            <td>{when.format(new Date(b.started_at))}</td>
            <td class="muted">{b.trigger}</td>
            <td>
              <StatusBadge status={b.status} />
              {#if b.error}<div class="error small">{b.error}</div>{/if}
            </td>
            <td>{b.status === 'running' ? '' : size(b.size_bytes)}</td>
            <td>{where(b)}</td>
            <td>
              <div class="buttons">
                {#if !session.canWrite}
                  <!-- A Backup's contents are Secrets; a viewer neither downloads nor restores it. -->
                {:else if b.status === 'succeeded'}
                  <a class="button" href={`/api/backups/${b.id}/download`} download={b.file_name}>Download</a>
                  <button disabled={busy || active || database.status !== 'running'} onclick={() => restore(b)}>Restore</button>
                {/if}
                {#if b.status !== 'running' && session.canWrite}
                  <button class="danger" disabled={busy} onclick={() => remove(b)}>Delete</button>
                {/if}
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
{/if}

<style>
  .form {
    display: grid;
    gap: 0.8rem;
    max-width: 48rem;
    margin-bottom: 1.5rem;
  }
  h2,
  p {
    margin: 0;
  }
  .row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
    gap: 0.8rem;
  }
  .field {
    display: grid;
    gap: 0.3rem;
  }
  .field span {
    font-size: 0.8rem;
    color: var(--muted);
  }
  .check {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .actions,
  .head {
    display: flex;
    gap: 0.75rem;
    align-items: center;
  }
  .head {
    justify-content: space-between;
    margin-bottom: 0.75rem;
  }
  .head + p,
  p + table {
    margin-bottom: 0.75rem;
  }
  .spacer {
    flex: 1;
  }
  .buttons {
    display: flex;
    gap: 0.4rem;
    justify-content: flex-end;
  }
  .small {
    font-size: 0.8rem;
    max-width: 24rem;
  }
  .ok {
    color: var(--ok);
  }
</style>
