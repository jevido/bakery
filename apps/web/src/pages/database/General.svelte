<script lang="ts">
  // Coolify's Database General page
  // (resources/views/livewire/project/database/{postgresql,mysql,mariadb,mongodb,redis}/general.blade.php,
  // status-info.blade.php and components/database-status-info.blade.php;
  // app/Livewire/Project/Database/*/General.php; Apache-2.0, see NOTICE),
  // with only the sections and fields The Bakery has a setting for. Valkey
  // follows Redis's page. The Bakery generates the credentials once and never
  // changes them, so they are shown read-only. Saving sends the Database as
  // loaded with name, description and image changed; Access saves at once,
  // as Coolify's instantSave does.
  import { untrack } from 'svelte'
  import { api, ApiError } from '../../lib/api'
  import { session } from '../../lib/session.svelte'
  import { scrollToPendingSettingsSection } from '../../lib/settingsSection.svelte'
  import type { Database, DatabaseInput, DatabaseType } from '../../lib/types'
  import CopyButton from '../../lib/ui/CopyButton.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import SettingsSection from '../../lib/ui/SettingsSection.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'

  let { database, onchange }: { database: Database; onchange: (d: Database) => void } = $props()

  // Coolify's product name in the section descriptions and its databaseLabel()
  // in the URL fields.
  const product: Record<DatabaseType, string> = {
    postgresql: 'PostgreSQL',
    mysql: 'MySQL',
    mariadb: 'MariaDB',
    mongodb: 'MongoDB',
    redis: 'Redis',
    valkey: 'Valkey',
  }
  const urlLabel: Record<DatabaseType, string> = {
    postgresql: 'Postgres',
    mysql: 'MySQL',
    mariadb: 'MariaDB',
    mongodb: 'Mongo',
    redis: 'Redis',
    valkey: 'Valkey',
  }
  const defaultPort: Record<DatabaseType, number> = {
    postgresql: 5432,
    mysql: 3306,
    mariadb: 3306,
    mongodb: 27017,
    redis: 6379,
    valkey: 6379,
  }
  const hiddenValue = 'Hidden (viewers cannot see it)'
  const urlHelper = 'The Bakery generated the credentials in this URL when it created the database; they never change.'

  const canUpdate = $derived(session.canWrite)

  let name = $state('')
  let description = $state('')
  let image = $state('')
  // A number input binds a number, or null (or undefined) while empty.
  let publicPort = $state<number | null | undefined>(null)
  let errors = $state<Record<string, string>>({})
  let saving = $state(false)
  let switching = $state(false)

  function reset() {
    name = database.name
    description = database.description ?? ''
    image = database.image
    publicPort = database.public_port
    errors = {}
  }

  $effect(() => {
    void database.id
    untrack(reset)
  })

  $effect(scrollToPendingSettingsSection)

  const isPublic = $derived(database.public_port != null)
  const portTyped = $derived(typeof publicPort === 'number')
  const dirty = $derived(
    canUpdate && (name !== database.name || description !== (database.description ?? '') || image !== database.image),
  )

  function input(change: Partial<DatabaseInput>): DatabaseInput {
    return {
      name: database.name,
      description: database.description ?? '',
      version: database.version,
      public_port: database.public_port,
      resource_limits: database.resource_limits,
      ...change,
    }
  }

  async function save() {
    if (saving || !dirty) return
    saving = true
    errors = {}
    const restart = image !== database.image
    try {
      const r = await api<{ database: Database }>('PATCH', `/databases/${database.id}`, input({ name, description, image }))
      onchange(r.database)
      reset()
      toast.success('Database updated.', restart ? 'The database restarts with the new image.' : '')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { name: err.message }
    } finally {
      saving = false
    }
  }

  // Coolify's Access listbox: Public keeps the port typed in Public port,
  // Private clears it. Both restart the database.
  async function accessChanged(value: string) {
    const wantPublic = value === 'public'
    if (wantPublic === isPublic || switching) return
    switching = true
    errors = {}
    try {
      const port = wantPublic ? (publicPort ?? null) : null
      const r = await api<{ database: Database }>('PATCH', `/databases/${database.id}`, input({ public_port: port }))
      onchange(r.database)
      toast.success(wantPublic ? 'Database is now publicly accessible.' : 'Database is no longer publicly accessible.')
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
      errors = Object.keys(err.errors).length ? err.errors : { public_port: err.message }
    } finally {
      switching = false
    }
  }

  // The credentials in Coolify's labels for each type; Redis and Valkey
  // authenticate as their default user. A viewer gets the labels only.
  type Field = { label: string; value: string; secret?: boolean }
  const credentials = $derived.by<Field[]>(() => {
    const c = database.credentials ?? { username: '', password: '', root_password: '', database_name: '' }
    switch (database.type) {
      case 'mysql':
      case 'mariadb':
        return [
          { label: 'Root password', value: c.root_password ?? '', secret: true },
          { label: 'Normal user', value: c.username },
          { label: 'Normal user password', value: c.password, secret: true },
          { label: 'Initial database', value: c.database_name },
        ]
      case 'mongodb':
        return [
          { label: 'Initial username', value: c.username },
          { label: 'Initial password', value: c.password, secret: true },
          { label: 'Initial database', value: c.database_name },
        ]
      case 'redis':
      case 'valkey':
        return [
          { label: 'Username', value: 'default' },
          { label: 'Password', value: c.password, secret: true },
        ]
      default:
        return [
          { label: 'Username', value: c.username },
          { label: 'Password', value: c.password, secret: true },
          { label: 'Initial database', value: c.database_name },
        ]
    }
  })
