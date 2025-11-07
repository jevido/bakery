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
let autoRefreshing = $state(false);
let logPanel;
const terminalLogs = $derived.by(() => {
	const streaming = logs.filter((log) => {
		const metadata = normalizeLogMetadata(log.metadata);
		return metadata && metadata.stream;
	});
	const source = streaming.length ? streaming : logs;
	return [...source].reverse();
});
const visibleVersions = $derived(() => {
	return Array.isArray(versions) ? versions.slice(0, 5) : [];
});
	const BUSY_DEPLOYMENT_STATUSES = new Set([
		'pending',
		'provisioning',
		'deploying',
		'initializing'
	]);
	const ACTIVE_TASK_STATUSES = new Set(['pending', 'running']);
	const REFRESH_INTERVAL_MS = 1500;

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

	function clearLogs() {
		logs = [];
	}

	$effect(() => {
		const shouldPoll =
			(deployment.status && BUSY_DEPLOYMENT_STATUSES.has(deployment.status)) ||
			tasks.some((task) => task.status && ACTIVE_TASK_STATUSES.has(task.status));
		autoRefreshing = shouldPoll;
		if (!shouldPoll) return;

		const interval = setInterval(() => {
			refreshDeployment();
		}, REFRESH_INTERVAL_MS);

		return () => {
			clearInterval(interval);
		};
	});

	$effect(() => {
		logs.length;
		if (!logPanel) return;
		requestAnimationFrame(() => {
			if (!logPanel) return;
			logPanel.scrollTop = logPanel.scrollHeight;
		});
	});

	function normalizeLogMetadata(metadata) {
		if (!metadata) return null;
		if (typeof metadata === 'object') return metadata;
		try {
			return JSON.parse(metadata);
		} catch {
			return null;
		}
	}

	function formatMetadataValue(value) {
		if (value == null) return '';
		if (typeof value === 'object') {
			try {
				return JSON.stringify(value, null, 2);
			} catch {
				return String(value);
			}
		}
		return String(value);
	}

	function getLogLevelClass(level) {
		const normalized = String(level || '').toLowerCase();
		if (['error', 'err', 'fatal'].includes(normalized)) return 'text-red-400';
		if (['warn', 'warning'].includes(normalized)) return 'text-amber-300';
		if (['success', 'ok'].includes(normalized)) return 'text-emerald-300';
		if (['debug', 'trace'].includes(normalized)) return 'text-sky-300';
		return 'text-slate-300';
	}

	async function runAction(action, successMessage) {
		working = true;
		message = '';
		error = '';
		autoRefreshing = true;
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
				{#if deployment.node}
					· Server {deployment.node.name}
					{#if deployment.node.status !== 'active'}
						({deployment.node.status})
					{/if}
				{/if}
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

	{#if autoRefreshing}
		<div
			class="flex items-start gap-3 rounded-xl border border-amber-400/40 bg-amber-500/10 p-3 text-sm text-amber-600"
		>
			<Loader2 class="mt-0.5 h-4 w-4 animate-spin" />
			<div>
				<p class="font-medium text-foreground/90">Deployment activity in progress</p>
				<p class="text-xs text-muted-foreground">
					Refreshing logs every few seconds so you can see when tasks finish or fail.
				</p>
			</div>
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

	<section class="space-y-3 rounded-2xl border bg-slate-950/90 p-4 text-slate-100 shadow-inner">
		<header class="flex flex-wrap items-center justify-between gap-3">
			<div>
				<h2 class="text-lg font-semibold text-white">Live task log</h2>
				<p class="text-xs text-slate-400">
					Events update automatically while work is running — newest lines appear at the bottom.
				</p>
			</div>
			<div class="flex gap-2">
				<Button
					variant="outline"
					class="border-slate-700 bg-transparent text-xs text-slate-200"
					onclick={refreshDeployment}
				>
					Refresh now
				</Button>
				<Button
					variant="ghost"
					class="border-slate-800 bg-transparent text-xs text-slate-300 hover:text-white"
					onclick={clearLogs}
				>
					Clear log
				</Button>
			</div>
		</header>
		<div
			class="h-64 overflow-y-auto rounded-xl border border-slate-800 bg-black/70 p-3 font-mono text-[11px] leading-relaxed"
			bind:this={logPanel}
		>
			{#if terminalLogs.length === 0}
				<p class="text-slate-500">No deployment logs yet.</p>
			{:else}
				<ul class="space-y-2">
					{#each terminalLogs as log (log.id ?? `${log.created_at}-${log.level}-${log.message}`)}
						{@const metadata = normalizeLogMetadata(log.metadata)}
						<li class="rounded-xl border border-slate-800/80 bg-black/30 p-2">
							<div class="flex flex-wrap items-center gap-2 text-[10px] uppercase tracking-wide text-slate-500">
								<span>{formatDate(log.created_at)}</span>
								{#if metadata?.stream}
									<span class="rounded border border-slate-800 px-1.5 py-0.5 text-[9px] font-semibold text-slate-200">
										{metadata.stream}
									</span>
								{/if}
								<span class={`ml-auto font-semibold ${getLogLevelClass(log.level)}`}>
									{(log.level || 'info').toUpperCase()}
								</span>
							</div>
							<p class="mt-1 whitespace-pre-wrap text-[11px] text-slate-100">
								{log.message}
							</p>
							{#if metadata}
								{@const metadataEntries = Object.entries(metadata).filter(([key]) => key !== 'stream')}
								{#if metadataEntries.length}
									<div class="mt-2 space-y-1 rounded-lg bg-black/40 p-2 text-[10px] text-slate-300">
										{#each metadataEntries as [key, value]}
											{@const rendered = formatMetadataValue(value)}
											{#if rendered}
												<div>
													<p class="font-semibold uppercase tracking-wide text-slate-500">{key}</p>
													{#if rendered.includes('\n')}
														<pre class="mt-1 whitespace-pre-wrap text-slate-300">{rendered}</pre>
													{:else}
														<p class="text-slate-200">{rendered}</p>
													{/if}
												</div>
											{/if}
										{/each}
									</div>
								{/if}
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</section>

	<div class="grid gap-6 xl:grid-cols-[2fr,1fr]">
		<div class="space-y-6">
			<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
				<header>
					<h2 class="text-lg font-semibold">Domains</h2>
					<p class="text-sm text-muted-foreground">
						Add DNS records and trigger verification to provision SSL automatically.
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
					<textarea
						bind:value={envValue}
						class="min-h-[44px] rounded-lg border border-input bg-background px-3 py-2 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
						placeholder="Secret value"
						rows="2"
					></textarea>
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
										<textarea
											class="min-h-[44px] rounded-lg border border-input bg-background px-3 py-2 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
											placeholder="New value"
											rows="3"
											value={editingEnv[item.key]}
											oninput={(event) => {
												editingEnv = {
													...editingEnv,
													[item.key]: event.currentTarget.value
												};
											}}
										></textarea>
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
						{#each visibleVersions as version (version.id)}
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

		</aside>
	</div>
</section>
