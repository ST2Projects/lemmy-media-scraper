<script lang="ts">
	import { authClient } from '$lib/auth-client';
	import { goto } from '$app/navigation';
	import { Wrench } from 'lucide-svelte';

	let name = $state('Admin');
	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleSetup(e: SubmitEvent) {
		e.preventDefault();
		error = '';

		if (password !== confirmPassword) {
			error = 'Passwords do not match.';
			return;
		}

		if (password.length < 8) {
			error = 'Password must be at least 8 characters.';
			return;
		}

		loading = true;

		const result = await authClient.signUp.email({
			name,
			email,
			password
		});

		if (result.error) {
			error = result.error.message ?? 'Account creation failed.';
			loading = false;
		} else {
			goto('/login');
		}
	}
</script>

<svelte:head>
	<title>Setup - Lemmy Media Scraper</title>
</svelte:head>

<div class="flex min-h-[60vh] items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-8">
			<div class="mb-6 text-center">
				<Wrench class="mx-auto h-12 w-12 text-[#6366f1]" />
				<h1 class="mt-4 text-xl font-semibold text-[#e0e0e0]">Initial Setup</h1>
				<p class="mt-2 text-sm text-[#666]">
					Create the admin account for managing settings.
				</p>
			</div>

			{#if error}
				<div class="mb-4 rounded-md border border-[#ef4444]/30 bg-[#ef4444]/10 p-3 text-sm text-[#ef4444]">
					{error}
				</div>
			{/if}

			<form onsubmit={handleSetup}>
				<div class="mb-4">
					<label for="name" class="mb-1 block text-sm text-[#999]">Name</label>
					<input
						id="name"
						type="text"
						bind:value={name}
						required
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-[#e0e0e0] placeholder-[#666] focus:border-[#6366f1] focus:outline-none"
						placeholder="Admin"
					/>
				</div>

				<div class="mb-4">
					<label for="email" class="mb-1 block text-sm text-[#999]">Email</label>
					<input
						id="email"
						type="email"
						bind:value={email}
						required
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-[#e0e0e0] placeholder-[#666] focus:border-[#6366f1] focus:outline-none"
						placeholder="admin@example.com"
					/>
				</div>

				<div class="mb-4">
					<label for="password" class="mb-1 block text-sm text-[#999]">Password</label>
					<input
						id="password"
						type="password"
						bind:value={password}
						required
						minlength={8}
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-[#e0e0e0] placeholder-[#666] focus:border-[#6366f1] focus:outline-none"
						placeholder="At least 8 characters"
					/>
				</div>

				<div class="mb-6">
					<label for="confirmPassword" class="mb-1 block text-sm text-[#999]">
						Confirm Password
					</label>
					<input
						id="confirmPassword"
						type="password"
						bind:value={confirmPassword}
						required
						minlength={8}
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-[#e0e0e0] placeholder-[#666] focus:border-[#6366f1] focus:outline-none"
						placeholder="Repeat your password"
					/>
				</div>

				<button
					type="submit"
					disabled={loading}
					class="w-full rounded-md bg-[#6366f1] px-4 py-2 font-medium text-white transition-colors hover:bg-[#818cf8] disabled:opacity-50"
				>
					{loading ? 'Creating account...' : 'Create Admin Account'}
				</button>
			</form>

			<p class="mt-4 text-center text-sm text-[#666]">
				Already set up? <a href="/login" class="text-[#6366f1] hover:underline">Sign in</a>
			</p>
		</div>
	</div>
</div>
