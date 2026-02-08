<script lang="ts">
	import { authClient } from '$lib/auth-client';
	import { goto } from '$app/navigation';
	import { LogIn } from 'lucide-svelte';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleLogin(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		loading = true;

		const result = await authClient.signIn.email({
			email,
			password
		});

		if (result.error) {
			error = result.error.message ?? 'Login failed. Please check your credentials.';
			loading = false;
		} else {
			goto('/');
		}
	}
</script>

<svelte:head>
	<title>Login - Lemmy Media Scraper</title>
</svelte:head>

<div class="flex min-h-[60vh] items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-8">
			<div class="mb-6 text-center">
				<LogIn class="mx-auto h-12 w-12 text-[#6366f1]" />
				<h1 class="mt-4 text-xl font-semibold text-[#e0e0e0]">Login</h1>
			</div>

			{#if error}
				<div class="mb-4 rounded-md border border-[#ef4444]/30 bg-[#ef4444]/10 p-3 text-sm text-[#ef4444]">
					{error}
				</div>
			{/if}

			<form onsubmit={handleLogin}>
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

				<div class="mb-6">
					<label for="password" class="mb-1 block text-sm text-[#999]">Password</label>
					<input
						id="password"
						type="password"
						bind:value={password}
						required
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-[#e0e0e0] placeholder-[#666] focus:border-[#6366f1] focus:outline-none"
						placeholder="Enter your password"
					/>
				</div>

				<button
					type="submit"
					disabled={loading}
					class="w-full rounded-md bg-[#6366f1] px-4 py-2 font-medium text-white transition-colors hover:bg-[#818cf8] disabled:opacity-50"
				>
					{loading ? 'Signing in...' : 'Sign In'}
				</button>
			</form>

			<p class="mt-4 text-center text-sm text-[#666]">
				First time? <a href="/setup" class="text-[#6366f1] hover:underline">Create admin account</a>
			</p>
		</div>
	</div>
</div>
