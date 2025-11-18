<script>
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';

	import {
		redeployDeployment,
		restartDeployment,
		rollbackDeployment,
		updateDeploymentEnv,
		deleteDeploymentEnv,
		addDeploymentDomain,
		removeDeploymentDomain,
		verifyDeploymentDomain,
		deleteDeployment,
		apiFetch,
		stopDeployment,
		startDeployment,
		provisionDeploymentDatabase
	} from '$lib/api.js';
	import { isLocalHostname } from '$lib/shared/domainRules.js';
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
		Clipboard,
		Trash2,
		Activity,
		HardDrive,
		Layers,
		ExternalLink,
		Play,
		Square,
		PauseCircle
	} from '@lucide/svelte';

	let { data } = $props();

	let deployment = $state(data.deployment ?? {});
	let domains = $state(data.domains ?? []);
	let environment = $state(data.environment ?? []);
	let versions = $state(data.versions ?? []);
	let databases = $state(data.databases ?? []);
let logs = $state(data.logs ?? []);
let tasks = $state(data.tasks ?? []);
let resourceSummary = $state(data.resourceSummary ?? null);
let runtimeStatus = $state(data.runtimeStatus ?? null);
let autoRefreshing = $state(false);
let activeTab = $state('overview');
let working = $state(false);
let workingAction = $state(null);
let message = $state('');
let error = $state('');
let newDomain = $state('');
let envKey = $state('');
let envValue = $state('');
let editingEnv = $state({});
let deletingDeployment = $state(false);
let deleteDialogOpen = $state(false);
let copyFeedback = $state('');
let copyTimeout;
let logPanel;
let logPinned = true;
	const MIGRATION_COMMAND = 'bun run db:migrate';
	const LOCAL_DOMAIN_MESSAGE =
		'Local-only hostnames (like *.local, *.localhost, or private IPs) are disabled for now. We will reintroduce local overrides in a future release.';

	const BUSY_DEPLOYMENT_STATUSES = new Set([
		'pending',
		'provisioning',
		'deploying',
		'initializing'
	]);
	const ACTIVE_TASK_STATUSES = new Set(['pending', 'running']);
	const REFRESH_INTERVAL_MS = 1500;
	const HEALTHY_DEPLOYMENT_STATUSES = new Set(['active', 'running']);

	const terminalLogs = $derived.by(() => {
		const streaming = logs.filter((log) => {
			const metadata = normalizeLogMetadata(log.metadata);
			return metadata && metadata.stream;
		});
		const source = streaming.length ? streaming : logs;
		return [...source].reverse();
	});

	const visibleVersions = $derived.by(() => {
		return Array.isArray(versions) ? versions.slice(0, 5) : [];
	});

	const diskUsage = $derived(resourceSummary?.disk?.used_bytes ?? null);
	const diskCapturedAt = $derived(resourceSummary?.disk?.captured_at ?? null);
	const databaseFootprint = $derived.by(
		() =>
			resourceSummary?.database?.size_bytes ??
			databases.reduce((total, db) => total + (db.size_bytes || 0), 0)
	);
	const databaseCapturedAt = $derived(resourceSummary?.database?.captured_at ?? null);
	const trafficSnapshot = $derived(resourceSummary?.traffic ?? null);
	const totalDomains = $derived(domains.length);
	const verifiedDomains = $derived(domains.filter((domain) => domain.verified).length);
	const mutableTasks = $derived(tasks.slice(0, 5));
	const publicVisitDomain = $derived.by(() => {
		if (!Array.isArray(domains) || domains.length === 0) {
			return null;
		}
		const sorted = [...domains].sort((a, b) => Number(b.verified) - Number(a.verified));
		return sorted.find((domain) => domain?.hostname && !isLocalHostname(domain.hostname)) ?? null;
	});
	const deploymentUrl = $derived.by(() => {
		const domain = publicVisitDomain;
		if (!domain) {
			return null;
		}
		const secureStatuses = new Set(['active', 'ready']);
		const protocol = secureStatuses.has(domain.ssl_status) || domain.verified ? 'https' : 'http';
		return `${protocol}://${domain.hostname}`;
	});
	const runtimeMismatch = $derived.by(() => {
		if (!runtimeStatus) return false;
		if (runtimeStatus.state === 'unknown') return false;
		if (runtimeStatus.state === 'inactive') return false;
		return deployment.status === 'running' && runtimeStatus.state !== 'running';
	});

	function isHealthyStatus(status) {
		if (!status) return false;
		return HEALTHY_DEPLOYMENT_STATUSES.has(String(status).toLowerCase());
	}

	function isBusyStatus(status) {
		if (!status) return false;
		return BUSY_DEPLOYMENT_STATUSES.has(String(status).toLowerCase());
	}

