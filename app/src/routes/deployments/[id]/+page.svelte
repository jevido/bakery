<script>
	import { Button } from '$lib/components/ui/button';

	import {
		redeployDeployment,
		restartDeployment,
		rollbackDeployment,
		updateDeploymentEnv,
		deleteDeploymentEnv,
		addDeploymentDomain,
		removeDeploymentDomain,
		verifyDeploymentDomain,
		apiFetch
	} from '$lib/api.js';
	import { goto } from '$app/navigation';
	import {
		BadgeCheck,
		AlertTriangle,
		Clock,
		Loader2,
		Globe,
		Plus,
		X,
		RotateCcw,
		RefreshCw,
		ShieldCheck,
		Database,
		Clipboard
	} from '@lucide/svelte';

	let { data } = $props();

	let deployment = $state(data.deployment ?? {});
	let domains = $state(data.domains ?? []);
	let environment = $state(data.environment ?? []);
	let versions = $state(data.versions ?? []);
	let databases = $state(data.databases ?? []);
	let logs = $state(data.logs ?? []);
	let tasks = $state(data.tasks ?? []);

	let working = $state(false);
	let message = $state('');
	let error = $state('');

	let newDomain = $state('');
	let envKey = $state('');
	let envValue = $state('');
	let editingEnv = $state({});

	function formatDate(value) {
		if (!value) return '—';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return '—';
		return date.toLocaleString();
	}

	async function refreshDeployment() {
		try {
			const payload = await apiFetch(`/api/deployments/${deployment.id}`);
			deployment = payload.deployment ?? deployment;
			domains = payload.domains ?? domains;
			environment = payload.environment ?? environment;
			versions = payload.versions ?? versions;
			databases = payload.databases ?? databases;
			logs = payload.logs ?? logs;
			tasks = payload.tasks ?? tasks;
		} catch {
			// ignore refresh failures
		}
	}

	async function runAction(action, successMessage) {
		working = true;
		message = '';
		error = '';
		try {
			await action();
			message = successMessage;
			await refreshDeployment();
		} catch (err) {
			error = err?.message || 'Action failed';
		} finally {
			working = false;
		}
	}

	async function handleRedeploy() {
		await runAction(() => redeployDeployment(deployment.id), 'Redeploy task queued.');
	}

	async function handleRestart() {
		await runAction(() => restartDeployment(deployment.id), 'Restart task queued.');
	}

	async function handleRollback(versionId) {
		await runAction(() => rollbackDeployment(deployment.id, versionId), 'Rollback queued.');
	}

	async function addEnvVar() {
		if (!envKey.trim()) {
			error = 'Environment key is required.';
			return;
		}
		await runAction(async () => {
			await updateDeploymentEnv(deployment.id, envKey.trim(), envValue);
		}, 'Environment variable saved.');
		envKey = '';
		envValue = '';
	}

	async function updateExistingEnv(key) {
		const value = editingEnv[key] ?? '';
		await runAction(async () => {
			await updateDeploymentEnv(deployment.id, key, value);
		}, 'Environment variable updated.');
		editingEnv = { ...editingEnv, [key]: '' };
	}

	async function removeEnv(key) {
		await runAction(async () => {
			await deleteDeploymentEnv(deployment.id, key);
		}, 'Environment variable removed.');
	}

	async function addDomainAction() {
		const hostname = newDomain.trim();
		if (!hostname) {
			error = 'Domain hostname is required.';
			return;
		}
		await runAction(async () => {
			await addDeploymentDomain(deployment.id, hostname);
		}, 'Domain added.');
		newDomain = '';
	}

	async function removeDomainAction(domainId) {
		await runAction(async () => {
			await removeDeploymentDomain(deployment.id, domainId);
		}, 'Domain removed.');
	}

	async function verifyDomainAction(domainId) {
		await runAction(async () => {
			await verifyDeploymentDomain(domainId);
		}, 'Domain verification triggered.');
	}
</script>

<svelte:head>
	<title>{deployment.name} ~ The Bakery</title>
</svelte:head>

