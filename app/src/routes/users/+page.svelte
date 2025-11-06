<script>
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import { createUserAccount, updateUserAccount, deleteUserAccount } from '$lib/api.js';
	import { UserPlus, ShieldCheck, ShieldOff, Lock, Trash2 } from '@lucide/svelte';

	let { data } = $props();
	let users = $state(data.users ?? []);
	let form = $state({ email: '', password: '', confirm: '', isAdmin: false });
	let formError = $state('');
	let submitting = $state(false);
	let userMessage = $state('');
	let busyUserId = $state(null);

	let confirmDialog = $state({ open: false, user: null });
	let resetDialog = $state({ open: false, user: null, password: '' });
	let resetError = $state('');

	function resetForm() {
		form = { email: '', password: '', confirm: '', isAdmin: false };
		formError = '';
	}

	async function handleCreate(event) {
		event.preventDefault();
		formError = '';
		userMessage = '';
		const email = form.email.trim();
		if (!email) {
			formError = 'Email is required.';
			return;
		}
		if (form.password.length < 8) {
			formError = 'Password must be at least 8 characters.';
			return;
		}
		if (form.password !== form.confirm) {
			formError = 'Passwords do not match.';
			return;
		}
		submitting = true;
		try {
			const payload = await createUserAccount({
				email,
				password: form.password,
				isAdmin: form.isAdmin
			});
			if (payload?.user) {
				users = [payload.user, ...users.filter((user) => user.id !== payload.user.id)];
				userMessage = `User ${payload.user.email} created.`;
				resetForm();
			}
		} catch (err) {
			formError = err?.message ?? 'Failed to create user.';
		} finally {
			submitting = false;
		}
	}

	async function toggleAdmin(user) {
		busyUserId = user.id;
		userMessage = '';
		try {
			const payload = await updateUserAccount(user.id, { isAdmin: !user.is_admin });
			if (payload?.user) {
				users = users.map((row) => (row.id === user.id ? payload.user : row));
			}
		} catch (err) {
			userMessage = err?.message ?? 'Failed to update user.';
		} finally {
			busyUserId = null;
		}
	}

	function openResetDialog(user) {
		resetDialog = { open: true, user, password: '' };
		resetError = '';
	}

	async function confirmResetPassword() {
		if (!resetDialog.password || resetDialog.password.length < 8) {
			resetError = 'Password must be at least 8 characters.';
			return;
		}
		busyUserId = resetDialog.user.id;
		resetError = '';
		userMessage = '';
		try {
			await updateUserAccount(resetDialog.user.id, { password: resetDialog.password });
			userMessage = `Password for ${resetDialog.user.email} updated.`;
		} catch (err) {
			resetError = err?.message ?? 'Failed to reset password.';
		} finally {
			busyUserId = null;
			resetDialog = { open: false, user: null, password: '' };
		}
	}

	function openConfirmDialog(user) {
		confirmDialog = { open: true, user };
	}

	async function confirmDelete() {
		if (!confirmDialog.user) return;
		const user = confirmDialog.user;
		confirmDialog = { open: false, user: null };
		busyUserId = user.id;
		userMessage = '';
		try {
			await deleteUserAccount(user.id);
			users = users.filter((row) => row.id !== user.id);
			userMessage = `User ${user.email} removed.`;
		} catch (err) {
			userMessage = err?.message ?? 'Failed to delete user.';
		} finally {
			busyUserId = null;
		}
	}
</script>

<svelte:head>
	<title>Users ~ The Bakery</title>
</svelte:head>

