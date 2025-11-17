<script>
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import {
		AlertDialog,
		AlertDialogAction,
		AlertDialogCancel,
		AlertDialogContent,
		AlertDialogDescription,
		AlertDialogFooter,
		AlertDialogHeader,
		AlertDialogTitle,
		AlertDialogTrigger
	} from '$lib/components/ui/alert-dialog/index.js';
	import { createNodeRecord, fetchNodes, deleteNode, verifyNode, renameNode } from '$lib/api.js';
	import { Check, Copy, RefreshCw, Trash2, ShieldCheck, CloudUpload, Send } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	const DEFAULT_SSH_USER = 'bakery-agent';

	let { data } = $props();

	let nodes = $state(data.nodes ?? []);
	let controlPlane = $state(data.controlPlane ?? null);
	let newNodeName = $state('');
	let creating = $state(false);
	let refreshing = $state(false);
	let globalMessage = $state('');
	let globalError = $state('');
	let copyState = $state({});
	let latestInstaller = $state(null);
	let deleting = $state({});
	let deleteDialogId = $state(null);
	let verifyInputs = $state({});
	let verifying = $state({});
	let renaming = $state({});
	let renameInputs = $state({});
	let renameEditors = $state({});

	const workflowPhases = [
		{
			key: 'channel',
			title: 'Phase 1 · SSH channel',
			description: 'Bootstrap nodes, capture host metadata, and confirm SSH reachability.',
			icon: ShieldCheck,
			points: [
				'Create a node to mint SSH keys and generate a bootstrap command.',
				'Run install-node-agent on the server to add the bakery-agent user.',
				'Store host, port, and username; validate with a push ping.'
			]
		},
		{
			key: 'webhook',
			title: 'Phase 2 · Push CI/CD',
			description: 'GitHub push events fan out over SSH to execute builds instantly.',
			icon: CloudUpload,
			points: [
				'GitHub webhook hits /api/webhooks/github.',
				'Control plane opens SSH and triggers the agent runtime.',
				'Logs stream back into Bakery for every delivery.'
			]
		},
		{
			key: 'manual',
			title: 'Phase 3 · Manual control',
			description: 'One-click updates or rescues when you need direct intervention.',
			icon: Send,
			points: [
				'Trigger SSH pushes from the UI when automation is paused.',
				'Watch streaming logs for each run in the activity feed.',
				'Fallback to the control plane if a remote node becomes unreachable.'
			]
		}
	];

	function nodeStatusLabel(status) {
		switch (status) {
			case 'active':
				return 'Active';
			case 'pending':
				return 'Awaiting verification';
			default:
				return status ?? 'unknown';
		}
	}

	function nodeStatusColor(status) {
		switch (status) {
			case 'active':
				return 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200';
			case 'pending':
				return 'bg-amber-50 text-amber-700 ring-1 ring-amber-200';
			default:
				return 'bg-slate-100 text-slate-600 ring-1 ring-slate-200';
		}
	}

	let activeCount = $derived(nodes.filter((node) => node.status === 'active').length);
	let pendingCount = $derived(nodes.filter((node) => node.status !== 'active').length);
	let lastSeenTimestamp = $derived.by(() => {
		let latest = null;
		for (const node of nodes ?? []) {
			if (!node.last_seen) continue;
			const ms = new Date(node.last_seen).getTime();
			if (Number.isNaN(ms)) continue;
			if (latest == null || ms > latest) {
				latest = ms;
			}
		}
		return latest;
	});
	let lastHandshakeText = $derived(relativeTimeFromNow(lastSeenTimestamp));
	let latestInstallerKey = $derived(
		latestInstaller ? buildCopyKey(latestInstaller.nodeId, 'install') : null
	);
	let computedNodes = $derived(
		nodes.map((node) => ({
			node,
			verifyInput: resolveVerifyInput(node),
			installKey: buildCopyKey(node.id, 'install'),
			updateKey: buildCopyKey(node.id, 'update'),
			pushKey: buildCopyKey(node.id, 'push'),
			handshakeCommand: buildHandshakeCommand(node),
			lastSeenText: relativeTimestamp(node.last_seen),
			lastHandshakeText: relativeTimestamp(node.ssh_last_connected),
			needsPairing: node.status !== 'active' || !node.ssh_host
		}))
	);

	function relativeTimeFromNow(timestamp) {
		if (!timestamp) return 'never';
		const diffMs = Date.now() - timestamp;
		if (!Number.isFinite(diffMs) || diffMs < 0) {
			return 'just now';
		}
		const minutes = Math.floor(diffMs / 60000);
		if (minutes < 1) return 'just now';
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		return `${days}d ago`;
	}

	function formatDateTime(value) {
		if (!value) return '—';
		try {
			return new Date(value).toLocaleString();
		} catch {
			return String(value);
		}
	}

	function buildCopyKey(id, scope) {
		return `${id}-${scope}`;
	}

	function resolveVerifyInput(node) {
		return (
			verifyInputs[node.id] ?? {
				host: node.ssh_host ?? '',
				port: node.ssh_port ?? 22,
				user: node.ssh_user ?? DEFAULT_SSH_USER
			}
		);
	}

	function buildHandshakeCommand(node) {
		const snapshot = resolveVerifyInput(node);
		const host = node.ssh_host || snapshot.host;
		if (!host) return null;
		const port = node.ssh_port || snapshot.port || 22;
		const user = node.ssh_user || snapshot.user || DEFAULT_SSH_USER;
		return `ssh -p ${port} ${user}@${host} 'echo bakery-ready'`;
	}

	function relativeTimestamp(value) {
		if (!value) return 'never';
		const timestamp = new Date(value).getTime();
		if (Number.isNaN(timestamp)) return 'never';
		return relativeTimeFromNow(timestamp);
	}

	function dropKey(collection, key) {
		const { [key]: _removed, ...rest } = collection;
		return rest;
	}

	function openRenameEditor(nodeId) {
		const currentName = nodes.find((node) => node.id === nodeId)?.name ?? '';
		renameInputs = {
			...renameInputs,
			[nodeId]: renameInputs[nodeId] ?? currentName
		};
		renameEditors = { ...renameEditors, [nodeId]: true };
	}

	function closeRenameEditor(nodeId) {
		renameEditors = dropKey(renameEditors, nodeId);
		renameInputs = dropKey(renameInputs, nodeId);
	}

	async function refreshNodes() {
		refreshing = true;
		try {
			const payload = await fetchNodes();
			nodes = payload.nodes || [];
			controlPlane = payload.controlPlane ?? controlPlane;
			if (!nodes.some((node) => node.id === deleteDialogId)) {
				deleteDialogId = null;
			}
		} catch (error) {
			const message = error?.message || 'Failed to refresh nodes';
			globalError = message;
			toast.error(message);
		} finally {
			refreshing = false;
		}
	}

	async function handleCreate(event) {
		event.preventDefault();
		globalError = '';
		globalMessage = '';
		if (!newNodeName.trim()) {
			globalError = 'Provide a name for the server node.';
			toast.warning(globalError);
			return;
		}
		creating = true;
		try {
			const payload = await createNodeRecord({ name: newNodeName.trim() });
			nodes = [payload.node, ...nodes.filter((node) => node.id !== payload.node.id)];
			latestInstaller =
				payload.node?.install_command != null
					? { nodeId: payload.node.id, command: payload.node.install_command }
					: null;
			globalMessage =
				'Node created. Run the bootstrap command, then verify SSH access to activate pushes.';
			newNodeName = '';
			toast.success(globalMessage);
		} catch (error) {
			const message = error?.message || 'Failed to create node';
			globalError = message;
			toast.error(message);
		} finally {
			creating = false;
		}
	}

	async function handleCopy(command, key) {
		if (!command) return;
		try {
			await navigator.clipboard.writeText(command);
			copyState = { ...copyState, [key]: true };
			setTimeout(() => {
				copyState = { ...copyState, [key]: false };
			}, 2000);
		} catch {
			const message = 'Unable to copy command to clipboard';
			globalError = message;
			toast.error(message);
		}
	}

	function updateVerifyInput(nodeId, field, value) {
		verifyInputs = {
			...verifyInputs,
			[nodeId]: {
				...(verifyInputs[nodeId] ?? { host: '', port: 22, user: DEFAULT_SSH_USER }),
				[field]: value
			}
		};
	}

	async function handleVerify(nodeId) {
		globalError = '';
		globalMessage = '';
		const targetNode = nodes.find((entry) => entry.id === nodeId) ?? null;
		const snapshot = targetNode
			? resolveVerifyInput(targetNode)
			: { host: '', port: 22, user: DEFAULT_SSH_USER };
		const input = { ...snapshot };
		if (!input.host.trim()) {
			globalError = 'Provide the server host or IP address before verifying.';
			toast.warning(globalError);
			return;
		}
		verifying = { ...verifying, [nodeId]: true };
		try {
			const payload = await verifyNode(nodeId, {
				host: input.host.trim(),
				port: Number(input.port) || 22,
				user: input.user?.trim() || undefined
			});
			nodes = nodes.map((node) => (node.id === nodeId ? payload.node : node));
			globalMessage = 'SSH push channel verified.';
			toast.success(globalMessage);
		} catch (error) {
			const message = error?.message || 'Failed to verify node.';
			globalError = message;
			toast.error(message);
		} finally {
			verifying = { ...verifying, [nodeId]: false };
		}
	}

	async function deleteNodeAction(nodeId) {
		globalError = '';
		globalMessage = '';
		deleting = { ...deleting, [nodeId]: true };
		try {
			await deleteNode(nodeId);
			nodes = nodes.filter((node) => node.id !== nodeId);
			globalMessage = 'Node removed.';
			toast.success(globalMessage);
			if (deleteDialogId === nodeId) {
				deleteDialogId = null;
			}
		} catch (error) {
			const message = error?.message || 'Failed to delete node';
			globalError = message;
			toast.error(message);
		} finally {
			deleting = { ...deleting, [nodeId]: false };
		}
	}

	async function handleRename(nodeId) {
		globalError = '';
		globalMessage = '';
		const nextName = renameInputs[nodeId]?.trim();
		if (!nextName) {
			globalError = 'Provide a new name.';
			toast.warning(globalError);
			return;
		}
		renaming = { ...renaming, [nodeId]: true };
		try {
			const payload = await renameNode(nodeId, nextName);
			nodes = nodes.map((node) => (node.id === nodeId ? payload.node : node));
			globalMessage = 'Node renamed.';
			toast.success(globalMessage);
			closeRenameEditor(nodeId);
		} catch (error) {
			const message = error?.message || 'Failed to rename node.';
			globalError = message;
			toast.error(message);
		} finally {
			renaming = { ...renaming, [nodeId]: false };
		}
	}
