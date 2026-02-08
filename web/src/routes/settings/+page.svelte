<script lang="ts">
	import {
		Settings,
		Server,
		Database,
		HardDrive,
		Bot,
		Clock,
		Globe,
		Image,
		Search,
		Save,
		Loader,
		X,
		Plus,
		Check,
		AlertCircle
	} from 'lucide-svelte';
	import { getConfig, updateConfig } from '$lib/api';
	import type { AppConfig } from '$lib/api';

	let config = $state<AppConfig | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let toast = $state<{ type: 'success' | 'error'; message: string } | null>(null);
	let newCommunity = $state('');

	// Human-readable interval string
	let intervalStr = $state('');

	$effect(() => {
		loadConfig();
	});

	// Auto-dismiss toast
	$effect(() => {
		if (toast) {
			const timer = setTimeout(() => {
				toast = null;
			}, 4000);
			return () => clearTimeout(timer);
		}
	});

	async function loadConfig() {
		loading = true;
		try {
			config = await getConfig();
			intervalStr = nanosToHumanStr(config.run_mode.interval);
		} catch (e) {
			showToast('error', 'Failed to load configuration');
		} finally {
			loading = false;
		}
	}

	async function handleSave() {
		if (!config) return;
		saving = true;

		// Convert interval string back to nanoseconds
		const intervalNanos = humanStrToNanos(intervalStr);
		if (intervalNanos === null && config.run_mode.mode === 'continuous') {
			showToast('error', 'Invalid interval format. Use e.g. "5m", "1h30m", "30s"');
			saving = false;
			return;
		}

		try {
			const configToSave = {
				...config,
				run_mode: {
					...config.run_mode,
					interval: intervalNanos ?? config.run_mode.interval
				}
			};
			await updateConfig(configToSave);
			showToast('success', 'Configuration saved. Restart the application for changes to take effect.');
		} catch (e) {
			showToast('error', 'Failed to save configuration');
		} finally {
			saving = false;
		}
	}

	function showToast(type: 'success' | 'error', message: string) {
		toast = { type, message };
	}

	function addCommunity() {
		if (!config || !newCommunity.trim()) return;
		if (!config.lemmy.communities.includes(newCommunity.trim())) {
			config.lemmy.communities = [...config.lemmy.communities, newCommunity.trim()];
		}
		newCommunity = '';
	}

	function removeCommunity(index: number) {
		if (!config) return;
		config.lemmy.communities = config.lemmy.communities.filter((_, i) => i !== index);
	}

	function handleCommunityKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			addCommunity();
		}
	}

	// Duration conversion helpers
	function nanosToHumanStr(nanos: number): string {
		if (!nanos || nanos <= 0) return '5m';
		const totalSeconds = Math.floor(nanos / 1_000_000_000);
		const hours = Math.floor(totalSeconds / 3600);
		const minutes = Math.floor((totalSeconds % 3600) / 60);
		const seconds = totalSeconds % 60;
		let parts: string[] = [];
		if (hours > 0) parts.push(`${hours}h`);
		if (minutes > 0) parts.push(`${minutes}m`);
		if (seconds > 0 || parts.length === 0) parts.push(`${seconds}s`);
		return parts.join('');
	}

	function humanStrToNanos(str: string): number | null {
		const trimmed = str.trim();
		if (!trimmed) return null;

		let totalSeconds = 0;
		let remaining = trimmed;
		let matched = false;

		const patterns: [RegExp, number][] = [
			[/^(\d+)h/, 3600],
			[/^(\d+)m/, 60],
			[/^(\d+)s/, 1]
		];

		while (remaining.length > 0) {
			let foundMatch = false;
			for (const [pattern, multiplier] of patterns) {
				const match = remaining.match(pattern);
				if (match) {
					totalSeconds += parseInt(match[1]) * multiplier;
					remaining = remaining.slice(match[0].length);
					foundMatch = true;
					matched = true;
					break;
				}
			}
			if (!foundMatch) return null;
		}

		return matched ? totalSeconds * 1_000_000_000 : null;
	}
</script>

<svelte:head>
	<title>Settings - Lemmy Media Scraper</title>
</svelte:head>

