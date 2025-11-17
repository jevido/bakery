<script>
	import { goto } from '$app/navigation';

	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Label } from '$lib/components/ui/label';

	import { apiFetch, createDeployment, fetchGithubBranches } from '$lib/api.js';
	import { isLocalHostname } from '$lib/shared/domainRules.js';
	import {
		Plus,
		X,
		Loader2,
		ServerCog,
		Cloud,
		Code2,
		Layers,
		Terminal,
		Shield,
		Loader2Icon
	} from '@lucide/svelte';
	import * as NativeSelect from '$lib/components/ui/native-select/';
	import { toast } from 'svelte-sonner';

	let { data } = $props();
	let repositories = $derived(data.repositories ?? []);
	let nodes = $derived(data.nodes ?? []);

	let name = $state('');
	let repository = $state('');
	let branch = $state('');
	let branchOptions = $state([]);
	let fetchingBranches = $state(false);

	let isFetchingRepositories = $state(false);

	const initialNodeCandidates = data.nodes ?? [];
	const initialNodeId =
		(initialNodeCandidates.find((item) => item.status === 'active') ?? initialNodeCandidates[0])
			?.id ?? '';
	let nodeId = $state(initialNodeId);

	let domains = $state([]);
	let newDomain = $state('');
	let envInput = $state('');
	let dockerfilePath = $state('Dockerfile');
	let buildContext = $state('.');

	let enableBlueGreen = $state(true);
	let createDatabaseFlag = $state(false);
	let submitting = $state(false);
	let error = $state('');

	const LOCAL_DOMAIN_MESSAGE =
		'Local-only hostnames (like *.local, *.localhost, or private IPs) are disabled for now. We will reintroduce local overrides in a future release.';

	const pipelineStages = [
		{
			title: 'Pull & build',
			description: 'Clone repo + branch on the node, install deps, and build the container.',
			icon: Code2
		},
		{
			title: 'Proxy & TLS',
			description: 'Generate Nginx + Certbot config for every attached domain.',
			icon: Shield
		},
		{
			title: 'Blue/green',
			description: 'Deploy to the idle slot, validate health, then flip traffic.',
			icon: Layers
		},
		{
			title: 'Streaming logs',
			description: 'Follow build + runtime logs live in the dashboard.',
			icon: Terminal
		}
	];

	function handleRepositoryChange(value) {
		repository = value;
		branch = '';
		branchOptions = [];
		if (!value) return;
		hydrateBranches(value);
	}

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
			const message = err?.message || 'Unable to load branches.';
			error = message;
			toast.error(message);
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
			toast.warning(error);
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

	function parseEnvInput(input) {
		const env = {};
		if (!input) return env;
		const lines = input.split('\n');
		for (const rawLine of lines) {
			const line = rawLine.trim();
			if (!line || line.startsWith('#')) continue;
			const [key, ...rest] = line.split('=');
			const normalizedKey = key.trim();
			if (!normalizedKey) continue;
			const value = rest.length > 0 ? rest.join('=').trim() : '';
			env[normalizedKey] = value;
		}
		return env;
	}

	async function handleSubmit(event) {
		event.preventDefault();
		error = '';

		if (!name || !repository || !branch) {
			error = 'Name, repository, and branch are required.';
			toast.warning(error);
			return;
		}

		const environment = parseEnvInput(envInput);
		let trimmedDockerfile = dockerfilePath.trim();
		let trimmedContext = buildContext.trim();
		if (!trimmedDockerfile) {
			trimmedDockerfile = 'Dockerfile';
		}
		if (!trimmedContext) {
			trimmedContext = '.';
		}

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
				nodeId: nodeId || null,
				dockerfilePath: trimmedDockerfile,
				buildContext: trimmedContext
			});
			const deploymentId = payload.deployment?.id;
			if (deploymentId) {
				await goto(`/deployments/${deploymentId}`);
			} else {
				await goto('/deployments');
			}
		} catch (err) {
			const message = err?.message || 'Deployment failed to start.';
			error = message;
			toast.error(message);
		} finally {
			submitting = false;
		}
	}

	async function refreshRepositories() {
		try {
			isFetchingRepositories = true;
			const payload = await apiFetch('/api/github/repos');
			repositories = payload.repositories || [];
		} catch (err) {
			const message = err?.message || 'Unable to refresh repositories.';
			error = message;
			toast.error(message);
			isFetchingRepositories = false;
		}
		isFetchingRepositories = false;
	}
</script>

<svelte:head>
	<title>New SSH deployment ~ The Bakery</title>
</svelte:head>