function formatDate(value) {
	if (!value) return '—';
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return '—';
	return date.toLocaleString();
}

function formatLogTimestamp(value) {
	if (!value) return '--:--:--';
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return '--:--:--';
	return date.toLocaleTimeString([], {
		hour12: false,
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit'
	});
}

function formatBytes(value) {
	if (value == null || Number.isNaN(Number(value))) return '—';
	let size = Number(value);
	if (!Number.isFinite(size) || size < 0) return '—';
	const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
		let idx = 0;
		while (size >= 1024 && idx < units.length - 1) {
			size /= 1024;
			idx += 1;
		}
		const precision = idx === 0 ? 0 : 1;
		return `${size.toFixed(precision)} ${units[idx]}`;
	}

	function formatMetadataValue(value) {
		if (value == null) return '';
		if (typeof value === 'string') return value;
		if (typeof value === 'number' || typeof value === 'boolean') return String(value);
		try {
			return JSON.stringify(value, null, 2);
		} catch {
			return String(value);
		}
	}

function getLogLevelClass(level) {
	const normalized = String(level || '').toLowerCase();
	if (['error', 'err', 'fatal'].includes(normalized)) return 'text-red-400';
	if (['warn', 'warning'].includes(normalized)) return 'text-amber-300';
	if (['success', 'ok'].includes(normalized)) return 'text-emerald-300';
	if (['debug', 'trace'].includes(normalized)) return 'text-sky-300';
	return 'text-slate-300';
}

function formatMetadataInline(value) {
	if (value == null) return '';
	if (typeof value === 'string') {
		return value.length > 120 ? `${value.slice(0, 117)}...` : value;
	}
	try {
		const rendered = JSON.stringify(value);
		return rendered.length > 160 ? `${rendered.slice(0, 157)}...` : rendered;
	} catch {
		return String(value);
	}
}

function formatMetadataExtras(metadata) {
	const normalized = normalizeLogMetadata(metadata);
	if (!normalized) return { stream: null, entries: [] };
	const { stream, ...rest } = normalized;
	const entries = Object.entries(rest)
		.map(([key, value]) => {
			const rendered = formatMetadataInline(value);
			return rendered ? `${key}=${rendered}` : '';
		})
		.filter(Boolean);
	return { stream, entries };
}

