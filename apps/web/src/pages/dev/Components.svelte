<script lang="ts">
  // Every component of the kit in each variant and size, for comparing with
  // Paperclip's ui/src/components/ui in both themes: first the shadcn
  // components in lib/components/ui, then lib/ui built on them. Dev builds only: the router answers
  // `notfound` for #/dev/components in production and App.svelte imports
  // this page behind the same check.
  import Button from '../../lib/ui/Button.svelte'
  import Callout from '../../lib/ui/Callout.svelte'
  import Checkbox from '../../lib/ui/Checkbox.svelte'
  import ConfirmationModal from '../../lib/ui/ConfirmationModal.svelte'
  import Dropdown from '../../lib/ui/Dropdown.svelte'
  import Empty from '../../lib/ui/Empty.svelte'
  import CollectionToolbar from '../../lib/CollectionToolbar.svelte'
  import EntityRow from '../../lib/EntityRow.svelte'
  import PageHeader from '../../lib/PageHeader.svelte'
  import ResourceNav, { type ResourceNavGroup } from '../../lib/ResourceNav.svelte'
  import SearchField from '../../lib/SearchField.svelte'
  import SortPopover from '../../lib/SortPopover.svelte'
  import ViewToggle from '../../lib/ViewToggle.svelte'
  import ProjectTile from '../../lib/ProjectTile.svelte'
  import MetricCard from '../../lib/MetricCard.svelte'
  import PageSkeleton from '../../lib/PageSkeleton.svelte'
  import Helper from '../../lib/ui/Helper.svelte'
  import Input from '../../lib/ui/Input.svelte'
  import Modal from '../../lib/ui/Modal.svelte'
  import SectionHeading from '../../lib/ui/SectionHeading.svelte'
  import Select from '../../lib/ui/Select.svelte'
  import StatusBadge, { containerStatus } from '../../lib/ui/StatusBadge.svelte'
  import StatusSummary from '../../lib/ui/StatusSummary.svelte'
  import { statusTypes } from '../../lib/statusColors'
  import Textarea from '../../lib/ui/Textarea.svelte'
  import { toast } from '../../lib/ui/toast.svelte'
  import UnsavedBar from '../../lib/ui/UnsavedBar.svelte'
  import { breadcrumb } from '../../lib/breadcrumb.svelte'
  import { theme, type Theme } from '../../lib/theme.svelte'
  import * as AlertDialog from '@bakery/ui/components/ui/alert-dialog'
  import * as Avatar from '@bakery/ui/components/ui/avatar'
  import { Badge } from '@bakery/ui/components/ui/badge'
  import * as Breadcrumb from '@bakery/ui/components/ui/breadcrumb'
  import { Button as UiButton, type ButtonSize, type ButtonVariant } from '@bakery/ui/components/ui/button'
  import * as Card from '@bakery/ui/components/ui/card'
  import { Checkbox as UiCheckbox } from '@bakery/ui/components/ui/checkbox'
  import * as Collapsible from '@bakery/ui/components/ui/collapsible'
  import * as Dialog from '@bakery/ui/components/ui/dialog'
  import * as DropdownMenu from '@bakery/ui/components/ui/dropdown-menu'
  import { Input as UiInput } from '@bakery/ui/components/ui/input'
  import { Label } from '@bakery/ui/components/ui/label'
  import * as Popover from '@bakery/ui/components/ui/popover'
  import { ScrollArea } from '@bakery/ui/components/ui/scroll-area'
  import * as UiSelect from '@bakery/ui/components/ui/select'
  import { Separator } from '@bakery/ui/components/ui/separator'
  import * as Sheet from '@bakery/ui/components/ui/sheet'
  import { Skeleton } from '@bakery/ui/components/ui/skeleton'
  import { Switch } from '@bakery/ui/components/ui/switch'
  import * as Tabs from '@bakery/ui/components/ui/tabs'
  import { Textarea as UiTextarea } from '@bakery/ui/components/ui/textarea'
  import * as Tooltip from '@bakery/ui/components/ui/tooltip'
  import { buttonVariants as uiButtonVariants } from '@bakery/ui/components/ui/button'
  import { ChevronDown, Plus } from '@lucide/svelte'

  // Triggers render their own element; they take the outline button's classes.
  const buttonClass = uiButtonVariants({ variant: 'outline' })

  $effect(() => breadcrumb.set({ label: 'Components' }))

  let name = $state('whoami')
  let navSection = $state('details')
  const navGroups: ResourceNavGroup[] = [
    {
      label: 'Settings',
      items: [
        {
          label: 'General',
          path: '/dev/components',
          icon: 'settings',
          active: true,
          sections: [
            { id: 'details', label: 'Application details' },
            { id: 'networking', label: 'Networking' },
          ],
        },
        { label: 'Domains', path: '/dev/components', icon: 'globe', active: false },
      ],
    },
    { label: 'Observe & troubleshoot', items: [{ label: 'Runtime Logs', path: '/dev/components', icon: 'unordered-list', active: false }] },
  ]
  let password = $state('s3cret-value')
  let dockerfile = $state('FROM nginx:alpine\nCOPY . /usr/share/nginx/html')
  let buildPack = $state('nixpacks')
  let autoDeploy = $state(true)
  let instantSaves = $state(0)
  let deleted = $state<string[] | null>(null)
  let loading = $state(false)
  let modalOpen = $state(false)
  let demoSearch = $state('')
  let demoSort = $state<'name-asc' | 'name-desc'>('name-asc')
  const demoSortOptions = [
    { value: 'name-asc' as const, label: 'Name A–Z' },
    { value: 'name-desc' as const, label: 'Name Z–A' },
  ]
  let demoView = $state<'table' | 'grid'>('table')
  let savedTitle = $state('My project')
  let draftTitle = $state('My project')
  let draftSaving = $state(false)

  const buttonVariants: ButtonVariant[] = ['default', 'cta', 'destructive', 'outline', 'secondary', 'ghost', 'link']
  const buttonSizes: ButtonSize[] = ['default', 'xs', 'sm', 'lg']
  const iconSizes: ButtonSize[] = ['icon', 'icon-xs', 'icon-sm', 'icon-lg']
  const badgeVariants = ['default', 'secondary', 'destructive', 'outline', 'ghost', 'link'] as const
  const themes: Theme[] = ['dark', 'light', 'system']
  let uiChecked = $state(true)
  let switchOn = $state(true)
  let selectValue = $state('nixpacks')

  function saveDraft() {
    draftSaving = true
    setTimeout(() => {
      savedTitle = draftTitle
      draftSaving = false
      toast.success('Project updated.')
    }, 500)
  }

  function fakeSave() {
    loading = true
    setTimeout(() => {
      loading = false
      toast.success('Saved', 'The configuration was saved.')
    }, 800)
  }