</script>

<form
  class="application-settings-form flex flex-col"
  onsubmit={(e) => {
    e.preventDefault()
    save()
  }}
>
  {#if canUpdate}
    <UnsavedBar {dirty} {saving} onsave={save} onreset={reset} />
  {/if}
  <div class="flex flex-col gap-6">
    <SettingsSection
      id="database-details-section"
      title="Database details"
      helper={`Manage the identity and container image for this ${product[database.type]} database.`}
    >
      <div class="grid gap-4 lg:grid-cols-2">
        <Input label="Name" bind:value={name} error={errors.name} required disabled={!canUpdate} />
        <Input label="Description" bind:value={description} error={errors.description} disabled={!canUpdate} />
        <div class="lg:col-span-2">
          <Input
            label="Image"
            bind:value={image}
            error={errors.image ?? errors.version}
            required
            helper={`Use a published ${product[database.type]} image from Docker Hub. Only its tag can change; a new image restarts the database and keeps its data.`}
            disabled={!canUpdate}
          />
        </div>
      </div>
    </SettingsSection>

    <SettingsSection
      id="credentials-section"
      title="Credentials"
      helper={`The Bakery generated these when it created the database and configured ${product[database.type]} with them. They cannot be changed.`}
    >
      <div class="grid gap-4 lg:grid-cols-2" data-testid="database-credentials">
        {#if database.secrets_hidden}
          {#each credentials as field (field.label)}
            <Input label={field.label} disabled value={hiddenValue} />
          {/each}
        {:else}
          {#each credentials as field (field.label)}
            <CopyButton label={field.label} text={field.value} secret={field.secret} />
          {/each}
        {/if}
      </div>
    </SettingsSection>

    <SettingsSection
      id="runtime-network-section"
      title="Runtime and network"
      helper="How applications and clients connect to this database."
    >
      <div class="space-y-5">
        {#if database.secrets_hidden}
          <Input label={`${urlLabel[database.type]} URL (internal)`} disabled value={hiddenValue} />
          <Input label={`${urlLabel[database.type]} URL (public)`} disabled value={hiddenValue} />
        {:else}
          <CopyButton label={`${urlLabel[database.type]} URL (internal)`} text={database.internal_url ?? ''} secret testid="internal-url" />
          {#if database.public_url}
            <CopyButton label={`${urlLabel[database.type]} URL (public)`} text={database.public_url} secret testid="public-url" />
          {/if}
          <p class="text-xs text-neutral-500 dark:text-fg-dim">
            {urlHelper} Applications in any project reach the database on the internal URL: paste it into an environment
            variable such as <span class="font-mono">DATABASE_URL</span>.
          </p>
        {/if}
      </div>
    </SettingsSection>

    <SettingsSection
      id="public-access-section"
      title="Public access"
      helper="Publish this database on a port of the server, so clients outside it can connect. Switching access restarts the database; its data stays."
    >
      <div class="grid gap-4 lg:grid-cols-2">
        <Select
          label="Access"
          value={isPublic ? 'public' : 'private'}
          onchange={(e) => accessChanged(e.currentTarget.value)}
          disabled={!canUpdate || switching}
          data-testid="database-access"
        >
          <option value="private">Private</option>
          <option value="public" disabled={!isPublic && !portTyped}>
            {!isPublic && !portTyped ? 'Public on its port (set public port first)' : 'Public on its port'}
          </option>
        </Select>
        <Input
          label="Public port"
          type="number"
          bind:value={publicPort}
          error={errors.public_port}
          placeholder={String(defaultPort[database.type])}
          disabled={!canUpdate || isPublic}
          helper="A free port on the server, between 1024 and 65535."
        />
      </div>
    </SettingsSection>
  </div>
</form>