function normalizeLogMetadata(metadata) {
	if (!metadata) return null;
	if (typeof metadata === 'object') return metadata;
	try {
		return JSON.parse(metadata);
		} catch {
			return null;
		}
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
			resourceSummary = payload.resourceSummary ?? resourceSummary;
			runtimeStatus = payload.runtimeStatus ?? runtimeStatus;
		} catch {
			// ignore refresh failures
		}
	}

	function clearLogs() {
		logs = [];
		logPinned = true;
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

		return () => clearInterval(interval);
	});

	$effect(() => {
		logs.length;
		if (!logPanel || !logPinned) return;
		requestAnimationFrame(() => {
			if (!logPanel || !logPinned) return;
			logPanel.scrollTop = logPanel.scrollHeight;
		});
	});

	function handleLogScroll() {
		if (!logPanel) return;
		const nearBottom =
			logPanel.scrollTop + logPanel.clientHeight >= logPanel.scrollHeight - 24;
		logPinned = nearBottom;
	}

	function jumpToLatest() {
		if (!logPanel) return;
		logPanel.scrollTop = logPanel.scrollHeight;
		logPinned = true;
	}

	function describeRuntimeStatus(status) {
		if (!status) return 'Unknown';
		switch (status.state) {
			case 'running':
				return 'Process running';
			case 'stopped':
				return 'Process stopped';
			case 'inactive':
				return 'Not deployed yet';
			case 'unknown':
				return 'Process unknown';
			default:
				return 'Process unknown';
		}
	}

	function runtimeStatusTone(status) {
		if (!status) return 'text-muted-foreground';
		switch (status.state) {
			case 'running':
				return 'text-emerald-600';
			case 'stopped':
				return 'text-rose-600';
			case 'unknown':
				return 'text-amber-600';
			default:
				return 'text-amber-600';
		}
	}

	async function runAction(action, successMessage, actionName = null) {
		working = true;
		workingAction = actionName;
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
			workingAction = null;
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

	async function handleDeleteDeployment() {
		if (deletingDeployment) return;
		deletingDeployment = true;
		error = '';
		message = '';
		try {
			await deleteDeployment(deployment.id);
			await goto('/deployments');
		} catch (err) {
			error = err?.message || 'Failed to delete deployment.';
		} finally {
			deletingDeployment = false;
			deleteDialogOpen = false;
		}
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
		editingEnv = { ...editingEnv, [key]: undefined };
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
		if (isLocalHostname(hostname)) {
			error = LOCAL_DOMAIN_MESSAGE;
			return;
		}
		await runAction(
			async () => {
				await addDeploymentDomain(deployment.id, hostname);
			},
			'Domain added.',
			'add-domain'
		);
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

	async function handleStop() {
		await runAction(() => stopDeployment(deployment.id), 'Stop command queued.', 'stop');
	}

	async function handleStart() {
		await runAction(() => startDeployment(deployment.id), 'Start command queued.', 'start');
	}

	async function provisionDatabaseAction() {
		await runAction(
			() => provisionDeploymentDatabase(deployment.id),
			'Database provisioning started.',
			'provision-db'
		);
	}

	async function copyToClipboard(value, label = 'Value') {
		if (!value) return;
		if (typeof navigator === 'undefined' || !navigator.clipboard) {
			copyFeedback = 'Clipboard is unavailable in this environment.';
			clearTimeout(copyTimeout);
			copyTimeout = setTimeout(() => {
				copyFeedback = '';
			}, 2500);
			return;
		}
		try {
			await navigator.clipboard.writeText(value);
			copyFeedback = `${label} copied`;
		} catch {
			copyFeedback = 'Failed to copy text';
		}
		clearTimeout(copyTimeout);
		copyTimeout = setTimeout(() => {
			copyFeedback = '';
		}, 2000);
	}
</script>

<svelte:head>
	<title>{deployment.name} ~ The Bakery</title>
</svelte:head>

<section class="space-y-6 p-6 md:p-10">
	<header class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
		<div class="space-y-2">
			<div class="flex items-center gap-3">
				{#if isHealthyStatus(deployment.status)}
					<BadgeCheck class="h-5 w-5 text-emerald-500" />
				{:else if deployment.status === 'failed'}
					<AlertTriangle class="h-5 w-5 text-rose-500" />
				{:else if isBusyStatus(deployment.status)}
					<Clock class="h-5 w-5 text-amber-500" />
				{:else}
					<PauseCircle class="h-5 w-5 text-muted-foreground" />
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
			<Button variant="outline" href="/deployments">Back to deployments</Button>
			{#if deploymentUrl}
				<Button
					variant="secondary"
					class="gap-2"
					href={deploymentUrl}
					target="_blank"
					rel="noreferrer"
				>
					<ExternalLink class="h-4 w-4" />
					Open app
				</Button>
			{/if}
			{#if deployment.status === 'running'}
				<Button variant="destructive" class="gap-2" onclick={handleStop} disabled={working}>
					{#if working && workingAction === 'stop'}
						<Loader2 class="h-4 w-4 animate-spin" />
					{:else}
						<Square class="h-4 w-4" />
					{/if}
					Stop
				</Button>
			{:else if deployment.status === 'inactive'}
				<Button variant="outline" class="gap-2" onclick={handleStart} disabled={working}>
					{#if working && workingAction === 'start'}
						<Loader2 class="h-4 w-4 animate-spin" />
					{:else}
						<Play class="h-4 w-4" />
					{/if}
					Start
				</Button>
			{/if}
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

	{#if deployment.status === 'failed'}
		<div
			class="flex items-start gap-3 rounded-xl border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive"
		>
			<AlertTriangle class="mt-0.5 h-4 w-4" />
			<div>
				<p class="font-semibold text-destructive">Runtime crashed</p>
				<p class="text-xs text-muted-foreground">
					The app exited unexpectedly. Review the live logs below, fix the error, then redeploy to
					bring it back online.
				</p>
			</div>
		</div>
	{/if}

	{#if runtimeMismatch}
		<div class="flex items-start gap-3 rounded-xl border border-rose-400/40 bg-rose-500/10 p-3 text-sm text-rose-600">
			<AlertTriangle class="mt-0.5 h-4 w-4" />
			<div>
				<p class="font-semibold text-rose-700">Runtime appears offline</p>
				<p class="text-xs text-rose-600/90">
					We couldn&rsquo;t detect an active process even though the deployment is marked as running.
					Restart or redeploy to bring it back online.
				</p>
			</div>
		</div>
	{:else if runtimeStatus?.state === 'unknown'}
		<div class="flex items-start gap-3 rounded-xl border border-slate-400/40 bg-slate-500/10 p-3 text-sm text-slate-600">
			<Clock class="mt-0.5 h-4 w-4" />
			<div>
				<p class="font-semibold text-muted-foreground">Runtime status pending</p>
				<p class="text-xs text-muted-foreground/90">
					Remote nodes report their status separately. Refresh or inspect the server directly if this
					persists.
				</p>
			</div>
		</div>
	{/if}

	<Tabs.Root bind:value={activeTab} class="space-y-4">
		<Tabs.List class="w-full flex-wrap rounded-2xl border bg-muted/40 p-1">
			<Tabs.Trigger value="overview" class="min-w-[120px] flex-1">Overview</Tabs.Trigger>
			<Tabs.Trigger value="environment" class="min-w-[120px] flex-1">Environment</Tabs.Trigger>
			<Tabs.Trigger value="database" class="min-w-[120px] flex-1">Database</Tabs.Trigger>
			<Tabs.Trigger value="danger" class="min-w-[120px] flex-1">Danger zone</Tabs.Trigger>
		</Tabs.List>

		<Tabs.Content value="overview" class="space-y-6">
			<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
				<div class="rounded-2xl border bg-card p-4 shadow-sm">
					<p class="text-xs text-muted-foreground uppercase">Deployment status</p>
					<div class="mt-2 flex items-center gap-2 text-lg font-semibold capitalize">
						{deployment.status || 'unknown'}
					</div>
					<p class="text-xs text-muted-foreground">Updated {formatDate(deployment.updated_at)}</p>
				</div>
				<div class="rounded-2xl border bg-card p-4 shadow-sm">
					<p class="text-xs text-muted-foreground uppercase">Runtime process</p>
					<div class={`mt-2 text-lg font-semibold ${runtimeStatusTone(runtimeStatus)}`}>
						{describeRuntimeStatus(runtimeStatus)}
					</div>
					<p class="text-xs text-muted-foreground">
						{#if runtimeStatus?.service}
							Service {runtimeStatus.service}
						{:else if runtimeStatus?.reason === 'remote_node'}
							Reported by remote agent
						{:else}
							Status is approximate
						{/if}
					</p>
				</div>
				<div class="rounded-2xl border bg-card p-4 shadow-sm">
					<p class="text-xs text-muted-foreground uppercase">Domains</p>
					<div class="mt-2 flex items-center gap-2 text-lg font-semibold">
						{verifiedDomains}/{totalDomains}
						<span class="text-sm font-normal text-muted-foreground">verified</span>
					</div>
					<p class="text-xs text-muted-foreground">
						{deployment.active_slot?.toUpperCase()} slot · {deployment.node?.name ||
							'Control plane'}
					</p>
				</div>
				<div class="rounded-2xl border bg-card p-4 shadow-sm">
					<div class="flex items-center gap-2 text-xs text-muted-foreground uppercase">
						<HardDrive class="h-4 w-4 text-muted-foreground" />
						Disk usage
					</div>
					<p class="mt-2 text-lg font-semibold">{formatBytes(diskUsage)}</p>
					<p class="text-xs text-muted-foreground">
						Last sampled {diskCapturedAt ? formatDate(diskCapturedAt) : '—'}
					</p>
				</div>
				<div class="rounded-2xl border bg-card p-4 shadow-sm">
					<div class="flex items-center gap-2 text-xs text-muted-foreground uppercase">
						<Database class="h-4 w-4 text-muted-foreground" />
						Database impact
					</div>
					<p class="mt-2 text-lg font-semibold">{formatBytes(databaseFootprint)}</p>
					<p class="text-xs text-muted-foreground">
						Last sampled {databaseCapturedAt ? formatDate(databaseCapturedAt) : '—'}
					</p>
				</div>
			</div>

			<div class="grid gap-6 xl:grid-cols-[2fr,1fr]">
				<div class="space-y-6">
					<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
						<header class="flex flex-wrap items-center justify-between gap-3">
							<div>
								<h2 class="text-lg font-semibold">Domains</h2>
								<p class="text-sm text-muted-foreground">
									Add DNS records and trigger verification to provision SSL (skipped for local
									targets).
								</p>
							</div>
							{#if deploymentUrl}
								<Button
									variant="secondary"
									class="gap-2"
									href={deploymentUrl}
									target="_blank"
									rel="noreferrer"
								>
									<ExternalLink class="h-4 w-4" />
									Visit app
								</Button>
							{/if}
						</header>
						<div class="flex flex-col gap-3 sm:flex-row">
							<Input
								bind:value={newDomain}
								class="h-11 flex-1"
								placeholder="deploy.example.com"
								onkeydown={(event) => {
									if (event.key === 'Enter') {
										event.preventDefault();
										addDomainAction();
									}
								}}
							/>
							<Button class="gap-2 sm:w-auto" onclick={addDomainAction} disabled={working}>
								{#if working && workingAction === 'add-domain'}
									<Loader2 class="h-4 w-4 animate-spin" />
								{:else}
									<Plus class="h-4 w-4" />
								{/if}
								Add domain
							</Button>
						</div>
						<p class="text-[11px] text-muted-foreground">
							{LOCAL_DOMAIN_MESSAGE}
						</p>

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
													Status {domain.verified ? 'Verified' : 'Pending'} · TLS {domain.ssl_status}
												</p>
											</div>
											<div class="flex items-center gap-2">
												<Button
													variant="outline"
													class="h-8 gap-2"
													onclick={() => verifyDomainAction(domain.id)}
													disabled={working || domain?.resolution_hint?.localOnly}
												>
													<Globe class="h-3.5 w-3.5" />
													Verify
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
										{#if domain.resolution_hint}
											<div
												class="mt-3 space-y-2 rounded-lg border border-dashed border-muted bg-muted/20 p-3 text-xs text-muted-foreground"
											>
												<p class="font-semibold text-foreground">Local override</p>
												<p>{domain.resolution_hint.instructions}</p>
												<pre
													class="rounded bg-background px-2 py-1 font-mono text-[11px] text-foreground">
{domain.resolution_hint.target} {domain.hostname}</pre>
												<p class="text-[11px]">TLS is skipped automatically for this domain.</p>
											</div>
										{/if}
									</li>
								{/each}
							</ul>
						{/if}
					</section>

					<section
						class="space-y-3 rounded-2xl border bg-slate-950/90 p-4 text-slate-100 shadow-inner"
					>
						<header class="flex flex-wrap items-center justify-between gap-3">
							<div>
								<h2 class="text-lg font-semibold text-white">Live log</h2>
								<p class="text-xs text-slate-400">
									Events update automatically while work is running — newest lines appear at the
									bottom.
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
								{#if !logPinned && terminalLogs.length}
									<Button
										variant="ghost"
										class="border-slate-800 bg-transparent text-xs text-slate-300 hover:text-white"
										onclick={jumpToLatest}
									>
										Jump to latest
									</Button>
								{/if}
							</div>
						</header>
						<div
							class="h-64 overflow-y-auto rounded-xl border border-slate-800 bg-black/70 p-3 font-mono text-[11px] leading-relaxed"
							bind:this={logPanel}
							onscroll={handleLogScroll}
						>
							{#if terminalLogs.length === 0}
								<p class="text-slate-500">No deployment logs yet.</p>
							{:else}
								<div class="space-y-1">
									{#each terminalLogs as log (log.id ?? `${log.created_at}-${log.level}-${log.message}`)}
										{@const { stream, entries } = formatMetadataExtras(log.metadata)}
										<div class="whitespace-pre-wrap text-[11px] leading-relaxed text-slate-100">
											<span class="text-slate-500">
												[{formatLogTimestamp(log.created_at)}]
											</span>
											{#if stream}
												<span class="text-slate-400">[{stream}]</span>
											{/if}
											<span class={`font-semibold ${getLogLevelClass(log.level)}`}>
												[{(log.level || 'info').toUpperCase()}]
											</span>
											<span class="text-slate-100">{log.message}</span>
											{#if entries.length}
												<span class="text-slate-400"> -- {entries.join(' ')}</span>
											{/if}
										</div>
									{/each}
								</div>
							{/if}
						</div>
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
								No databases attached. Enable the provisioning option during a redeploy to create
								one automatically.
							</p>
						{:else}
							<ul class="space-y-3 text-sm">
								{#each databases as db (db.id)}
									<li class="rounded-lg border bg-background/60 p-3">
										<p class="font-medium">{db.name}</p>
										<p class="text-xs text-muted-foreground">
											Status {db.status} · Size {db.size_bytes && db.size_bytes > 0
												? formatBytes(db.size_bytes)
												: '—'}
										</p>
									</li>
								{/each}
							</ul>
						{/if}
					</section>

					<section class="space-y-3 rounded-2xl border bg-card p-6 shadow-sm">
						<header class="flex items-center gap-2">
							<Activity class="h-4 w-4 text-muted-foreground" />
							<h2 class="text-lg font-semibold">Recent tasks</h2>
						</header>
						{#if mutableTasks.length === 0}
							<p class="text-sm text-muted-foreground">No tasks queued in the last day.</p>
						{:else}
							<ul class="space-y-3 text-sm">
								{#each mutableTasks as task (task.id)}
									<li class="rounded-lg border bg-background/60 p-3">
										<p class="font-medium capitalize">{task.type}</p>
										<p class="text-xs text-muted-foreground">
											Status {task.status}
											{#if task.updated_at}
												· Updated {formatDate(task.updated_at)}
											{/if}
										</p>
									</li>
								{/each}
							</ul>
						{/if}
					</section>

					{#if trafficSnapshot}
						<section class="space-y-3 rounded-2xl border bg-card p-6 shadow-sm">
							<header class="flex items-center gap-2">
								<Layers class="h-4 w-4 text-muted-foreground" />
								<h2 class="text-lg font-semibold">Traffic snapshot</h2>
							</header>
							<p class="text-sm text-muted-foreground">
								Visits {trafficSnapshot.visits ?? 0} · Bandwidth {formatBytes(
									trafficSnapshot.bandwidth ?? 0
								)}
							</p>
							<p class="text-xs text-muted-foreground">
								Captured {formatDate(trafficSnapshot.captured_at)}
							</p>
						</section>
					{/if}
				</aside>
			</div>
		</Tabs.Content>

		<Tabs.Content value="database" class="space-y-6">
			<section class="space-y-3 rounded-2xl border bg-card p-6 shadow-sm">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<h2 class="text-xl font-semibold">Managed Postgres</h2>
						<p class="text-sm text-muted-foreground">
							Provision an isolated Postgres database for this deployment and automatically inject
							the DATABASE_URL secret.
						</p>
					</div>
					<Button class="gap-2" onclick={provisionDatabaseAction} disabled={working}>
						{#if working && workingAction === 'provision-db'}
							<Loader2 class="h-4 w-4 animate-spin" />
						{:else}
							<Database class="h-4 w-4" />
						{/if}
						Initialize database
					</Button>
				</div>
				<p class="text-xs text-muted-foreground">
					Need multiple datastores? Provision as many as you need — credentials stay encrypted in
					Bakery.
				</p>
			</section>

			<section class="space-y-4 rounded-2xl border bg-card p-6 shadow-sm">
				<header class="space-y-2">
					<h3 class="text-lg font-semibold">Connection details</h3>
					<p class="text-sm text-muted-foreground">
						Copy the Postgres URL or run migrations with the ready-to-use command snippet.
					</p>
					{#if copyFeedback}
						<p class="text-xs text-emerald-600">{copyFeedback}</p>
					{/if}
				</header>

				{#if databases.length === 0}
					<p class="text-sm text-muted-foreground">
						No databases yet. Use “Initialize database” above to create one instantly.
					</p>
				{:else}
					<div class="space-y-4">
						{#each databases as db (db.id)}
							<article class="space-y-4 rounded-xl border bg-background/60 p-4">
								<div class="flex flex-wrap items-center justify-between gap-3">
									<div>
										<p class="font-semibold">{db.name}</p>
										<p class="text-xs text-muted-foreground capitalize">Status {db.status}</p>
									</div>
									<p class="text-xs text-muted-foreground">
										Created {formatDate(db.created_at)}
									</p>
								</div>
								<div class="space-y-2">
									<p class="text-[11px] tracking-wide text-muted-foreground uppercase">
										Postgres URL
									</p>
									<div class="flex flex-col gap-2 lg:flex-row">
										<Input value={db.connection_url} readonly class="font-mono text-xs" />
										<Button
											variant="outline"
											size="icon"
											class="shrink-0"
											onclick={() => copyToClipboard(db.connection_url, `${db.name} URL`)}
										>
											<Clipboard class="h-4 w-4" />
										</Button>
									</div>
								</div>
								<div class="space-y-2">
									<p class="text-[11px] tracking-wide text-muted-foreground uppercase">
										Run migrations
									</p>
									<pre class="overflow-x-auto rounded-lg bg-muted/40 p-3 font-mono text-xs">
DATABASE_URL="{db.connection_url}" {MIGRATION_COMMAND}</pre>
									<Button
										variant="ghost"
										class="w-fit gap-2 text-xs"
										onclick={() =>
											copyToClipboard(
												`DATABASE_URL="${db.connection_url}" ${MIGRATION_COMMAND}`,
												'migration command'
											)}
									>
										<Clipboard class="h-3.5 w-3.5" />
										Copy command
									</Button>
								</div>
							</article>
						{/each}
					</div>
				{/if}
			</section>
		</Tabs.Content>

		<Tabs.Content value="environment" class="rounded-2xl border bg-card p-6 shadow-sm">
			<div class="space-y-6">
				<header class="space-y-2">
					<h2 class="text-xl font-semibold">Environment variables</h2>
					<p class="text-sm text-muted-foreground">
						Secrets are stored encrypted in PostgreSQL. Textareas make it easier to paste multiline
						values like certificates.
					</p>
				</header>

				<div class="grid gap-3 md:grid-cols-[240px,1fr]">
					<Textarea
						bind:value={envKey}
						class="min-h-12"
						rows="2"
						placeholder="KEY (e.g. DATABASE_URL)"
					/>
					<div class="space-y-3">
						<Textarea bind:value={envValue} class="min-h-24" rows="4" placeholder="Secret value" />
						<Button type="button" onclick={addEnvVar} disabled={working}>Save variable</Button>
					</div>
				</div>

				{#if environment.length === 0}
					<p class="text-sm text-muted-foreground">No environment variables yet.</p>
				{:else}
					<ul class="space-y-3">
						{#each environment as item (item.key)}
							<li class="space-y-3 rounded-lg border bg-background/60 p-4">
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
									<div class="space-y-3">
										<Textarea
											class="min-h-20"
											rows="4"
											placeholder="New value"
											bind:value={editingEnv[item.key]}
										/>
										<div class="flex gap-2">
											<Button
												class="h-10"
												onclick={() => updateExistingEnv(item.key)}
												disabled={working}
											>
												Update
											</Button>
											<Button
												variant="outline"
												class="h-10"
												onclick={() => (editingEnv = { ...editingEnv, [item.key]: undefined })}
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
			</div>
		</Tabs.Content>

		<Tabs.Content value="danger" class="space-y-6 rounded-2xl border bg-card p-6 shadow-sm">
			<header class="space-y-1">
				<h2 class="text-xl font-semibold text-destructive">Danger zone</h2>
				<p class="text-sm text-muted-foreground">
					Destructive actions live here. Rollbacks immediately swap slots, and deletion removes
					logs, builds, and domains.
				</p>
			</header>

			<section class="space-y-4 rounded-2xl border bg-background/40 p-4">
				<h3 class="text-lg font-semibold">Rollback to a previous release</h3>
				{#if visibleVersions.length === 0}
					<p class="text-sm text-muted-foreground">No historical versions captured yet.</p>
				{:else}
					<ul class="space-y-3">
						{#each visibleVersions as version (version.id)}
							<li class="rounded-lg border bg-card/60 p-4">
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

			<section class="rounded-2xl border border-destructive/40 bg-destructive/5 p-4">
				<div class="flex items-start gap-3">
					<Trash2 class="mt-0.5 h-5 w-5 text-destructive" />
					<div class="flex-1 space-y-2">
						<h3 class="text-lg font-semibold text-destructive">Delete deployment</h3>
						<p class="text-sm text-muted-foreground">
							This permanently removes the deployment, its build artifacts, logs, and domains. This
							cannot be undone.
						</p>
						<AlertDialog.Root
							open={deleteDialogOpen}
							onOpenChange={(event) => (deleteDialogOpen = event.detail)}
						>
							<AlertDialog.Trigger>
								{#snippet child({ props })}
									<Button
										{...props}
										variant="destructive"
										class="mt-2"
										onclick={(event) => {
											props.onClick?.(event);
											deleteDialogOpen = true;
										}}
									>
										Delete deployment
									</Button>
								{/snippet}
							</AlertDialog.Trigger>
							<AlertDialog.Content>
								<AlertDialog.Header>
									<AlertDialog.Title>Delete {deployment.name}?</AlertDialog.Title>
									<AlertDialog.Description>
										This action cannot be undone and will detach all slots, databases, and logs tied
										to this deployment.
									</AlertDialog.Description>
								</AlertDialog.Header>
								<AlertDialog.Footer>
									<AlertDialog.Cancel onclick={() => (deleteDialogOpen = false)}>
										Cancel
									</AlertDialog.Cancel>
									<AlertDialog.Action
										onclick={handleDeleteDeployment}
										disabled={deletingDeployment}
									>
										{#if deletingDeployment}
											<Loader2 class="mr-2 h-4 w-4 animate-spin" />
										{/if}
										Delete
									</AlertDialog.Action>
								</AlertDialog.Footer>
							</AlertDialog.Content>
						</AlertDialog.Root>
					</div>
				</div>
			</section>
		</Tabs.Content>
	</Tabs.Root>
</section>
