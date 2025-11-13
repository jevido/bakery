<script>
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';

	import { goto } from '$app/navigation';
	import { apiFetch, createDeployment, fetchGithubBranches } from '$lib/api.js';
	import { isLocalHostname } from '$lib/shared/domainRules.js';
	import { Plus, X, Loader2 } from '@lucide/svelte';

	let { data } = $props();
	let repositories = $derived(data.repositories ?? []);
	let nodes = $derived(data.nodes ?? []);

	let name = $state('');
	let repository = $state('');
	let branch = $state('');
	let branchOptions = $state([]);
	let fetchingBranches = $state(false);

	let nodeId = $state('');

	let domains = $state([]);
	let newDomain = $state('');

	let envText = $state('');

	let enableBlueGreen = $state(true);
	let createDatabaseFlag = $state(false);
	let submitting = $state(false);
	let error = $state('');
	const LOCAL_DOMAIN_MESSAGE =
		'Local-only hostnames (like *.local, *.localhost, or private IPs) are disabled for now. We will reintroduce local overrides in a future release.';

	$effect(() => {
		if (!repository) {
			branch = '';
			branchOptions = [];
		}
	});

	$effect(() => {
		if (!nodeId && nodes.length > 0) {
			const activeNode = nodes.find((item) => item.status === 'active');
			nodeId = activeNode ? activeNode.id : '';
		}
	});

	async function hydrateBranches(selectedRepo) {
		if (!selectedRepo) return;
		fetchingBranches = true;
		try {
			const payload = await fetchGithubBranches(selectedRepo);
			branchOptions = payload.branches || [];
			if (!branch && branchOptions.length > 0) {
				branch = branchOptions[0].name;
			}
		} catch (err) {
			error = err?.message || 'Unable to load branches.';
			branchOptions = [];
		} finally {
			fetchingBranches = false;
		}
	}

	function addDomain() {
		const trimmed = newDomain.trim();
		if (!trimmed) return;
		if (isLocalHostname(trimmed)) {
			error = LOCAL_DOMAIN_MESSAGE;
			return;
		}
		const normalized = trimmed.toLowerCase();
		const alreadyAdded = domains.some((hostname) => hostname.toLowerCase() === normalized);
		if (alreadyAdded) {
			newDomain = '';
			return;
		}
		error = '';
		domains = [...domains, trimmed];
		newDomain = '';
	}

	function removeDomain(index) {
		domains = domains.filter((_, idx) => idx !== index);
	}
	
	function sanitizeDomains(list) {
		const seen = new Set();
		const sanitized = [];
		for (const hostname of list) {
			const trimmed = hostname?.trim();
			if (!trimmed) continue;
			const normalized = trimmed.toLowerCase();
			if (seen.has(normalized)) continue;
			seen.add(normalized);
			sanitized.push(trimmed);
		}
		return sanitized;
	}

	async function handleSubmit(event) {
		event.preventDefault();
		error = '';

		if (!name || !repository || !branch) {
			error = 'Name, repository, and branch are required.';
			return;
		}

		const environment = envText
			.split('\n')
			.map((line) => line.trim())
			.filter(Boolean)
			.reduce((acc, line) => {
				const eq = line.indexOf('=');
				if (eq === -1) {
					return acc;
				}
				const key = line.slice(0, eq).trim();
				if (!key) {
					return acc;
				}
				acc[key] = line.slice(eq + 1);
				return acc;
			}, {});

		const sanitizedDomains = sanitizeDomains(domains);
		domains = sanitizedDomains;

		submitting = true;
		try {
			const payload = await createDeployment({
				name,
				repository,
				branch,
				domains: sanitizedDomains,
				environment,
				enableBlueGreen,
				createDatabase: createDatabaseFlag,
				nodeId: nodeId || null
			});
			const deploymentId = payload.deployment?.id;
			if (deploymentId) {
				await goto(`/deployments/${deploymentId}`);
			} else {
				await goto('/deployments');
			}
		} catch (err) {
			error = err?.message || 'Deployment failed to start.';
		} finally {
			submitting = false;
		}
	}

	async function refreshRepositories() {
		try {
			const payload = await apiFetch('/api/github/repos');
			repositories = payload.repositories || [];
		} catch (err) {
			error = err?.message || 'Unable to refresh repositories.';
		}
	}
</script>

<svelte:head>
	<title>New deployment ~ The Bakery</title>
</svelte:head>

