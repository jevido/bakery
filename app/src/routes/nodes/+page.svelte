<script>
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { createNodeRecord, fetchNodes, deleteNode, verifyNode, renameNode } from '$lib/api.js';
	import { Copy, Check, RefreshCw, Trash2, Pencil } from '@lucide/svelte';
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

	function initializeVerifyInput(node) {
		if (!node) return;
		const existing = verifyInputs[node.id];
		if (existing) return;
		verifyInputs = {
			...verifyInputs,
			[node.id]: {
				host: node.ssh_host ?? '',
				port: node.ssh_port ?? 22
			}
		};
	}

	$effect(() => {
		for (const node of nodes ?? []) {
			initializeVerifyInput(node);
			if (!renameInputs[node.id]) {
				renameInputs = { ...renameInputs, [node.id]: node.name };
			}
		}
	});

	function nodeStatusLabel(status) {
		switch (status) {
			case 'active':
				return 'Active';
			case 'pending':
				return 'Awaiting verification';
			default:
				return status;
		}
	}

	function nodeStatusColor(status) {
		switch (status) {
			case 'active':
				return 'bg-emerald-100 text-emerald-700';
			case 'pending':
				return 'bg-amber-100 text-amber-700';
			default:
				return 'bg-slate-100 text-slate-600';
		}
	}

	async function refreshNodes() {
		refreshing = true;
		try {
			const payload = await fetchNodes();
			nodes = payload.nodes || [];
			nodes.forEach(initializeVerifyInput);
			controlPlane = payload.controlPlane ?? controlPlane;
			if (!nodes.some((node) => node.id === deleteDialogId)) {
				deleteDialogId = null;
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
		if (!newNodeName.trim()) {
			globalError = 'Provide a name for the server node.';
			return;
		}
		creating = true;
		try {
			const payload = await createNodeRecord({ name: newNodeName.trim() });
			nodes = [payload.node, ...nodes.filter((node) => node.id !== payload.node.id)];
			initializeVerifyInput(payload.node);
			latestInstaller =
				payload.node?.install_command != null
					? { nodeId: payload.node.id, command: payload.node.install_command }
					: null;
			globalMessage =
				'Node created. Run the install command on the server, then verify SSH access.';
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

	function updateVerifyInput(nodeId, field, value) {
		verifyInputs = {
			...verifyInputs,
			[nodeId]: {
				...(verifyInputs[nodeId] ?? { host: '', port: 22 }),
				[field]: value
			}
		};
	}

	async function handleVerify(nodeId) {
		globalError = '';
		globalMessage = '';
		const input = verifyInputs[nodeId] ?? { host: '', port: 22 };
		if (!input.host.trim()) {
			globalError = 'Provide the server host or IP address before verifying.';
			return;
		}
		verifying = { ...verifying, [nodeId]: true };
		try {
			const payload = await verifyNode(nodeId, {
				host: input.host.trim(),
				port: Number(input.port) || 22
			});
			nodes = nodes.map((node) => (node.id === nodeId ? payload.node : node));
			globalMessage = 'Node verified and activated.';
		} catch (error) {
			globalError = error?.message || 'Failed to verify node.';
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
			if (deleteDialogId === nodeId) {
				deleteDialogId = null;
			}
		} catch (error) {
			globalError = error?.message || 'Failed to delete node';
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
			return;
		}
		renaming = { ...renaming, [nodeId]: true };
		try {
			const payload = await renameNode(nodeId, nextName);
			nodes = nodes.map((node) => (node.id === nodeId ? payload.node : node));
			globalMessage = 'Node renamed.';
		} catch (error) {
			globalError = error?.message || 'Failed to rename node.';
		} finally {
			renaming = { ...renaming, [nodeId]: false };
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
			Connect additional servers to run the deployments on with the powerful combination of: Nginx &
			Docker. Automatically use blue/green deployments while managing everything from this control
			plane.
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
					No external servers yet. Create one using the form on the right to generate the installer
					command.
				</p>
			{:else}
				<div class="space-y-4">
					{#each nodes as node (node.id)}
						<div class="rounded-xl border bg-background p-4 shadow-sm">
							<div class="flex flex-wrap items-center justify-between gap-3">
								<div class="flex flex-col gap-2">
									<div class="flex items-center gap-2">
										<p class="font-medium">{node.name}</p>
										<Button
											variant="ghost"
											size="icon"
											class="h-8 w-8"
											onclick={() =>
												(renameInputs = {
													...renameInputs,
													[node.id]: renameInputs[node.id] ?? node.name ?? ''
												})}
											aria-label={`Rename ${node.name}`}
										>
											<Pencil class="h-4 w-4" />
										</Button>
									</div>
									<div class="flex flex-wrap items-end gap-2">
										<label class="flex flex-col gap-1 text-xs font-medium">
											<span>Rename</span>
											<Input
												class="w-48"
												value={renameInputs[node.id] ?? node.name}
												oninput={(event) =>
													(renameInputs = {
														...renameInputs,
														[node.id]: event.currentTarget.value
													})}
												placeholder="New name"
											/>
										</label>
										<Button
											variant="outline"
											size="sm"
											class="gap-2"
											onclick={() => handleRename(node.id)}
											disabled={renaming[node.id]}
										>
											{#if renaming[node.id]}
												<RefreshCw class="h-3.5 w-3.5 animate-spin" />
												Renaming…
											{:else}
												Save name
											{/if}
										</Button>
									</div>
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
								<p>SSH host: {node.ssh_host ?? '—'}</p>
							</div>

							{#if node.install_command}
								<div class="mt-4 space-y-2">
									<p class="text-sm font-medium">Run this command on the server:</p>
									<Textarea
										readonly
										class="h-28 w-full resize-none text-xs"
										value={node.install_command}
									/>
									<Button
										variant="outline"
										class="w-full gap-2"
										onclick={() => handleCopy(node.install_command, node.id)}
									>
										{#if copyState[node.id]}
											<Check class="h-4 w-4" />
											Copied
										{:else}
											<Copy class="h-4 w-4" />
											Copy command
										{/if}
									</Button>
								</div>
							{/if}

							{#if node.status !== 'active'}
								{@const verifyInput = verifyInputs[node.id] ?? { host: '', port: 22 }}
								<div class="mt-4 space-y-3">
									<p class="text-sm font-medium">
										Verify SSH access (after running the installer on the server)
									</p>
									<div class="grid gap-3 sm:grid-cols-2">
										<label class="flex flex-col gap-1 text-xs font-medium">
											<span>Host / IP</span>
											<Input
												value={verifyInput.host}
												oninput={(event) => updateVerifyInput(node.id, 'host', event.target.value)}
												placeholder="95.217.x.x"
											/>
										</label>
										<label class="flex flex-col gap-1 text-xs font-medium">
											<span>Port</span>
											<Input
												type="number"
												min="1"
												max="65535"
												value={verifyInput.port}
												oninput={(event) => updateVerifyInput(node.id, 'port', event.target.value)}
											/>
										</label>
									</div>
									<Button
										class="w-full gap-2"
										onclick={() => handleVerify(node.id)}
										disabled={verifying[node.id]}
									>
										{#if verifying[node.id]}
											<RefreshCw class="h-4 w-4 animate-spin" />
											Verifying…
										{:else}
											Verify & activate
										{/if}
									</Button>
								</div>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
			<h2 class="text-lg font-semibold">Link a server</h2>
			<p class="text-sm text-muted-foreground">
				Enter a friendly name and click <em>Link node</em>
				to copy the prefilled installer command. Run it on your VPS as root, wait for it to finish, then
				return here to verify SSH access.
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
					Link node
				</Button>
			</form>

			{#if latestInstaller?.command}
				<div class="space-y-2 rounded-lg border bg-background p-4">
					<p class="text-sm font-medium">Latest installer command</p>
					<Textarea
						readonly
						class="h-32 w-full resize-none text-xs"
						value={latestInstaller.command}
					/>
					<Button
						variant="outline"
						class="w-full gap-2"
						onclick={() => handleCopy(latestInstaller.command, latestInstaller.nodeId)}
					>
						{#if copyState[latestInstaller.nodeId]}
							<Check class="h-4 w-4" />
							Copied
						{:else}
							<Copy class="h-4 w-4" />
							Copy command
						{/if}
					</Button>
					<p class="text-xs text-muted-foreground">
						Run this on the target VPS right away. Each newly linked node will produce an updated
						command here and inside the node list.
					</p>
				</div>
			{/if}
		</section>
	</div>
</section>