</script>

<div class="chrome mx-auto flex max-w-4xl flex-col gap-10 pb-20" data-testid="components-page">
  <div>
    <h1 class="text-2xl font-semibold">Components</h1>
    <p class="mt-1 text-sm text-muted-foreground">Every component of the kit, in each of its variants and sizes.</p>
  </div>

  <section id="theme" class="flex items-center gap-2" data-testid="theme-switch">
    <span class="text-sm text-muted-foreground">Theme</span>
    {#each themes as value (value)}
      <UiButton size="sm" variant={theme.current === value ? 'default' : 'outline'} onclick={() => theme.set(value)}>{value}</UiButton>
    {/each}
  </section>

  <section id="ui-button" class="flex flex-col gap-3">
    <SectionHeading title="ui/button" subtitle="variants, sizes and icon sizes" />
    <div class="flex flex-wrap items-center gap-2">
      {#each buttonVariants as variant (variant)}
        <UiButton {variant}>{variant}</UiButton>
      {/each}
      <UiButton disabled>disabled</UiButton>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      {#each buttonSizes as size (size)}
        <UiButton variant="outline" {size}><Plus />{size}</UiButton>
      {/each}
      {#each iconSizes as size (size)}
        <UiButton variant="outline" {size} aria-label={size}><Plus /></UiButton>
      {/each}
    </div>
  </section>

  <section id="ui-badge" class="flex flex-col gap-3">
    <SectionHeading title="ui/badge, ui/avatar, ui/skeleton, ui/separator" />
    <div class="flex flex-wrap items-center gap-2">
      {#each badgeVariants as variant (variant)}
        <Badge {variant}>{variant}</Badge>
      {/each}
    </div>
    <div class="flex items-center gap-3">
      {#each ['xs', 'sm', 'default', 'lg'] as const as size (size)}
        <Avatar.Root {size}><Avatar.Fallback>TB</Avatar.Fallback></Avatar.Root>
      {/each}
      <Separator orientation="vertical" class="h-6" />
      <Skeleton class="h-4 w-32" />
      <Skeleton class="size-8 rounded-full" />
    </div>
    <Separator />
  </section>

  <section id="ui-form" class="flex flex-col gap-3">
    <SectionHeading title="ui/input, ui/textarea, ui/select, ui/checkbox, ui/switch, ui/label" />
    <div class="grid gap-4 md:grid-cols-2">
      <div class="flex flex-col gap-2">
        <Label for="ui-input">Input</Label>
        <UiInput id="ui-input" placeholder="whoami" />
      </div>
      <div class="flex flex-col gap-2">
        <Label for="ui-input-invalid">Invalid</Label>
        <UiInput id="ui-input-invalid" aria-invalid="true" value="bad value" />
      </div>
      <div class="flex flex-col gap-2">
        <Label>Select</Label>
        <UiSelect.Root type="single" bind:value={selectValue}>
          <UiSelect.Trigger class="w-full">{selectValue}</UiSelect.Trigger>
          <UiSelect.Content>
            <UiSelect.Group>
              <UiSelect.Label>Build Pack</UiSelect.Label>
              {#each ['nixpacks', 'dockerfile', 'static'] as value (value)}
                <UiSelect.Item {value} label={value} />
              {/each}
            </UiSelect.Group>
          </UiSelect.Content>
        </UiSelect.Root>
      </div>
      <div class="flex flex-col gap-2">
        <Label for="ui-disabled">Disabled</Label>
        <UiInput id="ui-disabled" disabled value="disabled" />
      </div>
    </div>
    <UiTextarea placeholder="Textarea" />
    <div class="flex flex-wrap items-center gap-6">
      <div class="flex items-center gap-2"><UiCheckbox id="ui-check" bind:checked={uiChecked} /><Label for="ui-check">Checkbox</Label></div>
      <div class="flex items-center gap-2"><UiCheckbox id="ui-check-off" /><Label for="ui-check-off">Unchecked</Label></div>
      <div class="flex items-center gap-2"><Switch bind:checked={switchOn} /><span class="text-sm">Switch</span></div>
      <div class="flex items-center gap-2"><Switch size="lg" checked={false} /><span class="text-sm">Switch lg</span></div>
    </div>
  </section>

  <section id="ui-card" class="flex flex-col gap-3">
    <SectionHeading title="ui/card, ui/tabs, ui/breadcrumb" />
    <Card.Root>
      <Card.Header>
        <Card.Title>Card title</Card.Title>
        <Card.Description>A card's description.</Card.Description>
        <Card.Action><UiButton size="sm" variant="outline">Action</UiButton></Card.Action>
      </Card.Header>
      <Card.Content><p class="text-sm">Card content.</p></Card.Content>
      <Card.Footer><UiButton size="sm">Save</UiButton></Card.Footer>
    </Card.Root>
    <Tabs.Root value="general">
      <Tabs.List>
        <Tabs.Trigger value="general">General</Tabs.Trigger>
        <Tabs.Trigger value="members">Members</Tabs.Trigger>
        <Tabs.Trigger value="roles">Roles</Tabs.Trigger>
      </Tabs.List>
      <Tabs.Content value="general" class="text-sm">General settings.</Tabs.Content>
      <Tabs.Content value="members" class="text-sm">Members.</Tabs.Content>
      <Tabs.Content value="roles" class="text-sm">Roles.</Tabs.Content>
    </Tabs.Root>
    <Tabs.Root value="one">
      <Tabs.List variant="line">
        <Tabs.Trigger value="one">Line</Tabs.Trigger>
        <Tabs.Trigger value="two">Tabs</Tabs.Trigger>
      </Tabs.List>
    </Tabs.Root>
    <Breadcrumb.Root>
      <Breadcrumb.List>
        <Breadcrumb.Item><Breadcrumb.Link href="#/">Projects</Breadcrumb.Link></Breadcrumb.Item>
        <Breadcrumb.Separator />
        <Breadcrumb.Item><Breadcrumb.Ellipsis /></Breadcrumb.Item>
        <Breadcrumb.Separator />
        <Breadcrumb.Item><Breadcrumb.Page>whoami</Breadcrumb.Page></Breadcrumb.Item>
      </Breadcrumb.List>
    </Breadcrumb.Root>
  </section>

  <section id="ui-overlays" class="flex flex-col gap-3">
    <SectionHeading title="ui/dialog, ui/alert-dialog, ui/sheet, ui/dropdown-menu, ui/popover, ui/tooltip" />
    <div class="flex flex-wrap items-center gap-2">
      <Dialog.Root>
        <Dialog.Trigger class={buttonClass}>Dialog</Dialog.Trigger>
        <Dialog.Content>
          <Dialog.Header>
            <Dialog.Title>New Environment</Dialog.Title>
            <Dialog.Description>Environments group resources.</Dialog.Description>
          </Dialog.Header>
          <UiInput placeholder="staging" />
          <Dialog.Footer><UiButton>Continue</UiButton></Dialog.Footer>
        </Dialog.Content>
      </Dialog.Root>
      <AlertDialog.Root>
        <AlertDialog.Trigger class={buttonClass}>Alert dialog</AlertDialog.Trigger>
        <AlertDialog.Content>
          <AlertDialog.Header>
            <AlertDialog.Title>Delete the guild?</AlertDialog.Title>
            <AlertDialog.Description>This cannot be undone.</AlertDialog.Description>
          </AlertDialog.Header>
          <AlertDialog.Footer>
            <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
            <AlertDialog.Action>Delete</AlertDialog.Action>
          </AlertDialog.Footer>
        </AlertDialog.Content>
      </AlertDialog.Root>
      <Sheet.Root>
        <Sheet.Trigger class={buttonClass}>Sheet</Sheet.Trigger>
        <Sheet.Content side="left">
          <Sheet.Header>
            <Sheet.Title>Navigation</Sheet.Title>
            <Sheet.Description>The mobile drawer.</Sheet.Description>
          </Sheet.Header>
        </Sheet.Content>
      </Sheet.Root>
      <DropdownMenu.Root>
        <DropdownMenu.Trigger class={buttonClass}>Menu <ChevronDown /></DropdownMenu.Trigger>
        <DropdownMenu.Content>
          <DropdownMenu.Group>
            <DropdownMenu.Label>Guild</DropdownMenu.Label>
            <DropdownMenu.Item>Settings</DropdownMenu.Item>
            <DropdownMenu.Item>Invite</DropdownMenu.Item>
          </DropdownMenu.Group>
          <DropdownMenu.Separator />
          <DropdownMenu.Item variant="destructive">Log out</DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Root>
      <Popover.Root>
        <Popover.Trigger class={buttonClass}>Popover</Popover.Trigger>
        <Popover.Content><p class="text-sm">Popover content.</p></Popover.Content>
      </Popover.Root>
      <Tooltip.Provider>
        <Tooltip.Root>
          <Tooltip.Trigger class={buttonClass}>Tooltip</Tooltip.Trigger>
          <Tooltip.Content>Tooltip text</Tooltip.Content>
        </Tooltip.Root>
      </Tooltip.Provider>
    </div>
    <Collapsible.Root class="flex flex-col gap-2">
      <Collapsible.Trigger class={buttonClass}>Collapsible</Collapsible.Trigger>
      <Collapsible.Content class="text-sm text-muted-foreground">Hidden until opened.</Collapsible.Content>
    </Collapsible.Root>
    <ScrollArea class="h-24 rounded-md border">
      <div class="flex flex-col gap-1 p-3 text-sm">
        {#each Array.from({ length: 12 }, (_, i) => i + 1) as n (n)}
          <span>Scroll area row {n}</span>
        {/each}
      </div>
    </ScrollArea>
  </section>

  <Separator />
  <h2 class="text-lg font-semibold">lib/ui</h2>

  <section id="buttons">
    <SectionHeading title="Button" subtitle="forms/button: default, highlighted, error, disabled and loading" />
    <div class="flex flex-wrap items-center gap-2">
      <Button>Default</Button>
      <Button variant="highlighted">Highlighted</Button>
      <Button variant="error">Error</Button>
      <Button disabled>Disabled</Button>
      <Button variant="highlighted" {loading} onclick={fakeSave}>Save</Button>
    </div>
  </section>

  <section id="inputs">
    <SectionHeading title="Input, Textarea, Select" subtitle="forms/input, forms/textarea, forms/select" />
    <div class="grid gap-4 md:grid-cols-2">
      <Input label="Name" required bind:value={name} helper="The name of the resource, shown everywhere." />
      <Input label="Password" type="password" bind:value={password} />
      <Input label="Read-only" value="cannot change this" readonly />
      <Input label="With error" value="bad value" error="The value is not valid." />
      <Select label="Build Pack" bind:value={buildPack} helper="How the image is built.">
        <option value="nixpacks">Nixpacks</option>
        <option value="dockerfile">Dockerfile</option>
        <option value="static">Static</option>
      </Select>
      <Input label="Disabled" value="disabled" disabled />
    </div>
    <div class="mt-4">
      <Textarea label="Dockerfile" monospace allowTab rows={4} bind:value={dockerfile} />
    </div>
  </section>

  <section id="checkboxes">
    <SectionHeading title="Checkbox" subtitle="forms/checkbox, with an instant-save callback" />
    <div class="flex max-w-md flex-col">
      <Checkbox
        label="Auto Deploy"
        helper="Deploy on every push to the branch."
        bind:checked={autoDeploy}
        instantSave={(v) => {
          instantSaves++
          toast.success(v ? 'Auto Deploy enabled' : 'Auto Deploy disabled')
        }}
      />
      <Checkbox label="Disabled" disabled />
      <p class="px-2.5 text-xs text-muted-foreground" data-testid="instant-saves">Instant saves: {instantSaves}</p>
    </div>
  </section>

  <section id="helper">
    <SectionHeading title="Helper" subtitle="helper: hover or tap the icon" />
    <div class="flex items-center gap-2 text-sm">Health check <Helper helper="The path The Bakery calls to see if the container is ready." /></div>
  </section>

  <section id="badges">
    <SectionHeading title="StatusBadge" subtitle="status-badge and status/*" />
    <div class="flex flex-wrap gap-2">
      <StatusBadge {...containerStatus('running:healthy')} />
      <StatusBadge {...containerStatus('running:unknown')} />
      <StatusBadge {...containerStatus('restarting')} />
      <StatusBadge {...containerStatus('degraded:unhealthy')} />
      <StatusBadge {...containerStatus('exited')} />
      <StatusBadge status="Refresh" onclick={() => toast.info('Refreshing status')} class="cursor-pointer border-transparent hover:bg-accent" />
    </div>
    <p class="mt-4 mb-2 text-xs text-muted-foreground">Every state in the status palette, once</p>
    <div class="flex flex-wrap gap-2" data-testid="status-palette">
      {#each Object.keys(statusTypes) as state (state)}
        <StatusBadge status={state} />
      {/each}
    </div>
    <p class="mt-4 mb-2 text-xs text-muted-foreground">status-summary: click to see the container and its healthcheck</p>
    <div class="flex flex-wrap gap-2">
      <StatusSummary status="running:healthy" />
      <StatusSummary status="running:unknown" />
      <StatusSummary status="degraded:unhealthy" />
      <StatusSummary status="exited" />
    </div>
  </section>

  <section id="callouts">
    <SectionHeading title="Callout" subtitle="callout: info, warning, danger, success" />
    <div class="flex flex-col gap-3">
      <Callout type="info" title="Info">Containers on this server are managed by The Bakery.</Callout>
      <Callout type="warning" title="Warning">The disk is almost full.</Callout>
      <Callout type="danger" title="Danger" ondismiss={() => toast.info('Dismissed')}>The deployment failed.</Callout>
      <Callout type="success" title="Success">The backup finished.</Callout>
    </div>
  </section>

  <section id="empty">
    <SectionHeading title="Empty" subtitle="empty" />
    <Empty title="No resources found" description="Add an application, a database or a service to this environment." icon="layers">
      <Button variant="highlighted">+ Add Resource</Button>
    </Empty>
  </section>

  <section id="collection-toolbar">
    <SectionHeading title="CollectionToolbar" subtitle="CollectionToolbar with SearchField, SortPopover and ViewToggle: context, search, controls, actions, feedback" />
    <CollectionToolbar>
      {#snippet context()}<span class="text-sm text-muted-foreground">3 projects</span>{/snippet}
      {#snippet search()}<SearchField bind:value={demoSearch} label="Search projects" />{/snippet}
      {#snippet controls()}
        <SortPopover bind:value={demoSort} options={demoSortOptions} label="Sort projects" />
        <ViewToggle value={demoView} onchange={(v) => (demoView = v)} />
      {/snippet}
      {#snippet actions()}<UiButton variant="outline" size="sm"><Plus class="size-4" />New project</UiButton>{/snippet}
      {#snippet feedback()}<Badge variant="secondary">Filtered by name</Badge>{/snippet}
    </CollectionToolbar>
  </section>

  <section id="page-header">
    <SectionHeading title="PageHeader" subtitle="PageHeader: leading tile, title, description, meta, actions" />
    <PageHeader title="Bakery" description="The deployment platform">
      {#snippet leading()}<ProjectTile size="lg" />{/snippet}
      {#snippet meta()}<span>2 environments in this project</span>{/snippet}
      {#snippet actions()}
        <UiButton variant="outline" size="sm">Settings</UiButton>
        <UiButton size="sm"><Plus class="size-4" />New environment</UiButton>
      {/snippet}
    </PageHeader>
  </section>

  <section id="resource-nav">
    <SectionHeading title="ResourceNav" subtitle="Grouped sub-pages with in-page sections; a select below xl" />
    <div class="max-w-[210px]">
      <ResourceNav groups={navGroups} label="Example sections" activeSection={navSection} onsection={(_, id) => (navSection = id)} />
    </div>
  </section>

  <section id="entity-row">
    <SectionHeading title="EntityRow, ProjectTile" subtitle="EntityRow in a card; ProjectTile xs, sm, md, lg, neutral and tinted" />
    <Card.Root class="block gap-0 overflow-hidden py-0">
      <EntityRow title="Bakery" subtitle="The deployment platform" href="#/dev/components">
        {#snippet leading()}<ProjectTile size="sm" />{/snippet}
        {#snippet trailing()}<span class="text-xs text-muted-foreground">2 envs · 5 resources</span>{/snippet}
      </EntityRow>
      <EntityRow title="Selected, no subtitle" reserveSubtitleSpace selected>
        {#snippet leading()}<ProjectTile size="sm" color="#6366f1" />{/snippet}
      </EntityRow>
    </Card.Root>
    <div class="mt-4 flex items-center gap-3">
      {#each ['xs', 'sm', 'md', 'lg'] as const as size (size)}
        <ProjectTile {size} />
        <ProjectTile {size} color="#0ea5e9" icon="servers" />
      {/each}
    </div>
  </section>

  <section id="metric-card">
    <SectionHeading title="Metric card" subtitle="MetricCard" />
    <div class="grid grid-cols-2 gap-2 xl:grid-cols-4">
      <MetricCard icon="projects" value={3} label="Projects" href="#/projects">
        {#snippet description()}Deployment workspaces{/snippet}
      </MetricCard>
      <MetricCard icon="alert-triangle" value={1} label="Servers needing attention" />
    </div>
  </section>

  <section id="page-skeleton">
    <SectionHeading title="Page skeleton" subtitle="PageSkeleton list, dashboard" />
    <div class="grid gap-6 md:grid-cols-2">
      <PageSkeleton />
      <PageSkeleton variant="dashboard" />
    </div>
  </section>

  <section id="dropdown">
    <SectionHeading title="Dropdown" subtitle="dropdown">
      {#snippet actions()}
        <Dropdown title="Actions" triggerClass="button">
          {#snippet children(close)}
            <button type="button" class="dropdown-item" onclick={() => (close(), toast.info('Restarting'))}>Restart</button>
            <button type="button" class="dropdown-item" onclick={() => (close(), toast.warning('Stopping'))}>Stop</button>
          {/snippet}
        </Dropdown>
      {/snippet}
    </SectionHeading>
  </section>

  <section id="toasts">
    <SectionHeading title="Toast" subtitle="toast: leaves after 4 s, hovering holds it" />
    <div class="flex flex-wrap gap-2">
      <Button onclick={() => toast.success('Deployment started')}>Success</Button>
      <Button onclick={() => toast.info('Status refreshed', 'All containers are running.')}>Info</Button>
      <Button onclick={() => toast.warning('High disk usage')}>Warning</Button>
      <Button onclick={() => toast.error('Deployment failed', 'exit status 1')}>Error</Button>
    </div>
  </section>

  <section id="unsaved-bar">
    <SectionHeading title="UnsavedBar" subtitle="unsaved-bar: shows while the field differs from what was saved; Enter saves" />
    <Input label="Name" bind:value={draftTitle} />
    <UnsavedBar dirty={draftTitle !== savedTitle} saving={draftSaving} onsave={saveDraft} onreset={() => (draftTitle = savedTitle)} />
  </section>

  <section id="modals">
    <SectionHeading title="Modal, ConfirmationModal" subtitle="modal-input, modal-confirmation" />
    <div class="flex flex-wrap gap-2">
      <Modal title="New Environment" subtitle="Environments group resources." buttonTitle="+ Add" variant="highlighted" bind:open={modalOpen}>
        <form
          class="flex flex-col gap-4"
          onsubmit={(e) => {
            e.preventDefault()
            modalOpen = false
            toast.success('Environment created')
          }}
        >
          <Input label="Name" required placeholder="staging" />
          <Button type="submit" variant="highlighted">Continue</Button>
        </form>
      </Modal>
      <ConfirmationModal
        title="Confirm Application Deletion?"
        buttonTitle="Delete"
        variant="error"
        checkboxes={[
          { id: 'delete_volumes', label: 'Permanently delete all volumes associated with this resource.', checked: true },
          { id: 'delete_configurations', label: 'Permanently delete all configuration files from the server.', checked: true },
        ]}
        actions={['This application will be deleted.', 'All deployments and their logs will be deleted.']}
        confirmationText="whoami"
        confirmationLabel="Please confirm the execution of the actions by entering the Application Name below"
        shortConfirmationLabel="Application Name"
        onconfirm={(selected) => {
          deleted = selected
          toast.success('Deleted', `whoami (${selected.join(', ') || 'nothing else'})`)
        }}
      />
    </div>
    {#if deleted}
      <p class="mt-2 text-xs text-muted-foreground" data-testid="deleted">Confirmed with: {deleted.join(', ') || 'none'}</p>
    {/if}
  </section>
</div>
