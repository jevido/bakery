<script>
	import { Button } from "$lib/components/ui/button"
	import { onMount } from 'svelte';
	import { Activity, HardDrive, RefreshCw, Server, CheckCircle2, AlertTriangle, Loader2 } from '@lucide/svelte';

	let { data } = $props();
	let analytics = $derived(data.analytics ?? {});
	let health = $state(data.health ?? null);
	let refreshing = $state(false);
	let refreshError = $state(null);

	let disk = $derived(analytics.disk ?? null);
	let systemDisk = $derived(analytics.systemDisk ?? null);
	let tasks = $derived(analytics.tasks ?? []);

	function formatBytes(bytes) {
		const value = Number(bytes ?? 0);
		if (!Number.isFinite(value) || value <= 0) return '—';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let index = 0;
		let size = value;
		while (size >= 1024 && index < units.length - 1) {
			size /= 1024;
			index += 1;
		}
		return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
	}

	function formatTime(value) {
		if (!value) return '—';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return '—';
		return date.toLocaleString();
	}

	async function refreshHealth() {
		refreshing = true;
		refreshError = null;
		try {
			const response = await fetch('/api/system/health', { credentials: 'include' });
			if (!response.ok) {
				throw new Error('Health check failed');
			}
			health = await response.json();
		} catch (err) {
			refreshError = err?.message ?? 'Failed to refresh system health';
		} finally {
			refreshing = false;
		}
	}

	onMount(() => {
		if (!health) {
			refreshHealth();
		}
	});
</script>

<section class="space-y-8 p-6 md:p-10">
	<header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">System overview</h1>
			<p class="text-sm text-muted-foreground">
				Monitor Bakery background services, task queue activity, and resource usage.
			</p>
		</div>
		<Button class="gap-2" onclick={refreshHealth} disabled={refreshing}>
			{#if refreshing}
				<Loader2 class="h-4 w-4 animate-spin" />
				Checking...
			{:else}
				<RefreshCw class="h-4 w-4" />
				Run health check
			{/if}
		</Button>
	</header>

	{#if refreshError}
		<div class="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/30 dark:bg-rose-900/30 dark:text-rose-200">
			{refreshError}
		</div>
	{/if}

	<section class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
		<div class="rounded-xl border bg-card p-5 shadow-sm">
			<header class="mb-4 flex items-center gap-3">
				<div class="flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
					<Server class="h-5 w-5 text-secondary-foreground" />
				</div>
				<div>
					<p class="text-sm font-semibold">API status</p>
					<p class="text-xs text-muted-foreground">SvelteKit + Bun worker</p>
				</div>
			</header>
			{#if health}
				<div class="flex items-center gap-2 text-sm">
					{#if health.status === 'ok'}
						<CheckCircle2 class="h-4 w-4 text-emerald-500" />
						<span class="text-emerald-600 dark:text-emerald-300">All systems operational</span>
					{:else}
						<AlertTriangle class="h-4 w-4 text-amber-500" />
						<span class="text-amber-600 dark:text-amber-200">Degraded response</span>
					{/if}
				</div>
				<p class="mt-2 text-xs text-muted-foreground">Last checked {formatTime(health.time)}</p>
			{:else}
				<p class="text-sm text-muted-foreground">Pending health check…</p>
			{/if}
		</div>

		<div class="rounded-xl border bg-card p-5 shadow-sm">
			<header class="mb-4 flex items-center gap-3">
				<div class="flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
					<Activity class="h-5 w-5 text-secondary-foreground" />
				</div>
				<div>
					<p class="text-sm font-semibold">Task queue</p>
					<p class="text-xs text-muted-foreground">Recent job executions</p>
				</div>
			</header>
			{#if tasks.length === 0}
				<p class="text-sm text-muted-foreground">No tasks have run recently.</p>
			{:else}
				<ul class="space-y-3 text-sm">
					{#each tasks as task (task.id)}
						<li class="rounded-lg border bg-background/70 px-3 py-2">
							<div class="flex items-center justify-between">
								<span class="font-medium capitalize">{task.type}</span>
								<span class="text-xs text-muted-foreground">{formatTime(task.created_at)}</span>
							</div>
							<p class="text-xs text-muted-foreground">Status: {task.status}</p>
							{#if task.error}
								<p class="mt-1 text-xs text-rose-500">{task.error}</p>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<div class="rounded-xl border bg-card p-5 shadow-sm">
			<header class="mb-4 flex items-center gap-3">
				<div class="flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
					<HardDrive class="h-5 w-5 text-secondary-foreground" />
				</div>
				<div>
					<p class="text-sm font-semibold">Disk usage</p>
					<p class="text-xs text-muted-foreground">Build artifacts & system volume</p>
				</div>
			</header>
			<ul class="space-y-2 text-sm">
				<li class="flex items-center justify-between">
					<span>Bakery data dir</span>
					<span class="text-muted-foreground">
						{disk ? `${formatBytes(disk.used)} used / ${formatBytes(disk.total)} total` : '—'}
					</span>
				</li>
				<li class="flex items-center justify-between">
					<span>System disk</span>
					<span class="text-muted-foreground">
						{systemDisk
							? `${formatBytes(systemDisk.used)} used / ${formatBytes(systemDisk.total)} total`
							: '—'}
					</span>
				</li>
			</ul>
		</div>
	</section>
</section>