<section class="space-y-6 p-6 md:p-10">
	<header class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
		<div class="space-y-2">
			<div class="flex items-center gap-3">
				{#if deployment.status === 'active'}
					<BadgeCheck class="h-5 w-5 text-emerald-500" />
				{:else if deployment.status === 'failed'}
					<AlertTriangle class="h-5 w-5 text-rose-500" />
				{:else}
					<Clock class="h-5 w-5 text-amber-500" />
				{/if}
				<h1 class="text-3xl font-semibold tracking-tight">{deployment.name}</h1>
			</div>
			<p class="text-sm text-muted-foreground">
				{deployment.repository} · {deployment.branch} · Active slot {deployment.active_slot}
			</p>
		</div>
		<div class="flex flex-wrap gap-3">
			<Button variant="outline" onclick={() => goto('/deployments')}>Back to deployments</Button>
			<Button variant="outline" class="gap-2" onclick={handleRestart} disabled={working}>
				{#if working}
					<Loader2 class="h-4 w-4 animate-spin" />
				{/if}
				<RotateCcw class="h-4 w-4" />
				Restart
			</Button>
			<Button class="gap-2" onclick={handleRedeploy} disabled={working}>
				{#if working}
					<Loader2 class="h-4 w-4 animate-spin" />
				{/if}
				<RefreshCw class="h-4 w-4" />
				Redeploy
			</Button>
		</div>
	</header>

	{#if message}
		<div
			class="flex items-start gap-2 rounded-xl border border-emerald-400/40 bg-emerald-500/10 p-3 text-sm text-emerald-500"
		>
			<ShieldCheck class="mt-0.5 h-4 w-4" />
			<p>{message}</p>
		</div>
	{/if}

	{#if error}
		<div
			class="flex items-start gap-2 rounded-xl border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive"
		>
			<AlertTriangle class="mt-0.5 h-4 w-4" />
			<p>{error}</p>
		</div>
	{/if}

	<div class="grid gap-6 xl:grid-cols-[2fr,1fr]">
		<div class="space-y-6">
			<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
				<header>
					<h2 class="text-lg font-semibold">Domains</h2>
					<p class="text-sm text-muted-foreground">
						Add Namecheap-compatible DNS records and trigger verification to provision SSL
						automatically.
					</p>
				</header>
				<div class="flex flex-col gap-3 sm:flex-row">
					<input
						bind:value={newDomain}
						class="h-11 flex-1 rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
						placeholder="deploy.example.com"
						onkeydown={(event) => {
							if (event.key === 'Enter') {
								event.preventDefault();
								addDomainAction();
							}
						}}
					/>
					<Button class="sm:w-auto" onclick={addDomainAction} disabled={working}>
						<Plus class="h-4 w-4" />
						Add domain
					</Button>
				</div>

				{#if domains.length === 0}
					<p
						class="rounded-lg border border-dashed border-border/60 bg-background/60 p-4 text-sm text-muted-foreground"
					>
						No domains added yet. Create an A record pointing to your server IP, then add the
						hostname above.
					</p>
				{:else}
					<ul class="space-y-3">
						{#each domains as domain (domain.id)}
							<li class="rounded-lg border bg-background/60 p-4">
								<div class="flex flex-wrap items-center justify-between gap-3">
									<div>
										<p class="font-medium">{domain.hostname}</p>
										<p class="text-xs text-muted-foreground">
											Status {domain.verified ? 'Verified' : 'Pending'} · SSL {domain.ssl_status}
										</p>
									</div>
									<div class="flex items-center gap-2">
										<Button
											variant="outline"
											class="h-8 gap-2"
											onclick={() => verifyDomainAction(domain.id)}
											disabled={working}
										>
											<Globe class="h-3.5 w-3.5" />
											Verify DNS
										</Button>
										<Button
											variant="ghost"
											class="h-8 gap-2 text-destructive"
											onclick={() => removeDomainAction(domain.id)}
											disabled={working}
										>
											<X class="h-3.5 w-3.5" />
											Remove
										</Button>
									</div>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
				<header>
					<h2 class="text-lg font-semibold">Environment</h2>
					<p class="text-sm text-muted-foreground">
						Secrets are stored encrypted in PostgreSQL. Override values to trigger secure redeploys.
					</p>
				</header>
				<div class="grid gap-3 sm:grid-cols-[1fr,1fr,auto]">
					<input
						bind:value={envKey}
						class="h-11 rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
						placeholder="KEY"
					/>
					<input
						bind:value={envValue}
						class="h-11 rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
						placeholder="Secret value"
					/>
					<Button type="button" onclick={addEnvVar} disabled={working}>Add</Button>
				</div>
				{#if environment.length === 0}
					<p class="text-sm text-muted-foreground">No environment variables yet.</p>
				{:else}
					<ul class="space-y-3">
						{#each environment as item (item.key)}
							<li class="rounded-lg border bg-background/60 p-4">
								<div class="flex flex-wrap items-center justify-between gap-3">
									<div>
										<p class="text-sm font-medium">{item.key}</p>
										<p class="text-xs text-muted-foreground">
											Value stored securely. Set a new value to override.
										</p>
									</div>
									<div class="flex items-center gap-2">
										<Button
											variant="ghost"
											class="h-8 gap-1"
											onclick={() => {
												editingEnv = { ...editingEnv, [item.key]: editingEnv[item.key] ?? '' };
											}}
										>
											<Clipboard class="h-3.5 w-3.5" />
											Edit
										</Button>
										<Button
											variant="ghost"
											class="h-8 gap-1 text-destructive"
											onclick={() => removeEnv(item.key)}
											disabled={working}
										>
											<X class="h-3.5 w-3.5" />
											Remove
										</Button>
									</div>
								</div>
								{#if editingEnv[item.key] !== undefined}
									<div class="mt-3 grid gap-3 sm:grid-cols-[1fr,auto]">
										<input
											class="h-10 rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
											placeholder="New value"
											value={editingEnv[item.key]}
											oninput={(event) => {
												editingEnv = {
													...editingEnv,
													[item.key]: event.currentTarget.value
												};
											}}
										/>
										<div class="flex gap-2">
											<Button
												class="h-10"
												onclick={() => updateExistingEnv(item.key)}
												disabled={working}
											>
												Save
											</Button>
											<Button
												variant="outline"
												class="h-10"
												onclick={() => {
													editingEnv = { ...editingEnv, [item.key]: undefined };
												}}
											>
												Cancel
											</Button>
										</div>
									</div>
								{/if}
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
				<header>
					<h2 class="text-lg font-semibold">Version history</h2>
					<p class="text-sm text-muted-foreground">
						Bakery retains the last five releases per deployment to enable instant rollback.
					</p>
				</header>
				{#if versions.length === 0}
					<p class="text-sm text-muted-foreground">No historical versions captured yet.</p>
				{:else}
					<ul class="space-y-3">
						{#each versions as version (version.id)}
							<li class="rounded-lg border bg-background/60 p-4">
								<div class="flex flex-wrap items-center justify-between gap-3">
									<div>
										<p class="text-sm font-medium">
											{version.slot.toUpperCase()} slot · {version.status}
										</p>
										<p class="text-xs text-muted-foreground">
											Commit {version.commit_sha?.slice(0, 7) || '–'} · Port {version.port || '—'} ·
											Created {formatDate(version.created_at)}
										</p>
									</div>
									{#if version.status !== 'active'}
										<Button
											variant="outline"
											size="sm"
											onclick={() => handleRollback(version.id)}
											disabled={working}
										>
											Rollback here
										</Button>
									{/if}
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		</div>

		<aside class="space-y-6">
			<section class="space-y-3 rounded-2xl border bg-card p-6 shadow-sm">
				<header class="flex items-center gap-2">
					<Database class="h-4 w-4 text-muted-foreground" />
					<h2 class="text-lg font-semibold">Databases</h2>
				</header>
				{#if databases.length === 0}
					<p class="text-sm text-muted-foreground">
						No databases attached. Enable the provisioning option during a redeploy to create one
						automatically.
					</p>
				{:else}
					<ul class="space-y-3 text-sm">
						{#each databases as db (db.id)}
							<li class="rounded-lg border bg-background/60 p-3">
								<p class="font-medium">{db.name}</p>
								<p class="text-xs text-muted-foreground">
									Status {db.status} · Size {db.size_bytes && db.size_bytes > 0
										? `${(db.size_bytes / (1024 * 1024)).toFixed(1)} MB`
										: '—'}
								</p>
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			<section class="space-y-3 rounded-2xl border bg-card p-6 shadow-sm">
				<header>
					<h2 class="text-lg font-semibold">Recent logs</h2>
					<p class="text-sm text-muted-foreground">
						Latest deployment events recorded by the task worker.
					</p>
				</header>
				{#if logs.length === 0}
					<p class="text-sm text-muted-foreground">No logs yet.</p>
				{:else}
					<ul class="space-y-2 text-xs text-muted-foreground">
						{#each logs.slice(0, 10) as log (log.id)}
							<li class="rounded-lg border bg-background/60 p-3">
								<p class="font-semibold tracking-wide uppercase">{log.level}</p>
								<p class="text-foreground">{log.message}</p>
								<p class="mt-1 text-[11px]">{formatDate(log.created_at)}</p>
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			<section class="space-y-3 rounded-2xl border bg-card p-6 shadow-sm">
				<header>
					<h2 class="text-lg font-semibold">Task history</h2>
					<p class="text-sm text-muted-foreground">
						Relevant tasks for this deployment within the past day.
					</p>
				</header>
				{#if tasks.length === 0}
					<p class="text-sm text-muted-foreground">No recent tasks.</p>
				{:else}
					<ul class="space-y-3 text-xs text-muted-foreground">
						{#each tasks as task (task.id)}
							<li class="rounded-lg border bg-background/60 p-3">
								<p class="text-sm font-medium capitalize">{task.type}</p>
								<p class="mt-1 text-[11px]">
									Status {task.status} · {formatDate(task.updated_at)}
								</p>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		</aside>
	</div>
</section>