<section class="space-y-5 p-4 md:p-8">
	{#if error}
		<div
			class="flex items-start gap-2 rounded-xl border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive"
		>
			<X class="mt-0.5 h-4 w-4" />
			<p>{error}</p>
		</div>
	{/if}

	<form class="space-y-4" onsubmit={handleSubmit}>
		<div class="grid gap-4 lg:grid-cols-3">
			<section class="relative space-y-3 rounded-2xl border bg-card p-4 shadow-sm">
				<div>
					<p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
						Source of truth
					</p>
					<h3 class="text-lg font-semibold">Repository & branch</h3>

					<p class="text-xs text-muted-foreground">
						Pick the repo + branch and toggle database provisioning.
					</p>
				</div>
				<Button
					variant="secondary"
					size="sm"
					class="absolute top-4 right-4 text-xs"
					onclick={refreshRepositories}
					disabled={isFetchingRepositories}
				>
					{#if !isFetchingRepositories}
						<Cloud class="mr-2 h-4 w-4" /> Refresh
					{:else}
						<Loader2Icon class="animate-spin" />
						Please wait
					{/if}
				</Button>

				<div class="grid grid-cols-2 gap-4">
					<Label for="repository" class="text-sm font-medium">Repository</Label>

					<Label for="branch">Branch</Label>
					{#if fetchingBranches}
						<span class="flex items-center gap-1 text-xs text-muted-foreground">
							<Loader2 class="h-3.5 w-3.5 animate-spin" /> Loading branches
						</span>
					{/if}
					<NativeSelect.Root
						bind:value={repository}
						id="repository"
						class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
						onchange={(event) => handleRepositoryChange(event.currentTarget.value)}
						required
					>
						<NativeSelect.Option value="" disabled selected>Pick a repository</NativeSelect.Option>
						{#each repositories as repo (repo.full_name)}
							<NativeSelect.Option value={repo.full_name}>{repo.full_name}</NativeSelect.Option>
						{/each}
					</NativeSelect.Root>
					<NativeSelect.Root
						bind:value={branch}
						id="branch"
						class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
						disabled={!repository || fetchingBranches}
						required
					>
						<NativeSelect.Option value="" disabled selected>Select a branch</NativeSelect.Option>
						{#each branchOptions as option (option.name)}
							<NativeSelect.Option value={option.name}>{option.name}</NativeSelect.Option>
						{/each}
					</NativeSelect.Root>
				</div>

				<div class="grid gap-3 md:grid-cols-2">
					<label class="flex flex-col gap-1 text-sm font-medium">
						<span>Dockerfile path</span>
						<Input
							id="dockerfile-path"
							placeholder="apps/api/Dockerfile"
							bind:value={dockerfilePath}
						/>
						<span class="text-xs font-normal text-muted-foreground">
							Path relative to the repo root. Defaults to `Dockerfile`.
						</span>
					</label>
					<label class="flex flex-col gap-1 text-sm font-medium">
						<span>Build context</span>
						<Input id="build-context" placeholder="." bind:value={buildContext} />
						<span class="text-xs font-normal text-muted-foreground">
							Directory sent to docker build (ex: apps/api or .).
						</span>
					</label>
				</div>
				<p class="text-xs text-muted-foreground">
					Need a Laravel or Rails container? Set the path to the Dockerfile plus the folder you
					want as the docker build context.
				</p>
				<div class="flex items-center justify-between rounded-xl border border-dashed p-3 text-xs">
					<div>
						<p class="text-sm font-semibold">Provision database</p>
						<p class="text-muted-foreground">Creates a Postgres DB + user on deploy.</p>
					</div>
					<label class="inline-flex cursor-pointer items-center gap-2 text-sm font-medium">
						<input
							bind:checked={createDatabaseFlag}
							type="checkbox"
							class="h-4 w-4 rounded border-primary text-primary focus:ring-primary"
						/>
						<span>{createDatabaseFlag ? 'Enabled' : 'Optional'}</span>
					</label>
				</div>
			</section>

			<section class="space-y-3 rounded-2xl border bg-card p-4 shadow-sm">
				<div>
					<p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
						Environment
					</p>
					<h3 class="text-lg font-semibold">Env variables</h3>
					<p class="text-xs text-muted-foreground">Paste KEY=value pairs below.</p>
				</div>
				<Textarea
					bind:value={envInput}
					placeholder="KEY=value"
					class="min-h-40 font-mono text-xs"
				/>
				<p class="text-xs text-muted-foreground">Lines starting with # are ignored.</p>
			</section>
			<section class="space-y-2 rounded-2xl border bg-card p-4 shadow-sm">
				<div>
					<p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
						Push pipeline
					</p>
					<p class="text-xs text-muted-foreground">
						Four quick steps from git push to live traffic.
					</p>
				</div>
				<ol class="space-y-2 text-xs text-muted-foreground">
					{#each pipelineStages as stage (stage.title)}
						<li class="rounded-lg border bg-background/80 p-3">
							<div class="flex items-center gap-2 text-foreground">
								<stage.icon class="h-4 w-4 text-primary" />
								<p class="text-sm font-semibold">{stage.title}</p>
							</div>
							<p class="mt-1">{stage.description}</p>
						</li>
					{/each}
				</ol>
			</section>
		</div>

		<div class="grid gap-4 lg:grid-cols-2">
			<section class="space-y-3 rounded-2xl border bg-card p-4 shadow-sm">
				<div>
					<p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
						Runtime
					</p>
					<h3 class="text-lg font-semibold">Node & release strategy</h3>
					<p class="text-xs text-muted-foreground">
						Pick the SSH node and toggle blue/green flips.
					</p>
				</div>

				<div class="space-y-3">
					<Label for="target-node" class="text-sm font-medium">Target node</Label>
					<NativeSelect.Root
						bind:value={nodeId}
						id="target-node"
						class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
						required
					>
						<NativeSelect.Option value="" disabled selected>Select a server</NativeSelect.Option>
						{#each nodes as node (node.id)}
							<NativeSelect.Option value={node.id}>
								{node.name}
								{node.status === 'active' ? '· Active' : '· Pending'}
							</NativeSelect.Option>
						{/each}
					</NativeSelect.Root>
				</div>

				<div class="flex flex-col gap-3 rounded-xl border border-dashed p-4">
					<div class="flex items-center justify-between">
						<div>
							<p class="text-sm font-semibold">Blue/green releases</p>
							<p class="text-xs text-muted-foreground">
								Deploy idle slot first, flip when healthy.
							</p>
						</div>
						<label class="inline-flex cursor-pointer items-center gap-2 text-sm font-medium">
							<input
								bind:checked={enableBlueGreen}
								type="checkbox"
								class="h-4 w-4 rounded border-primary text-primary focus:ring-primary"
							/>
							<span>{enableBlueGreen ? 'Enabled' : 'Disabled'}</span>
						</label>
					</div>
				</div>
			</section>

			<section class="space-y-3 rounded-2xl border bg-card p-4 shadow-sm">
				<div>
					<p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
						Domains & TLS
					</p>
					<h3 class="text-lg font-semibold">Domains & TLS</h3>
					<p class="text-xs text-muted-foreground">Add hostnames to provision Nginx + Certbot.</p>
				</div>
				<div class="flex flex-col gap-3 md:flex-row">
					<Input
						placeholder="app.example.com"
						bind:value={newDomain}
						onkeydown={(event) => {
							if (event.key === 'Enter') {
								event.preventDefault();
								addDomain();
							}
						}}
					/>
					<Button type="button" class="gap-2" onclick={addDomain}>
						<Plus class="h-4 w-4" /> Add domain
					</Button>
				</div>
				{#if domains.length > 0}
					<ul class="grid gap-2 md:grid-cols-2">
						{#each domains as domain, index}
							<li
								class="flex items-center justify-between rounded-lg border bg-background px-3 py-2 text-sm"
							>
								<span>{domain}</span>
								<Button
									type="button"
									variant="ghost"
									size="icon"
									aria-label={`Remove ${domain}`}
									onclick={() => removeDomain(index)}
								>
									<X class="h-4 w-4" />
								</Button>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="text-xs text-muted-foreground">No domains yet. Add now or configure later.</p>
				{/if}
			</section>
		</div>

		<div class="grid gap-4 lg:grid-cols-[2fr,1fr]">
			<section class="space-y-3 rounded-2xl border bg-card p-4 shadow-sm">
				<div>
					<p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
						Deployment name
					</p>
					<Input bind:value={name} placeholder="monorepo-api" required />
				</div>
				<div class="space-y-1 text-xs text-muted-foreground">
					<p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
						Log streaming
					</p>
					<p>
						After you deploy, Bakery opens the live activity stream so you can tail logs in place.
					</p>
				</div>
				<Button class="w-full gap-2" type="submit" disabled={submitting}>
					{#if submitting}
						<Loader2 class="h-4 w-4 animate-spin" />
						Deploying…
					{:else}
						<ServerCog class="h-4 w-4" /> Deploy over SSH
					{/if}
				</Button>
			</section>

			<!-- <section class="rounded-2xl border bg-card p-4 text-xs text-muted-foreground shadow-sm">
				<p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
					Quick tips
				</p>
				<ul class="mt-2 space-y-1.5">
					<li>
						<span class="font-medium text-foreground">Artifacts:</span>
						last two releases stay on each node for instant rollbacks.
					</li>
					<li>
						<span class="font-medium text-foreground">Secrets:</span>
						env vars are encrypted at rest and replayed only for this deploy.
					</li>
					<li>
						<span class="font-medium text-foreground">Automation:</span>
						GitHub push webhooks can auto-trigger this pipeline.
					</li>
				</ul>
			</section> -->
		</div>
	</form>
</section>
