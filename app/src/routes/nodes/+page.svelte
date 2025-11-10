<script>
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { createNodeRecord, fetchNodes, pairNode, deleteNode } from '$lib/api.js';
	import { Copy, Check, RefreshCw, Trash2 } from '@lucide/svelte';
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

	let { data } = $props();

	let nodes = $state(data.nodes ?? []);
	let controlPlane = $state(data.controlPlane ?? null);
	let installContext = $state({
		blocked: data.install?.blocked,
		warning: data.install?.warning ?? null,
		apiBase: data.install?.apiBase ?? null
	});
	let installBlocked = $derived(installContext.blocked);
	let installWarning = $derived(installContext.warning);
	let installApiBase = $derived(installContext.apiBase);
	let newNodeName = $state('');
	let creating = $state(false);
	let refreshing = $state(false);
	let globalMessage = $state('');
	let globalError = $state('');
	let installInfo = $state(null);
	let copyState = $state({});
	let pairInputs = $state({});
	let pairingState = $state({});
	let deleting = $state({});
	let deleteDialogId = $state(null);

	function nodeStatusLabel(status) {
		switch (status) {
			case 'active':
				return 'Active';
			case 'awaiting_pairing':
				return 'Awaiting pairing';
			case 'pending':
				return 'Pending installation';
			default:
				return status;
		}
	}

	function nodeStatusColor(status) {
		switch (status) {
			case 'active':
				return 'bg-emerald-100 text-emerald-700';
			case 'awaiting_pairing':
				return 'bg-amber-100 text-amber-700';
			case 'pending':
				return 'bg-slate-100 text-slate-600';
			default:
				return 'bg-slate-100 text-slate-600';
		}
	}

	async function refreshNodes() {
		refreshing = true;
		try {
			const payload = await fetchNodes();
			nodes = payload.nodes || [];
			controlPlane = payload.controlPlane ?? controlPlane;
			const currentContext = installContext;
			installContext = {
				blocked: Boolean(payload.installBlocked ?? currentContext.blocked),
				warning: payload.installWarning ?? currentContext.warning,
				apiBase: payload.apiBase ?? currentContext.apiBase
			};
			if (!nodes.some((node) => node.id === deleteDialogId)) {
				deleteDialogId = null;
			}
			if (installContext.blocked) {
				installInfo = null;
				newNodeName = '';
			}
		} catch (error) {
			globalError = error?.message || 'Failed to refresh nodes';
		} finally {
			refreshing = false;
		}
	}

	async function handleCreate(event) {
		event.preventDefault();
		globalError = '';
		globalMessage = '';
		installInfo = null;
		if (!newNodeName.trim()) {
			globalError = 'Provide a name for the server node.';
			return;
		}
		creating = true;
		try {
			const payload = await createNodeRecord({ name: newNodeName.trim() });
			nodes = [payload.node, ...nodes.filter((node) => node.id !== payload.node.id)];
			installInfo = {
				nodeId: payload.node.id,
				command: payload.installCommand,
				warning: payload.installCommandWarning,
				apiBase: payload.apiBase
			};
			installContext = {
				blocked: installContext.blocked,
				warning: payload.installCommandWarning ?? installContext.warning,
				apiBase: payload.apiBase ?? installContext.apiBase
			};
			pairInputs = { ...pairInputs, [payload.node.id]: '' };
			if (payload.installCommand) {
				globalMessage =
					'Node created. Run the command below on the target server to install the agent.';
				globalError = '';
			} else {
				globalMessage = '';
				globalError =
					payload.installCommandWarning ?? 'Set BAKERY_BASE_URL to a public address and try again.';
			}
			newNodeName = '';
		} catch (error) {
			globalError = error?.message || 'Failed to create node';
		} finally {
			creating = false;
		}
	}

	async function handleCopy(command, nodeId) {
		try {
			await navigator.clipboard.writeText(command);
			copyState = { ...copyState, [nodeId]: true };
			setTimeout(() => {
				copyState = { ...copyState, [nodeId]: false };
			}, 2000);
		} catch {
			globalError = 'Unable to copy command to clipboard';
		}
	}

	async function submitPair(nodeId) {
		globalError = '';
		globalMessage = '';
		const code = pairInputs[nodeId]?.trim();
		if (!code) {
			globalError = 'Enter the pairing code from the server output.';
			return;
		}
		pairingState = { ...pairingState, [nodeId]: true };
		try {
			const payload = await pairNode(nodeId, code);
			nodes = nodes.map((node) => (node.id === nodeId ? payload.node : node));
			globalMessage = 'Node paired successfully.';
		} catch (error) {
			globalError = error?.message || 'Failed to pair node';
		} finally {
			pairingState = { ...pairingState, [nodeId]: false };
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
			if (deleteDialogId === nodeId) {
				deleteDialogId = null;
			}
		} catch (error) {
			globalError = error?.message || 'Failed to delete node';
		} finally {
			deleting = { ...deleting, [nodeId]: false };
		}
	}
