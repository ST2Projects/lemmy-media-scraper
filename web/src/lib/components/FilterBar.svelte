<script lang="ts">
	import { Search, SlidersHorizontal } from 'lucide-svelte';
	import type { Community } from '$lib/api';

	interface Props {
		communities: Community[];
		selectedCommunity: string;
		selectedType: string;
		selectedSort: string;
		searchQuery: string;
		onFilterChange: () => void;
		onSearch: (query: string) => void;
	}

	let {
		communities,
		selectedCommunity = $bindable(),
		selectedType = $bindable(),
		selectedSort = $bindable(),
		searchQuery = $bindable(),
		onFilterChange,
		onSearch
	}: Props = $props();

	let searchInput = $state(searchQuery);
	let searchTimeout: ReturnType<typeof setTimeout>;

	function handleSearchInput() {
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			onSearch(searchInput);
		}, 300);
	}

	function handleSearchKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			clearTimeout(searchTimeout);
			onSearch(searchInput);
		}
	}
</script>

<div class="flex flex-wrap items-center gap-3 rounded-lg bg-[#1a1a1a] p-4">
	<div class="relative min-w-[200px] flex-1">
		<Search class="absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-[#666]" />
		<input
			type="text"
			placeholder="Search media..."
			bind:value={searchInput}
			oninput={handleSearchInput}
			onkeydown={handleSearchKeydown}
			class="w-full rounded-md border border-[#333] bg-[#2a2a2a] py-2 pr-4 pl-10 text-sm text-[#e0e0e0] placeholder-[#666] outline-none transition-colors focus:border-[#6366f1]"
		/>
	</div>

	<div class="flex items-center gap-2">
		<SlidersHorizontal class="h-4 w-4 text-[#999]" />

		<select
			bind:value={selectedCommunity}
			onchange={onFilterChange}
			class="rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none transition-colors focus:border-[#6366f1]"
		>
			<option value="">All Communities</option>
			{#each communities as community}
				<option value={community.name}>{community.name} ({community.count})</option>
			{/each}
		</select>

		<select
			bind:value={selectedType}
			onchange={onFilterChange}
			class="rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none transition-colors focus:border-[#6366f1]"
		>
			<option value="">All Types</option>
			<option value="image">Images</option>
			<option value="video">Videos</option>
			<option value="other">Other</option>
		</select>

		<select
			bind:value={selectedSort}
			onchange={onFilterChange}
			class="rounded-md border border-[#333] bg-[#2a2a2a] px-3 py-2 text-sm text-[#e0e0e0] outline-none transition-colors focus:border-[#6366f1]"
		>
			<option value="downloaded_at">Latest Downloaded</option>
			<option value="post_created">Post Date</option>
			<option value="file_size">File Size</option>
			<option value="post_score">Score</option>
		</select>
	</div>
</div>
