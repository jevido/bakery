<script>
	import { Button } from '$lib/components/ui/button';
	import { Copy, Database, Server, Check } from '@lucide/svelte';

	let { data } = $props();
	let databases = $derived(data.databases ?? []);
	let copiedId = $state(null);

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

	function formatDate(value) {
		if (!value) return '—';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return '—';
		return date.toLocaleString();
	}

	async function copyConnection(database) {
		if (typeof navigator === 'undefined') return;
		try {
			await navigator.clipboard.writeText(database.connection_url);
			copiedId = database.id;
			setTimeout(() => {
				if (copiedId === database.id) copiedId = null;
			}, 2000);
		} catch (error) {
			console.error('Failed to copy connection string', error);
		}
	}

	function statusVariant(status) {
		const normalized = (status || '').toLowerCase();
		if (['ready', 'available', 'active'].includes(normalized))
			return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200';
		if (['creating', 'provisioning', 'pending'].includes(normalized))
			return 'bg-amber-100 text-amber-800 dark:bg-amber-500/15 dark:text-amber-200';
		if (['failed', 'error'].includes(normalized))
			return 'bg-rose-100 text-rose-800 dark:bg-rose-500/15 dark:text-rose-200';
		return 'bg-secondary text-secondary-foreground';
	}
</script>

<svelte:head>
	<title>Databases ~ The Bakery</title>
</svelte:head>

<section class="space-y-6 p-6 md:p-10">
	<header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">Databases</h1>
			<p class="text-sm text-muted-foreground">
				Provisioned PostgreSQL instances bound to your deployments.
			</p>
		</div>
	</header>

	{#if databases.length === 0}
		<div
			class="flex flex-col items-center justify-center gap-4 rounded-2xl border border-dashed p-12 text-center"
		>
			<div class="flex h-12 w-12 items-center justify-center rounded-full bg-secondary">
				<Database class="h-6 w-6 text-secondary-foreground" />
			</div>
			<div class="space-y-1">
				<h2 class="text-lg font-semibold">No databases yet</h2>
				<p class="text-sm text-muted-foreground">
					Enable database provisioning when creating a deployment to see it listed here.
				</p>
			</div>
		</div>
	{:else}
		<div class="overflow-hidden rounded-2xl border">
			<table class="min-w-full divide-y divide-border bg-card">
				<thead class="bg-muted/50">
					<tr class="text-left text-xs font-medium tracking-wide text-muted-foreground uppercase">
						<th class="px-6 py-3">Database</th>
						<th class="px-6 py-3">Deployment</th>
						<th class="px-6 py-3">Status</th>
						<th class="px-6 py-3">Size</th>
						<th class="px-6 py-3">Updated</th>
						<th class="px-6 py-3"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-border text-sm">
					{#each databases as database (database.id)}
						<tr class="hover:bg-muted/30">
							<td class="px-6 py-4">
								<div class="font-medium text-foreground">{database.name}</div>
								<p class="text-xs break-all text-muted-foreground">{database.id}</p>
							</td>
							<td class="px-6 py-4">
								<div class="flex items-center gap-2 text-sm">
									<Server class="h-4 w-4 text-muted-foreground" />
									<span class="font-medium">{database.deployment_name}</span>
								</div>
								<p class="text-xs text-muted-foreground">
									{database.repository}
								</p>
							</td>
							<td class="px-6 py-4">
								<span
									class={`inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium ${statusVariant(database.status)}`}
								>
									{database.status}
								</span>
							</td>
							<td class="px-6 py-4 text-xs text-muted-foreground">
								{formatBytes(database.size_bytes)}
							</td>
							<td class="px-6 py-4 text-xs text-muted-foreground">
								{formatDate(database.updated_at)}
							</td>
							<td class="px-6 py-4 text-right">
								<Button variant="outline" class="gap-2" onclick={() => copyConnection(database)}>
									{#if copiedId === database.id}
										<Check class="h-4 w-4" />
										<span>Copied</span>
									{:else}
										<Copy class="h-4 w-4" />
										<span>Connection URL</span>
									{/if}
								</Button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