</script>

<svelte:head>
	<title>Server nodes ~ The Bakery</title>
</svelte:head>

<section class="space-y-6 p-6 md:p-10">
	<header class="flex flex-col gap-2">
		<h1 class="text-3xl font-semibold tracking-tight">Server nodes</h1>
		<p class="text-sm text-muted-foreground">
			Connect additional servers to offload builds, Nginx, Docker, and blue/green deployments while
			managing everything from this control plane.
		</p>
	</header>

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
			class="flex items-start gap-2 rounded-xl border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive"
		>
			<p>{globalError}</p>
		</div>
	{/if}

	<div class="grid gap-6 lg:grid-cols-[2fr,1fr]">
		<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
			<div class="flex items-center justify-between">
				<h2 class="text-lg font-semibold">Connected servers</h2>
				<Button variant="outline" class="gap-2" onclick={refreshNodes} disabled={refreshing}>
					{#if refreshing}
						<RefreshCw class="h-4 w-4 animate-spin" />
					{/if}
					Refresh
				</Button>
			</div>

			{#if controlPlane}
				<div class="rounded-xl border border-primary/40 bg-primary/5 p-4 shadow-sm">
					<div class="flex flex-wrap items-center justify-between gap-3">
						<div>
							<p class="font-medium">{controlPlane.name}</p>
							<p class="text-xs text-muted-foreground">Node ID: {controlPlane.id}</p>
						</div>
						<span
							class="rounded-full bg-emerald-100 px-3 py-1 text-xs font-medium text-emerald-700"
						>
							{nodeStatusLabel(controlPlane.status)}
						</span>
					</div>
					<p class="mt-2 text-xs text-muted-foreground">
						Deployments can target this control plane to keep builds and runtime on the current
						server.
					</p>
				</div>
			{/if}

			{#if nodes.length === 0}
				<p class="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
					{#if installBlocked}
						No external servers yet. Update your Bakery base URL so remote machines can reach this
						instance, then return to add a node.
					{:else}
						No external servers yet. Create one using the form on the right to generate an install
						command.
					{/if}
				</p>
			{:else}
				<div class="space-y-4">
					{#each nodes as node (node.id)}
						<div class="rounded-xl border bg-background p-4 shadow-sm">
							<div class="flex flex-wrap items-center justify-between gap-3">
								<div>
									<p class="font-medium">{node.name}</p>
									<p class="text-xs text-muted-foreground">Node ID: {node.id}</p>
								</div>
								<div class="flex items-center gap-2">
									<span
										class={`rounded-full px-3 py-1 text-xs font-medium ${nodeStatusColor(node.status)}`}
									>
										{nodeStatusLabel(node.status)}
									</span>
									{#if node.status === 'pending'}
										<Button
											variant="ghost"
											size="icon"
											onclick={() => deleteNodeAction(node.id)}
											disabled={deleting[node.id]}
											aria-label={`Delete ${node.name}`}
										>
											{#if deleting[node.id]}
												<RefreshCw class="h-4 w-4 animate-spin" />
											{:else}
												<Trash2 class="h-4 w-4 text-destructive" />
											{/if}
										</Button>
									{:else}
										<AlertDialog
											open={deleteDialogId === node.id}
											onOpenChange={(event) => {
												deleteDialogId = event.detail ? node.id : null;
											}}
										>
											<AlertDialogTrigger>
												{#snippet child({ props })}
													<Button
														{...props}
														variant="ghost"
														size="icon"
														aria-label={`Delete ${node.name}`}
														onclick={(event) => {
															props.onClick?.(event);
															deleteDialogId = node.id;
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
														This node may be running active deployments. Deleting it will detach any
														future deployments assigned here.
													</AlertDialogDescription>
												</AlertDialogHeader>
												<AlertDialogFooter>
													<AlertDialogCancel onclick={() => (deleteDialogId = null)}>
														Cancel
													</AlertDialogCancel>
													<AlertDialogAction
														onclick={() => deleteNodeAction(node.id)}
														disabled={deleting[node.id]}
													>
														{#if deleting[node.id]}
															<RefreshCw class="mr-2 h-4 w-4 animate-spin" />
														{/if}
														Delete
													</AlertDialogAction>
												</AlertDialogFooter>
											</AlertDialogContent>
										</AlertDialog>
									{/if}
								</div>
							</div>

							<div class="mt-3 grid gap-2 text-sm text-muted-foreground sm:grid-cols-2">
								<p>Last seen: {node.last_seen ? new Date(node.last_seen).toLocaleString() : '–'}</p>
								<p>Public IP: {node.metadata?.publicIp ?? '—'}</p>
							</div>

							{#if node.status === 'awaiting_pairing'}
								<div class="mt-4 flex flex-col gap-2 sm:flex-row sm:items-center">
									<Input
										bind:value={pairInputs[node.id]}
										placeholder="Enter pairing code"
										class="sm:max-w-xs"
									/>
									<Button
										class="gap-2"
										onclick={() => submitPair(node.id)}
										disabled={pairingState[node.id]}
									>
										{#if pairingState[node.id]}
											<RefreshCw class="h-4 w-4 animate-spin" />
										{/if}
										Submit code
									</Button>
								</div>
							{:else if installInfo?.nodeId === node.id}
								<p class="mt-3 text-xs text-muted-foreground">
									Pairing complete. This node is ready for deployments.
								</p>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
			<h2 class="text-lg font-semibold">Add a server</h2>
			{#if installBlocked}
				<div
					class="flex items-start gap-2 rounded-xl border border-amber-400/40 bg-amber-400/10 p-3 text-sm text-amber-600"
				>
					<RefreshCw class="mt-0.5 h-4 w-4" />
					<p>
						{installWarning ??
							'External servers require BAKERY_BASE_URL to point at a publicly reachable domain or IP.'}
						{#if installApiBase}
							<br />
							Current base URL:
							<code class="font-mono">{installApiBase}</code>
						{/if}
					</p>
				</div>
			{:else}
				<p class="text-sm text-muted-foreground">
					Give the server a friendly name. After creating it you'll receive a one-line install
					command to run on the remote machine. The installer prints a pairing code that you paste
					here to authorize the link.
				</p>

				<form class="space-y-3" onsubmit={handleCreate}>
					<label class="flex flex-col gap-1 text-sm font-medium">
						<span>Node name</span>
						<Input placeholder="hetzner-fsn1-primary" bind:value={newNodeName} />
					</label>
					<Button class="w-full" type="submit" disabled={creating}>
						{#if creating}
							<RefreshCw class="mr-2 h-4 w-4 animate-spin" />
						{/if}
						Create node
					</Button>
				</form>

				{#if installInfo}
					<div class="space-y-2 rounded-lg border bg-background p-4">
						{#if installInfo.command}
							<p class="text-sm font-medium">Run this command on the target server:</p>
							<Textarea
								readonly
								class="h-32 w-full resize-none text-xs"
								value={installInfo.command}
							/>
							<Button
								variant="outline"
								class="w-full gap-2"
								onclick={() => handleCopy(installInfo.command, installInfo.nodeId)}
							>
								{#if copyState[installInfo.nodeId]}
									<Check class="h-4 w-4" />
									Copied
								{:else}
									<Copy class="h-4 w-4" />
									Copy command
								{/if}
							</Button>
							<p class="text-xs text-muted-foreground">
								After the installer finishes, copy the pairing code it prints and submit it above to
								activate the node.
							</p>
						{:else}
							<p class="text-sm font-medium">Control plane URL is not reachable by remote nodes.</p>
							<p class="text-xs text-muted-foreground">
								{installInfo.warning ??
									'Set BAKERY_BASE_URL to a public IP or domain so servers can reach this Bakery instance.'}
							</p>
							{#if installInfo.apiBase}
								<p class="text-xs text-muted-foreground">
									Current base URL: <code class="font-mono">{installInfo.apiBase}</code>
								</p>
							{/if}
						{/if}
					</div>
				{/if}
			{/if}
		</section>
	</div>
</section>