<section class="space-y-8 p-6 md:p-10">
	<header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">User management</h1>
			<p class="text-sm text-muted-foreground">
				Invite teammates, promote administrators, and manage credentials.
			</p>
		</div>
	</header>

	<form class="rounded-xl border bg-card p-6 shadow-sm" onsubmit={handleCreate}>
		<header class="mb-6 flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
				<UserPlus class="h-5 w-5 text-secondary-foreground" />
			</div>
			<div>
				<h2 class="text-lg font-semibold">Create a new user</h2>
				<p class="text-sm text-muted-foreground">
					Set an email, password, and optional admin role.
				</p>
			</div>
		</header>
		<div class="grid gap-4 md:grid-cols-2">
			<div class="space-y-2">
				<Label for="new-email">Email</Label>
				<Input id="new-email" type="email" required autocomplete="off" bind:value={form.email} />
			</div>
			<div class="space-y-2">
				<Label for="new-password">Password</Label>
				<Input
					bind:value={form.password}
					id="new-password"
					type="password"
					required
					autocomplete="new-password"
					minlength={8}
				/>
			</div>
			<div class="space-y-2">
				<Label for="confirm-password">Confirm password</Label>
				<Input
					bind:value={form.confirm}
					id="confirm-password"
					type="password"
					required
					autocomplete="new-password"
					minlength={8}
				/>
			</div>
			<div class="flex items-center justify-between rounded-lg border bg-background/70 px-4 py-3">
				<div>
					<p class="text-sm font-medium">Administrator</p>
					<p class="text-xs text-muted-foreground">Grant full access to user management.</p>
				</div>
				<Switch
					checked={form.isAdmin}
					onchange={(event) => {
						form = { ...form, isAdmin: event.currentTarget.checked };
					}}
				/>
			</div>
		</div>
		{#if formError}
			<p
				class="mt-4 rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900/40 dark:bg-rose-900/30 dark:text-rose-200"
			>
				{formError}
			</p>
		{/if}
		<div class="mt-6 flex justify-end">
			<Button type="submit" disabled={submitting}>
				{#if submitting}
					Creating...
				{:else}
					Create user
				{/if}
			</Button>
		</div>
	</form>

	{#if userMessage}
		<p
			class="rounded border border-emerald-200 bg-emerald-50 px-4 py-2 text-sm text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-900/20 dark:text-emerald-200"
		>
			{userMessage}
		</p>
	{/if}

	<section class="overflow-hidden rounded-xl border bg-card shadow-sm">
		<table class="min-w-full divide-y divide-border">
			<thead class="bg-muted/50">
				<tr class="text-left text-xs font-medium tracking-wide text-muted-foreground uppercase">
					<th class="px-6 py-3">User</th>
					<th class="px-6 py-3">Role</th>
					<th class="px-6 py-3">Deployments</th>
					<th class="px-6 py-3">Created</th>
					<th class="px-6 py-3 text-right">Actions</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-border text-sm">
				{#each users as user (user.id)}
					<tr class="hover:bg-muted/30">
						<td class="px-6 py-4">
							<div class="font-medium text-foreground">{user.email}</div>
							{#if user.is_initial}
								<p class="text-xs text-muted-foreground">Initial administrator</p>
							{/if}
						</td>
						<td class="px-6 py-4">
							{#if user.is_admin}
								<span
									class="inline-flex items-center gap-2 rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-medium text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200"
								>
									<ShieldCheck class="h-3.5 w-3.5" /> Admin
								</span>
							{:else}
								<span
									class="inline-flex items-center gap-2 rounded-full bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground"
								>
									<ShieldOff class="h-3.5 w-3.5" /> User
								</span>
							{/if}
						</td>
						<td class="px-6 py-4 text-muted-foreground">{user.deployments}</td>
						<td class="px-6 py-4 text-xs text-muted-foreground">
							{new Date(user.created_at).toLocaleString()}
						</td>
						<td class="px-6 py-4">
							<div class="flex flex-wrap justify-end gap-2">
								<Button
									variant="outline"
									size="sm"
									disabled={busyUserId === user.id}
									onclick={() => toggleAdmin(user)}
								>
									{user.is_admin ? 'Remove admin' : 'Promote to admin'}
								</Button>
								<Button
									variant="outline"
									size="sm"
									disabled={busyUserId === user.id}
									onclick={() => openResetDialog(user)}
								>
									<Lock class="mr-2 h-4 w-4" /> Reset password
								</Button>
								<Button
									variant="destructive"
									size="sm"
									disabled={user.is_initial || busyUserId === user.id}
									onclick={() => openConfirmDialog(user)}
								>
									<Trash2 class="mr-2 h-4 w-4" /> Delete
								</Button>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</section>

	<!-- Reset Password Dialog -->
	<Dialog.Root open={resetDialog.open} onOpenChange={(v) => (resetDialog.open = v)}>
		<Dialog.Content>
			<Dialog.Header>
				<Dialog.Title>Reset password</Dialog.Title>
				<Dialog.Description>
					Set a new password for <strong>{resetDialog.user?.email}</strong>
					.
				</Dialog.Description>
			</Dialog.Header>
			<div class="space-y-3">
				<Label for="new-password">New password</Label>
				<Input
					id="new-password"
					type="password"
					minlength={8}
					required
					autocomplete="new-password"
					bind:value={resetDialog.password}
				/>
				{#if resetError}
					<p class="text-sm text-rose-600">{resetError}</p>
				{/if}
			</div>
			<Dialog.Footer class="mt-4 flex justify-end gap-2">
				<Button variant="outline" onclick={() => (resetDialog.open = false)}>Cancel</Button>
				<Button onclick={confirmResetPassword}>Update</Button>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>

	<!-- Delete Confirmation Dialog -->
	<Dialog.Root open={confirmDialog.open} onOpenChange={(v) => (confirmDialog.open = v)}>
		<Dialog.Content>
			<Dialog.Header>
				<Dialog.Title>Delete user</Dialog.Title>
				<Dialog.Description>
					Are you sure you want to delete
					<strong>{confirmDialog.user?.email}</strong>
					? This action cannot be undone.
				</Dialog.Description>
			</Dialog.Header>
			<Dialog.Footer class="flex justify-end gap-2">
				<Button variant="outline" onclick={() => (confirmDialog.open = false)}>Cancel</Button>
				<Button variant="destructive" onclick={confirmDelete}>Delete</Button>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>
</section>
