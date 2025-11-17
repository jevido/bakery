<script>
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { Button } from '$lib/components/ui/button';

	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { cn } from '$lib/utils.js';
	import {
		Menu,
		LayoutDashboard,
		Box,
		Server,
		Database,
		Users,
		Cog,
		SunMedium,
		Moon,
		PackagePlus,
		RefreshCw
	} from '@lucide/svelte';
	import { mode, ModeWatcher, toggleMode } from 'mode-watcher';
	import Separator from '$lib/components/ui/separator/separator.svelte';
	import { Toaster } from '$lib/components/ui/sonner/index.js';

	let { children, data } = $props();

	let user = $derived(data?.user);
	let currentPath = $derived(page.url.pathname);
	let isSidebarOpen = $state(false);

	const allNavItems = [
		{ label: 'Dashboard', href: '/', icon: LayoutDashboard },
		{ label: 'Deployments', href: '/deployments', icon: Box },
		{ label: 'Servers', href: '/nodes', icon: Server },
		{ label: 'Databases', href: '/databases', icon: Database },
		{ label: 'System', href: '/system', icon: Cog }
	];

	const bakeryNavItems = [
		{ label: 'Users', href: '/users', icon: Users, requiresAdmin: true },
		{ label: 'Self-update', href: '/system/updates', icon: RefreshCw, requiresAdmin: true }
	];

	let navItems = $derived(allNavItems.filter((item) => !item.requiresAdmin || user?.is_admin));
	let adminNavItems = $derived(
		bakeryNavItems.filter((item) => !item.requiresAdmin || user?.is_admin)
	);
	let githubLinked = $derived(Boolean(user?.github_connected));

	function toggleSidebar() {
		isSidebarOpen = !isSidebarOpen;
	}

	async function handleLogout() {
		await goto('/logout');
	}

	async function startGithubLink() {
		try {
			const response = await fetch('/api/auth/github/url', { credentials: 'include' });
			if (response.ok) {
				const payload = await response.json();
				if (payload.url) {
					window.location.href = payload.url;
				}
			}
		} catch {
			// ignore
		}
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Bakery App Deployment</title>
</svelte:head>

<ModeWatcher />
<Toaster />

{#if user}
	<div class="min-h-screen bg-background text-foreground">
		<div class="flex min-h-screen">
			<button
				class="absolute top-4 left-4 flex h-10 w-10 items-center justify-center rounded-lg border bg-background shadow md:hidden"
				type="button"
				onclick={toggleSidebar}
				aria-label="Toggle navigation"
			>
				<Menu class="h-5 w-5" />
			</button>

			<aside
				class={`fixed inset-y-0 z-40 w-72 transform border-r bg-sidebar px-6 py-8 shadow-lg transition-transform duration-200 md:static md:translate-x-0 ${
					isSidebarOpen ? 'translate-x-0' : '-translate-x-full'
				}`}
			>
				<div class="flex items-center justify-between">
					<div>
						<p class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
							Bakery
						</p>
						<p class="text-lg font-bold">Control plane</p>
					</div>
					<Button variant="ghost" size="icon" onclick={toggleMode} aria-label="Toggle theme">
						{#if mode.current === 'dark'}
							<Moon class="h-4 w-4" />
						{:else}
							<SunMedium class="h-4 w-4" />
						{/if}
					</Button>
				</div>

				<nav class="mt-10 space-y-2">
					{#each navItems as item (item.href)}
						{@render menuItem(item)}
					{/each}

					<Separator class="my-4" />

					{#each adminNavItems as item (item.href)}
						{@render menuItem(item)}
					{/each}
				</nav>

				<div class="mt-10 rounded-lg border bg-background/70 p-4">
					<p class="text-xs tracking-wide text-muted-foreground uppercase">Getting started</p>
					{#if githubLinked}
						<p class="mt-2 text-sm text-muted-foreground">
							GitHub is connected. Launch a deployment to bake your first app.
						</p>
						<Button class="mt-4 w-full" onclick={() => goto('/deployments/new')}>
							New deployment
						</Button>
					{:else}
						<p class="mt-2 text-sm text-muted-foreground">
							Link GitHub to unlock repository-driven deployments, configure Nginx, and request SSL
							in a few clicks.
						</p>
						<Button class="mt-4 w-full" variant="outline" onclick={startGithubLink}>
							Link GitHub
						</Button>
					{/if}
				</div>
			</aside>

			<div class="flex flex-1 flex-col">
				<header
					class="sticky top-0 z-30 flex flex-wrap items-center justify-between gap-4 border-b bg-background/95 px-6 py-4 backdrop-blur supports-backdrop-filter:bg-background/60"
				>
					<div>
						<h1 class="text-xl font-semibold">Bakery</h1>
						<p class="text-sm text-muted-foreground">Bake your projects to perfection.</p>
					</div>
					<div class="flex items-center gap-3">
						<div class="hidden text-right text-sm md:block">
							<p class="font-medium">{user.email}</p>
							<p class="text-xs text-muted-foreground">
								{user.is_admin ? 'Administrator' : 'User'}
							</p>
						</div>
						<Button variant="outline" class="hidden md:flex" href="/system">View activity</Button>
						<Button class="gap-2" href="/deployments/new">
							New deployment
							<PackagePlus class="h-4 w-4" />
						</Button>
						<Button variant="ghost" onclick={handleLogout}>Logout</Button>
					</div>
				</header>

				<main class="flex-1 overflow-y-auto bg-background">
					<svelte:boundary onerror={(e) => console.error(e)}>
						{@render children()}
						{#snippet failed(error, reset)}
							<p>Oops! {error.message}</p>
							<button onclick={reset}>Reset</button>
						{/snippet}
					</svelte:boundary>
				</main>
			</div>
		</div>
	</div>
{:else}
	<div class="min-h-screen bg-background text-foreground">
		{@render children()}
	</div>
{/if}

{#snippet menuItem(item)}
	{@const Icon = item.icon}
	{@const active =
		currentPath === item.href ||
		(currentPath.startsWith(item.href) && item.href !== '/') ||
		(item.href === '/' && currentPath === '/')}

	<Button
		href={item.href}
		variant="link"
		class={cn(
			'flex items-center justify-start gap-3 rounded-lg px-4 py-2 text-sm font-medium transition',
			active
				? 'bg-secondary text-foreground shadow-sm'
				: 'text-muted-foreground hover:bg-secondary hover:text-foreground'
		)}
		aria-current={active ? 'page' : undefined}
		onclick={() => {
			isSidebarOpen = false;
		}}
	>
		<Icon class="h-4 w-4" />
		<span>{item.label}</span>
	</Button>
{/snippet}