<!-- Toast notification -->
{#if toast}
	<div class="fixed top-4 right-4 z-50 flex items-center gap-2 rounded-lg border px-4 py-3 shadow-lg {toast.type === 'success' ? 'border-green-500/30 bg-green-500/10 text-green-400' : 'border-red-500/30 bg-red-500/10 text-red-400'}">
		{#if toast.type === 'success'}
			<Check class="h-4 w-4" />
		{:else}
			<AlertCircle class="h-4 w-4" />
		{/if}
		<span class="text-sm">{toast.message}</span>
		<button onclick={() => (toast = null)} class="ml-2 opacity-60 hover:opacity-100">
			<X class="h-3 w-3" />
		</button>
	</div>
{/if}

{#if loading}
	<div class="py-20 text-center">
		<div class="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-[#333] border-t-[#6366f1]"></div>
		<p class="mt-4 text-sm text-[#666]">Loading configuration...</p>
	</div>
{:else if config}
	<div class="mx-auto max-w-3xl space-y-6">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-3">
				<Settings class="h-6 w-6 text-[#6366f1]" />
				<h1 class="text-xl font-semibold text-[#e0e0e0]">Settings</h1>
			</div>
			<button
				onclick={handleSave}
				disabled={saving}
				class="flex items-center gap-2 rounded-md bg-[#6366f1] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#818cf8] disabled:opacity-50"
			>
				{#if saving}
					<Loader class="h-4 w-4 animate-spin" />
					Saving...
				{:else}
					<Save class="h-4 w-4" />
					Save Changes
				{/if}
			</button>
		</div>

		<!-- Lemmy Instance -->
		<section class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
			<div class="mb-4 flex items-center gap-2">
				<Server class="h-5 w-5 text-[#999]" />
				<h2 class="text-lg font-semibold text-[#e0e0e0]">Lemmy Instance</h2>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				<div>
					<label for="instance" class="mb-1 block text-sm text-[#999]">Instance URL</label>
					<input
						id="instance"
						type="text"
						bind:value={config.lemmy.instance}
						placeholder="https://lemmy.ml"
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] placeholder-[#666] outline-none focus:border-[#6366f1]"
					/>
				</div>
				<div>
					<label for="username" class="mb-1 block text-sm text-[#999]">Username</label>
					<input
						id="username"
						type="text"
						bind:value={config.lemmy.username}
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] placeholder-[#666] outline-none focus:border-[#6366f1]"
					/>
				</div>
				<div class="md:col-span-2">
					<label for="password" class="mb-1 block text-sm text-[#999]">Password</label>
					<input
						id="password"
						type="password"
						bind:value={config.lemmy.password}
						placeholder="Leave blank to keep existing"
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] placeholder-[#666] outline-none focus:border-[#6366f1]"
					/>
					<p class="mt-1 text-xs text-[#666]">Leave empty to keep the current password.</p>
				</div>
			</div>

			<!-- Communities -->
			<div class="mt-4">
				<label class="mb-2 block text-sm text-[#999]">Communities</label>
				<div class="flex flex-wrap gap-2">
					{#each config.lemmy.communities as community, i}
						<span class="flex items-center gap-1 rounded-md bg-[#2a2a2a] px-2.5 py-1 text-sm text-[#e0e0e0]">
							{community}
							<button
								onclick={() => removeCommunity(i)}
								class="ml-0.5 text-[#666] hover:text-red-400"
							>
								<X class="h-3 w-3" />
							</button>
						</span>
					{/each}
				</div>
				<div class="mt-2 flex gap-2">
					<input
						type="text"
						bind:value={newCommunity}
						onkeydown={handleCommunityKeydown}
						placeholder="Add community (e.g. technology or technology@lemmy.ml)"
						class="flex-1 rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] placeholder-[#666] outline-none focus:border-[#6366f1]"
					/>
					<button
						onclick={addCommunity}
						class="rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#999] transition-colors hover:border-[#6366f1] hover:text-[#e0e0e0]"
					>
						<Plus class="h-4 w-4" />
					</button>
				</div>
				<p class="mt-1 text-xs text-[#666]">Leave empty to scrape from the instance hot page.</p>
			</div>
		</section>

		<!-- Storage -->
		<section class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
			<div class="mb-4 flex items-center gap-2">
				<HardDrive class="h-5 w-5 text-[#999]" />
				<h2 class="text-lg font-semibold text-[#e0e0e0]">Storage</h2>
			</div>
			<div>
				<label for="base_dir" class="mb-1 block text-sm text-[#999]">Base Directory</label>
				<input
					id="base_dir"
					type="text"
					bind:value={config.storage.base_directory}
					class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
				/>
			</div>
		</section>

		<!-- Database -->
		<section class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
			<div class="mb-4 flex items-center gap-2">
				<Database class="h-5 w-5 text-[#999]" />
				<h2 class="text-lg font-semibold text-[#e0e0e0]">Database</h2>
			</div>
			<div>
				<label for="db_path" class="mb-1 block text-sm text-[#999]">Database Path</label>
				<input
					id="db_path"
					type="text"
					bind:value={config.database.path}
					class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
				/>
			</div>
		</section>

		<!-- Scraper -->
		<section class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
			<div class="mb-4 flex items-center gap-2">
				<Bot class="h-5 w-5 text-[#999]" />
				<h2 class="text-lg font-semibold text-[#e0e0e0]">Scraper</h2>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				<div>
					<label for="max_posts" class="mb-1 block text-sm text-[#999]">Max Posts Per Run</label>
					<input
						id="max_posts"
						type="number"
						min="1"
						bind:value={config.scraper.max_posts_per_run}
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
					/>
				</div>
				<div>
					<label for="sort_type" class="mb-1 block text-sm text-[#999]">Sort Type</label>
					<select
						id="sort_type"
						bind:value={config.scraper.sort_type}
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
					>
						<option value="Hot">Hot</option>
						<option value="New">New</option>
						<option value="TopDay">Top Day</option>
						<option value="TopWeek">Top Week</option>
						<option value="TopMonth">Top Month</option>
						<option value="TopYear">Top Year</option>
						<option value="TopAll">Top All</option>
						<option value="Active">Active</option>
					</select>
				</div>
				<div>
					<label for="seen_threshold" class="mb-1 block text-sm text-[#999]">Seen Posts Threshold</label>
					<input
						id="seen_threshold"
						type="number"
						min="1"
						bind:value={config.scraper.seen_posts_threshold}
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
					/>
					<p class="mt-1 text-xs text-[#666]">Number of consecutive seen posts before stopping.</p>
				</div>
			</div>

			<div class="mt-4 grid gap-4 sm:grid-cols-2 md:grid-cols-3">
				<label class="flex items-center gap-3 rounded-md bg-[#222] px-3 py-2.5">
					<input type="checkbox" bind:checked={config.scraper.stop_at_seen_posts} class="accent-[#6366f1]" />
					<div>
						<span class="text-sm text-[#e0e0e0]">Stop at Seen Posts</span>
						<p class="text-xs text-[#666]">Stop after hitting threshold</p>
					</div>
				</label>
				<label class="flex items-center gap-3 rounded-md bg-[#222] px-3 py-2.5">
					<input type="checkbox" bind:checked={config.scraper.skip_seen_posts} class="accent-[#6366f1]" />
					<div>
						<span class="text-sm text-[#e0e0e0]">Skip Seen Posts</span>
						<p class="text-xs text-[#666]">Skip but keep scraping</p>
					</div>
				</label>
				<label class="flex items-center gap-3 rounded-md bg-[#222] px-3 py-2.5">
					<input type="checkbox" bind:checked={config.scraper.enable_pagination} class="accent-[#6366f1]" />
					<div>
						<span class="text-sm text-[#e0e0e0]">Enable Pagination</span>
						<p class="text-xs text-[#666]">Fetch beyond 50-post limit</p>
					</div>
				</label>
				<label class="flex items-center gap-3 rounded-md bg-[#222] px-3 py-2.5">
					<input type="checkbox" bind:checked={config.scraper.include_images} class="accent-[#6366f1]" />
					<span class="text-sm text-[#e0e0e0]">Include Images</span>
				</label>
				<label class="flex items-center gap-3 rounded-md bg-[#222] px-3 py-2.5">
					<input type="checkbox" bind:checked={config.scraper.include_videos} class="accent-[#6366f1]" />
					<span class="text-sm text-[#e0e0e0]">Include Videos</span>
				</label>
				<label class="flex items-center gap-3 rounded-md bg-[#222] px-3 py-2.5">
					<input type="checkbox" bind:checked={config.scraper.include_other_media} class="accent-[#6366f1]" />
					<span class="text-sm text-[#e0e0e0]">Include Other Media</span>
				</label>
			</div>
		</section>

		<!-- Run Mode -->
		<section class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
			<div class="mb-4 flex items-center gap-2">
				<Clock class="h-5 w-5 text-[#999]" />
				<h2 class="text-lg font-semibold text-[#e0e0e0]">Run Mode</h2>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				<div>
					<label for="run_mode" class="mb-1 block text-sm text-[#999]">Mode</label>
					<select
						id="run_mode"
						bind:value={config.run_mode.mode}
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
					>
						<option value="once">Once</option>
						<option value="continuous">Continuous</option>
					</select>
				</div>
				{#if config.run_mode.mode === 'continuous'}
					<div>
						<label for="interval" class="mb-1 block text-sm text-[#999]">Interval</label>
						<input
							id="interval"
							type="text"
							bind:value={intervalStr}
							placeholder="e.g. 5m, 1h, 30m, 1h30m"
							class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] placeholder-[#666] outline-none focus:border-[#6366f1]"
						/>
						<p class="mt-1 text-xs text-[#666]">Format: 1h, 30m, 5m30s, etc.</p>
					</div>
				{/if}
			</div>
		</section>

		<!-- Web Server -->
		<section class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
			<div class="mb-4 flex items-center gap-2">
				<Globe class="h-5 w-5 text-[#999]" />
				<h2 class="text-lg font-semibold text-[#e0e0e0]">Web Server</h2>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				<div>
					<label for="host" class="mb-1 block text-sm text-[#999]">Host</label>
					<input
						id="host"
						type="text"
						bind:value={config.web_server.host}
						placeholder="localhost"
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] placeholder-[#666] outline-none focus:border-[#6366f1]"
					/>
				</div>
				<div>
					<label for="port" class="mb-1 block text-sm text-[#999]">Port</label>
					<input
						id="port"
						type="number"
						min="1"
						max="65535"
						bind:value={config.web_server.port}
						class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
					/>
				</div>
			</div>
		</section>

		<!-- Thumbnails -->
		<section class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
			<div class="mb-4 flex items-center gap-2">
				<Image class="h-5 w-5 text-[#999]" />
				<h2 class="text-lg font-semibold text-[#e0e0e0]">Thumbnails</h2>
			</div>

			<label class="mb-4 flex items-center gap-3">
				<input type="checkbox" bind:checked={config.thumbnails.enabled} class="accent-[#6366f1]" />
				<span class="text-sm text-[#e0e0e0]">Enable Thumbnail Generation</span>
			</label>

			{#if config.thumbnails.enabled}
				<div class="grid gap-4 md:grid-cols-2">
					<div>
						<label for="thumb_width" class="mb-1 block text-sm text-[#999]">Max Width</label>
						<input
							id="thumb_width"
							type="number"
							min="50"
							bind:value={config.thumbnails.max_width}
							class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
						/>
					</div>
					<div>
						<label for="thumb_height" class="mb-1 block text-sm text-[#999]">Max Height</label>
						<input
							id="thumb_height"
							type="number"
							min="50"
							bind:value={config.thumbnails.max_height}
							class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
						/>
					</div>
					<div class="md:col-span-2">
						<label for="thumb_quality" class="mb-1 block text-sm text-[#999]">
							Quality: {config.thumbnails.quality}
						</label>
						<input
							id="thumb_quality"
							type="range"
							min="1"
							max="100"
							bind:value={config.thumbnails.quality}
							class="w-full accent-[#6366f1]"
						/>
						<div class="flex justify-between text-xs text-[#666]">
							<span>1 (smallest)</span>
							<span>100 (best)</span>
						</div>
					</div>
					<div>
						<label for="thumb_dir" class="mb-1 block text-sm text-[#999]">Directory</label>
						<input
							id="thumb_dir"
							type="text"
							bind:value={config.thumbnails.directory}
							class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
						/>
					</div>
					<div>
						<label for="video_method" class="mb-1 block text-sm text-[#999]">Video Method</label>
						<select
							id="video_method"
							bind:value={config.thumbnails.video_method}
							class="w-full rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none focus:border-[#6366f1]"
						>
							<option value="ffmpeg">ffmpeg</option>
						</select>
					</div>
				</div>
			{/if}
		</section>

		<!-- Search -->
		<section class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
			<div class="mb-4 flex items-center gap-2">
				<Search class="h-5 w-5 text-[#999]" />
				<h2 class="text-lg font-semibold text-[#e0e0e0]">Search</h2>
			</div>

			<label class="flex items-center gap-3">
				<input type="checkbox" bind:checked={config.search.rebuild_index} class="accent-[#6366f1]" />
				<div>
					<span class="text-sm text-[#e0e0e0]">Rebuild Search Index</span>
					<p class="text-xs text-[#666]">Rebuild the FTS5 search index on next startup. Only needed if search results seem incorrect.</p>
				</div>
			</label>
		</section>

		<!-- Bottom save button -->
		<div class="flex justify-end pb-8">
			<button
				onclick={handleSave}
				disabled={saving}
				class="flex items-center gap-2 rounded-md bg-[#6366f1] px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-[#818cf8] disabled:opacity-50"
			>
				{#if saving}
					<Loader class="h-4 w-4 animate-spin" />
					Saving...
				{:else}
					<Save class="h-4 w-4" />
					Save Changes
				{/if}
			</button>
		</div>
	</div>
{/if}