<section class="space-y-6 p-6 md:p-10">
	<header class="flex flex-col gap-4">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">New deployment</h1>
			<p class="text-sm text-muted-foreground">
				Connect a repository, configure runtime options, and roll out with optional blue-green
				slots.
			</p>
		</div>
		{#if error}
			<div
				class="flex items-start gap-2 rounded-xl border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive"
			>
				<X class="mt-0.5 h-4 w-4" />
				<p>{error}</p>
			</div>
		{/if}
	</header>

	<form class="grid gap-10 lg:grid-cols-[2fr,1fr]" onsubmit={handleSubmit}>
		<section class="space-y-6 rounded-2xl border bg-card p-6 shadow-sm">
			<div class="space-y-1.5">
				<label for="name" class="text-sm font-medium">Deployment name</label>
				<input
					id="name"
					class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
					placeholder="monorepo-api"
					bind:value={name}
					required
				/>
			</div>

			<div class="space-y-1.5">
				<div class="flex items-center justify-between">
					<label for="repository" class="text-sm font-medium">GitHub repository</label>
					<Button variant="link" class="h-auto p-0 text-xs" onclick={refreshRepositories}>
						Refresh list
					</Button>
				</div>
				<select
					id="repository"
					class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
					bind:value={repository}
					required
					onchange={(event) => hydrateBranches(event.currentTarget.value)}
				>
					<option value="" disabled selected>Select a repository</option>
					{#each repositories as repo (repo.id)}
						<option value={repo.full_name}>
							{repo.full_name}
							{repo.private ? '(private)' : ''}
						</option>
					{/each}
				</select>
				{#if repositories.length === 0}
					<p class="text-xs text-muted-foreground">
						No repositories detected. Link GitHub from the dashboard to populate this list.
					</p>
				{/if}
			</div>

			<div class="space-y-1.5">
				<label for="branch" class="text-sm font-medium">Branch</label>
				<div class="relative">
					<select
						id="branch"
						class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20 disabled:cursor-not-allowed disabled:opacity-60"
						bind:value={branch}
						required
						disabled={!repository || fetchingBranches}
					>
						<option value="" disabled selected>
							{fetchingBranches ? 'Loading branches...' : 'Select a branch'}
						</option>
						{#each branchOptions as option (option.name)}
							<option value={option.name}>{option.name}</option>
						{/each}
					</select>
					{#if fetchingBranches}
						<Loader2
							class="absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 animate-spin text-muted-foreground"
						/>
					{/if}
				</div>
			</div>

			<div class="space-y-1.5">
				<label for="node" class="text-sm font-medium">Server node</label>
				<select
					id="node"
					class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
					bind:value={nodeId}
				>
					<option value="">Control plane (this server)</option>
					{#each nodes as node (node.id)}
						<option value={node.id} disabled={node.status !== 'active'}>
							{node.name}
							{node.status !== 'active' ? ` (${node.status.replace(/_/g, ' ')})` : ''}
						</option>
					{/each}
				</select>
				<p class="text-xs text-muted-foreground">
					Deployments will run on the selected node. Choose the control plane to keep builds on this
					server.
				</p>
				{#if nodes.length === 0}
					<p class="text-xs text-muted-foreground">
						No external nodes detected. Visit the Servers section to register additional hosts.
					</p>
				{/if}
			</div>

			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<p class="text-sm font-medium">Domains</p>
					<Button variant="link" class="h-auto gap-1 p-0 text-sm" onclick={addDomain}>
						<Plus class="h-3.5 w-3.5" />
						Add domain
					</Button>
				</div>
				<div class="flex gap-3">
					<input
						class="h-11 flex-1 rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
						placeholder="app.example.com"
						bind:value={newDomain}
						onkeydown={(event) => {
							if (event.key === 'Enter') {
								event.preventDefault();
								addDomain();
							}
						}}
					/>
				</div>
				{#if domains.length > 0}
					<ul class="space-y-2">
						{#each domains as domain, index (domain + index)}
							<li
								class="flex items-center justify-between rounded-lg border border-border/60 bg-background/60 px-3 py-2 text-sm"
							>
								<span>{domain}</span>
								<button
									type="button"
									class="rounded-md border border-border/70 p-1 text-muted-foreground hover:text-destructive"
									onclick={() => removeDomain(index)}
									aria-label={`Remove ${domain}`}
								>
									<X class="h-3 w-3" />
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</div>

			<div class="space-y-2">
				<p class="text-sm font-medium">Environment variables</p>
				<Textarea
					class="h-40 w-full rounded-lg border border-input bg-background px-3 py-2 font-mono text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
					placeholder={`KEY=value\nANOTHER=value`}
					bind:value={envText}
				/>
				<p class="text-xs text-muted-foreground">
					Enter one <code class="rounded bg-muted px-1.5 py-0.5 text-[11px]">KEY=value</code>
					pair per line. Lines without an equals sign are ignored.
				</p>
			</div>
		</section>

		<aside class="space-y-6 rounded-2xl border bg-card p-6 shadow-sm">
			<div class="space-y-3">
				<label class="flex items-start gap-3 text-sm font-medium">
					<input
						type="checkbox"
						class="mt-1 h-4 w-4 rounded border border-input accent-primary"
						bind:checked={enableBlueGreen}
					/>
					<span>
						Blue-green deployments
						<p class="text-xs font-normal text-muted-foreground">
							Deploy new versions to an idle slot, verify them, then switch traffic with zero
							downtime.
						</p>
					</span>
				</label>
			</div>

			<div class="space-y-3">
				<label class="flex items-start gap-3 text-sm font-medium">
					<input
						type="checkbox"
						class="mt-1 h-4 w-4 rounded border border-input accent-primary"
						bind:checked={createDatabaseFlag}
					/>
					<span>
						Provision PostgreSQL database
						<p class="text-xs font-normal text-muted-foreground">
							Bakery will create a dedicated Postgres database and inject the connection string as
							<code class="rounded bg-muted px-1.5 py-0.5 text-[11px]">DATABASE_URL</code>
							.
						</p>
					</span>
				</label>
			</div>

			<div
				class="rounded-xl border border-primary/30 bg-primary/5 p-4 text-xs text-muted-foreground"
			>
				<p class="font-medium text-foreground">What happens next?</p>
				<ul class="mt-2 list-disc space-y-1 pl-4">
					<li>Bakery clones your repo and detects Docker or Bun runtimes automatically.</li>
					<li>
						Build output is stored under <code>/var/lib/bakery/builds</code>
						and wired into Nginx.
					</li>
					<li>Certificates are issued with Certbot when you verify domains.</li>
				</ul>
			</div>

			<Button class="w-full justify-center gap-2" type="submit" disabled={submitting}>
				{#if submitting}
					<Loader2 class="h-4 w-4 animate-spin" />
					Creating deployment…
				{:else}
					Start deployment
				{/if}
			</Button>
			<Button
				variant="outline"
				class="w-full justify-center"
				type="button"
				onclick={() => goto('/deployments')}
			>
				Cancel
			</Button>
		</aside>
	</form>
</section>