</script>

<svelte:head>
	<title>SSH push workflow ~ The Bakery</title>
</svelte:head>

<section class="space-y-8 p-6 md:p-10">
	<div class="rounded-3xl border bg-card p-6 shadow-sm md:p-10">
		<div class="flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
			<div class="space-y-3">
				<p class="text-xs tracking-[0.3em] text-muted-foreground uppercase">Deployment workflow</p>
				<h1 class="text-3xl font-semibold tracking-tight">SSH push-based nodes</h1>
				<p class="text-sm text-muted-foreground">
					Create agents, bootstrap servers over SSH, and trigger push deployments without waiting
					for polling intervals.
				</p>
			</div>
			<Button class="self-start" variant="secondary" onclick={refreshNodes} disabled={refreshing}>
				{#if refreshing}
					<RefreshCw class="mr-2 h-4 w-4 animate-spin" />
				{/if}
				Sync nodes
			</Button>
		</div>
		<div class="mt-6 grid gap-4 text-sm md:grid-cols-3">
			<div class="rounded-2xl border bg-background/80 p-4">
				<p class="text-xs font-medium text-muted-foreground">Active nodes</p>
				<p class="mt-1 text-2xl font-semibold">{activeCount}</p>
			</div>
			<div class="rounded-2xl border bg-background/80 p-4">
				<p class="text-xs font-medium text-muted-foreground">Handshakes pending</p>
				<p class="mt-1 text-2xl font-semibold">{pendingCount}</p>
			</div>
			<div class="rounded-2xl border bg-background/80 p-4">
				<p class="text-xs font-medium text-muted-foreground">Last SSH push</p>
				<p class="mt-1 text-2xl font-semibold">{lastHandshakeText}</p>
			</div>
		</div>
	</div>

	{#if globalMessage}
		<div
			class="flex items-start gap-2 rounded-xl border border-emerald-400/40 bg-emerald-500/10 p-3 text-sm text-emerald-500"
		>
			<Check class="mt-0.5 h-4 w-4" />
			<p>{globalMessage}</p>
		</div>
	{/if}

	{#if globalError}
		<div
			class="rounded-xl border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive"
		>
			<p>{globalError}</p>
		</div>
	{/if}

	<section class="rounded-2xl border bg-card p-6 shadow-sm">
		<div class="flex flex-col gap-2">
			<h2 class="text-lg font-semibold">Push workflow map</h2>
			<p class="text-sm text-muted-foreground">
				Each phase builds on the last so your deployments can be pushed over SSH immediately after a
				commit hits GitHub.
			</p>
		</div>
		<div class="mt-5 grid gap-4 md:grid-cols-3">
			{#each workflowPhases as phase (phase.key)}
				<div class="flex flex-col gap-3 rounded-2xl border bg-background/80 p-4">
					<div class="flex items-center gap-3">
						<div class="rounded-full bg-primary/10 p-2 text-primary">
							<phase.icon class="h-5 w-5" />
						</div>
						<div>
							<p class="font-semibold">{phase.title}</p>
							<p class="text-xs text-muted-foreground">{phase.description}</p>
						</div>
					</div>
					<ul class="space-y-1 text-xs text-muted-foreground">
						{#each phase.points as point}
							<li class="flex gap-2">
								<span class="mt-0.5 h-1.5 w-1.5 rounded-full bg-primary/60"></span>
								<span>{point}</span>
							</li>
						{/each}
					</ul>
				</div>
			{/each}
		</div>
	</section>

	<div class="grid gap-6 lg:grid-cols-[1.1fr,0.9fr]">
		<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
			<div>
				<h2 class="text-lg font-semibold">Bootstrap a new node</h2>
				<p class="text-sm text-muted-foreground">
					Mint SSH keys, link the server name, and copy the one-line installer that configures the
					bakery-agent user.
				</p>
			</div>
			<form class="space-y-3" onsubmit={handleCreate}>
				<label class="flex flex-col gap-1 text-sm font-medium">
					<span>Node name</span>
					<Input placeholder="hetzner-fsn1-primary" bind:value={newNodeName} />
				</label>
				<Button class="w-full" type="submit" disabled={creating}>
					{#if creating}
						<RefreshCw class="mr-2 h-4 w-4 animate-spin" />
					{/if}
					Link node
				</Button>
			</form>
			<div class="rounded-2xl border border-dashed bg-background/60 p-4">
				<p class="text-[11px] font-semibold tracking-[0.2em] text-muted-foreground uppercase">
					Setup checklist
				</p>
				<div class="mt-3 grid gap-3 md:grid-cols-3">
					<div class="flex items-center gap-3 rounded-xl border bg-card/70 p-3">
						<span class="rounded-full bg-primary/10 p-2 text-primary">
							<ShieldCheck class="h-4 w-4" />
						</span>
						<div class="space-y-1 text-xs text-muted-foreground">
							<p class="font-semibold text-foreground">Run installer</p>
							<p>
								Executes install-node-agent.sh, creates the bakery-agent user, and uploads the SSH
								key.
							</p>
						</div>
					</div>
					<div class="flex items-center gap-3 rounded-xl border bg-card/70 p-3">
						<span class="rounded-full bg-primary/10 p-2 text-primary">
							<CloudUpload class="h-4 w-4" />
						</span>
						<div class="space-y-1 text-xs text-muted-foreground">
							<p class="font-semibold text-foreground">Store SSH metadata</p>
							<p>Record host, port, and username so pushes know where to dial.</p>
						</div>
					</div>
					<div class="flex items-center gap-3 rounded-xl border bg-card/70 p-3 md:col-span-1">
						<span class="rounded-full bg-primary/10 p-2 text-primary">
							<Send class="h-4 w-4" />
						</span>
						<div class="space-y-1 text-xs text-muted-foreground">
							<p class="font-semibold text-foreground">Verify push channel</p>
							<p>Bakery runs the same SSH echo it issues before streaming deployments.</p>
						</div>
					</div>
				</div>
			</div>
		</section>
		<section class="space-y-4">
			{#if latestInstaller?.command}
				<div class="space-y-3 rounded-2xl border bg-card p-6 shadow-sm">
					<p class="text-sm font-semibold">Latest bootstrap command</p>
					<Textarea
						readonly
						class="h-32 w-full resize-none text-xs"
						value={latestInstaller.command}
					/>
					<Button
						variant="outline"
						class="w-full gap-2"
						onclick={() => handleCopy(latestInstaller.command, latestInstallerKey)}
					>
						{#if latestInstallerKey && copyState[latestInstallerKey]}
							<Check class="h-4 w-4" /> Copied
						{:else}
							<Copy class="h-4 w-4" /> Copy command
						{/if}
					</Button>
					<p class="text-xs text-muted-foreground">
						Run immediately on the target VPS. Each new node regenerates this script with its own
						SSH key.
					</p>
				</div>
			{:else}
				<div
					class="rounded-2xl border border-dashed bg-background/70 p-6 text-sm text-muted-foreground"
				>
					Link a node to generate the install-node-agent command.
				</div>
			{/if}
		</section>
	</div>

	<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
		<div class="flex flex-col gap-2">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div>
					<h2 class="text-lg font-semibold">Connected nodes</h2>
					<p class="text-sm text-muted-foreground">
						Compact view of every SSH target, its metadata, and the latest handshake.
					</p>
				</div>
				<Button variant="outline" class="gap-2" onclick={refreshNodes} disabled={refreshing}>
					{#if refreshing}
						<RefreshCw class="h-4 w-4 animate-spin" />
						Please wait
					{:else}
						Refresh list
					{/if}
				</Button>
			</div>
		</div>

		{#if nodes.length === 0}
			<div
				class="rounded-2xl border border-dashed bg-background/70 p-6 text-sm text-muted-foreground"
			>
				No remote servers yet. Use the form above to mint keys and copy the installer, then return
				here to store SSH metadata and trigger the first push.
			</div>
		{:else}
			<div class="space-y-6">
				{#each computedNodes as view (view.node.id)}
					<article class="space-y-5 rounded-2xl border bg-background p-5 shadow-sm">
						<header class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
							<div class="space-y-2">
								<div class="flex flex-wrap items-center gap-3">
									<h3 class="text-xl font-semibold">{view.node.name}</h3>
									<span
										class={`rounded-full px-3 py-1 text-xs font-medium ${nodeStatusColor(view.node.status)}`}
									>
										{nodeStatusLabel(view.node.status)}
									</span>
								</div>
								<p class="text-xs text-muted-foreground">Node ID · {view.node.id}</p>
								<p class="text-xs text-muted-foreground">Last heartbeat · {view.lastSeenText}</p>
							</div>
							<div class="flex flex-wrap items-center gap-2">
								<Button
									variant="ghost"
									size="sm"
									class="text-xs font-medium"
									onclick={() => openRenameEditor(view.node.id)}
								>
									Rename
								</Button>
								<AlertDialog
									open={deleteDialogId === view.node.id}
									onOpenChange={(event) => {
										deleteDialogId = event.detail ? view.node.id : null;
									}}
								>
									<AlertDialogTrigger>
										{#snippet child({ props })}
											<Button
												{...props}
												variant="ghost"
												size="icon"
												aria-label={`Delete ${view.node.name}`}
												onclick={(event) => {
													props.onClick?.(event);
													deleteDialogId = view.node.id;
												}}
											>
												<Trash2 class="h-4 w-4 text-destructive" />
											</Button>
										{/snippet}
									</AlertDialogTrigger>
									<AlertDialogContent>
										<AlertDialogHeader>
											<AlertDialogTitle>Remove server node?</AlertDialogTitle>
											<AlertDialogDescription>
												Deleting this node detaches future deployments targeting it.
											</AlertDialogDescription>
										</AlertDialogHeader>
										<AlertDialogFooter>
											<AlertDialogCancel onclick={() => (deleteDialogId = null)}>
												Cancel
											</AlertDialogCancel>
											<AlertDialogAction
												onclick={() => deleteNodeAction(view.node.id)}
												disabled={deleting[view.node.id]}
											>
												{#if deleting[view.node.id]}
													<RefreshCw class="mr-2 h-4 w-4 animate-spin" />
												{/if}
												Delete
											</AlertDialogAction>
										</AlertDialogFooter>
									</AlertDialogContent>
								</AlertDialog>
							</div>
						</header>

						{#if renameEditors[view.node.id]}
							<div class="flex flex-wrap items-end gap-2 rounded-xl border bg-card/70 p-4">
								<label class="flex min-w-[200px] flex-1 flex-col gap-1 text-xs font-medium">
									<span>Node label</span>
									<Input
										value={renameInputs[view.node.id] ?? view.node.name}
										oninput={(event) =>
											(renameInputs = {
												...renameInputs,
												[view.node.id]: event.currentTarget.value
											})}
										placeholder="edge-paris-01"
									/>
								</label>
								<Button
									size="sm"
									class="gap-2"
									onclick={() => handleRename(view.node.id)}
									disabled={renaming[view.node.id]}
								>
									{#if renaming[view.node.id]}
										<RefreshCw class="h-3.5 w-3.5 animate-spin" />
										Saving…
									{:else}
										Save
									{/if}
								</Button>
								<Button variant="ghost" size="sm" onclick={() => closeRenameEditor(view.node.id)}>
									Cancel
								</Button>
							</div>
						{/if}

						<div class="grid gap-4 md:grid-cols-3">
							<section class="space-y-3 rounded-xl border bg-card/80 p-4">
								<p class="text-xs tracking-wide text-muted-foreground uppercase">Connection</p>
								<dl class="space-y-2 text-sm text-muted-foreground">
									<div class="flex items-center justify-between gap-4">
										<dt class="text-xs tracking-wide uppercase">Host</dt>
										<dd class="font-medium text-foreground">{view.node.ssh_host ?? '—'}</dd>
									</div>
									<div class="flex items-center justify-between gap-4">
										<dt class="text-xs tracking-wide uppercase">Port</dt>
										<dd class="font-medium text-foreground">{view.node.ssh_port ?? 22}</dd>
									</div>
									<div class="flex items-center justify-between gap-4">
										<dt class="text-xs tracking-wide uppercase">User</dt>
										<dd class="font-medium text-foreground">
											{view.node.ssh_user ?? DEFAULT_SSH_USER}
										</dd>
									</div>
								</dl>
							</section>

							<section class="space-y-3 rounded-xl border bg-card/80 p-4">
								<p class="text-xs tracking-wide text-muted-foreground uppercase">Activity</p>
								<dl class="space-y-2 text-sm text-muted-foreground">
									<div class="flex items-center justify-between gap-4">
										<dt class="text-xs tracking-wide uppercase">Status</dt>
										<dd class="font-medium text-foreground">{nodeStatusLabel(view.node.status)}</dd>
									</div>
									<div class="flex items-center justify-between gap-4">
										<dt class="text-xs tracking-wide uppercase">Last echo</dt>
										<dd class="font-medium text-foreground">{view.lastHandshakeText}</dd>
									</div>
									<div class="flex items-center justify-between gap-4">
										<dt class="text-xs tracking-wide uppercase">Updated</dt>
										<dd class="font-medium text-foreground">
											{formatDateTime(view.node.updated_at)}
										</dd>
									</div>
								</dl>
							</section>

							<section class="space-y-3 rounded-xl border bg-card/80 p-4">
								<p class="text-xs tracking-wide text-muted-foreground uppercase">
									Commands & actions
								</p>
								<div class="space-y-2 text-xs text-muted-foreground">
									<div>
										<p class="font-semibold text-foreground">Bootstrap script</p>
										{#if view.node.install_command}
											<div
												class="mt-1 max-h-28 overflow-y-auto rounded-lg bg-background/80 p-3 font-mono text-[11px] leading-relaxed text-muted-foreground"
											>
												{view.node.install_command}
											</div>
											<Button
												variant="outline"
												class="mt-2 w-full gap-2"
												onclick={() => handleCopy(view.node.install_command, view.installKey)}
											>
												{#if copyState[view.installKey]}
													<Check class="h-4 w-4" /> Copied
												{:else}
													<Copy class="h-4 w-4" /> Copy bootstrap
												{/if}
											</Button>
										{:else}
											<p class="mt-1 text-xs">Node is active. Mint a new node to rotate keys.</p>
										{/if}
									</div>
									{#if view.node.update_command}
										<div>
											<p class="font-semibold text-foreground">Update agent</p>
											<p class="mt-1 text-xs text-muted-foreground">
												Reruns the installer to pull new dependencies (Docker, Postgres, etc.).
											</p>
											<Button
												variant="outline"
												class="mt-2 w-full gap-2"
												onclick={() => handleCopy(view.node.update_command, view.updateKey)}
											>
												{#if copyState[view.updateKey]}
													<Check class="h-4 w-4" /> Copied
												{:else}
													<RefreshCw class="h-4 w-4" /> Copy update command
												{/if}
											</Button>
										</div>
									{/if}
									<div>
										<p class="font-semibold text-foreground">SSH test</p>
										{#if view.handshakeCommand}
											<div
												class="mt-1 max-h-28 overflow-y-auto rounded-lg bg-background/80 p-3 font-mono text-[11px] leading-relaxed text-muted-foreground"
											>
												{view.handshakeCommand}
											</div>
											<Button
												variant="outline"
												class="mt-2 w-full gap-2"
												onclick={() => handleCopy(view.handshakeCommand, view.pushKey)}
											>
												{#if copyState[view.pushKey]}
													<Check class="h-4 w-4" /> Copied
												{:else}
													<Copy class="h-4 w-4" /> Copy SSH test
												{/if}
											</Button>
										{:else}
											<p class="mt-1 text-xs">
												Add SSH metadata to unlock the manual test command.
											</p>
										{/if}
									</div>
									{#if !view.needsPairing}
										<Button
											variant="outline"
											class="w-full gap-2"
											onclick={() => handleVerify(view.node.id)}
											disabled={verifying[view.node.id]}
										>
											{#if verifying[view.node.id]}
												<RefreshCw class="h-4 w-4 animate-spin" />
												Checking…
											{:else}
												Run handshake test
											{/if}
										</Button>
									{/if}
								</div>
							</section>
						</div>

						{#if view.needsPairing}
							<div class="space-y-4 rounded-2xl border border-dashed bg-background/70 p-4">
								<div>
									<p class="text-sm font-semibold">Pair this server</p>
									<p class="text-xs text-muted-foreground">
										Store host metadata once, then run the verification check to activate pushes.
									</p>
								</div>
								<div class="grid gap-3 sm:grid-cols-2">
									<label class="flex flex-col gap-1 text-xs font-medium">
										<span>Host / IP</span>
										<Input
											value={view.verifyInput.host}
											oninput={(event) =>
												updateVerifyInput(view.node.id, 'host', event.currentTarget.value)}
											placeholder="95.217.x.x"
										/>
									</label>
									<label class="flex flex-col gap-1 text-xs font-medium">
										<span>SSH port</span>
										<Input
											type="number"
											min="1"
											max="65535"
											value={view.verifyInput.port}
											oninput={(event) =>
												updateVerifyInput(view.node.id, 'port', event.currentTarget.value)}
										/>
									</label>
									<label class="flex flex-col gap-1 text-xs font-medium sm:col-span-2">
										<span>SSH user</span>
										<Input
											value={view.verifyInput.user}
											oninput={(event) =>
												updateVerifyInput(view.node.id, 'user', event.currentTarget.value)}
											placeholder={DEFAULT_SSH_USER}
										/>
									</label>
								</div>
								<Button
									class="w-full gap-2"
									onclick={() => handleVerify(view.node.id)}
									disabled={verifying[view.node.id]}
								>
									{#if verifying[view.node.id]}
										<RefreshCw class="h-4 w-4 animate-spin" />
										Verifying…
									{:else}
										Verify & activate
									{/if}
								</Button>
							</div>
						{/if}
					</article>
				{/each}
			</div>
		{/if}
	</section>
</section>
