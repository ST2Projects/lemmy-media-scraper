<script lang="ts">
	import { getMedia, getCommunities, searchMedia } from '$lib/api';
	import type { MediaItem, Community } from '$lib/api';
	import FilterBar from '$lib/components/FilterBar.svelte';
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import MediaModal from '$lib/components/MediaModal.svelte';
	import Pagination from '$lib/components/Pagination.svelte';

	let media = $state<MediaItem[]>([]);
	let communities = $state<Community[]>([]);
	let total = $state(0);
	let loading = $state(true);

	let selectedCommunity = $state('');
	let selectedType = $state('');
	let selectedSort = $state('downloaded_at');
	let searchQuery = $state('');
	let offset = $state(0);
	const limit = 30;

	let selectedItem = $state<MediaItem | null>(null);

	$effect(() => {
		loadCommunities();
		loadMedia();
	});

	async function loadCommunities() {
		try {
			const res = await getCommunities();
			communities = res.communities || [];
		} catch {
			communities = [];
		}
	}

	async function loadMedia() {
		loading = true;
		try {
			if (searchQuery) {
				const res = await searchMedia(searchQuery, { limit, offset });
				media = res.media || [];
				total = res.total || 0;
			} else {
				const res = await getMedia({
					community: selectedCommunity || undefined,
					type: selectedType || undefined,
					sort: selectedSort,
					order: 'desc',
					limit,
					offset
				});
				media = res.media || [];
				total = res.total || 0;
			}
		} catch {
			media = [];
			total = 0;
		} finally {
			loading = false;
		}
	}

	function handleFilterChange() {
		offset = 0;
		loadMedia();
	}

	function handleSearch(query: string) {
		searchQuery = query;
		offset = 0;
		loadMedia();
	}

	function handlePageChange(newOffset: number) {
		offset = newOffset;
		loadMedia();
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	function handleSelect(item: MediaItem) {
		selectedItem = item;
	}

	function handleCloseModal() {
		selectedItem = null;
	}
</script>

<svelte:head>
	<title>Media Browser - Lemmy Media Scraper</title>
</svelte:head>

<div class="space-y-4">
	<FilterBar
		{communities}
		bind:selectedCommunity
		bind:selectedType
		bind:selectedSort
		bind:searchQuery
		onFilterChange={handleFilterChange}
		onSearch={handleSearch}
	/>

	{#if loading}
		<div class="py-20 text-center">
			<div class="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-[#333] border-t-[#6366f1]"></div>
			<p class="mt-4 text-sm text-[#666]">Loading media...</p>
		</div>
	{:else}
		<MediaGrid items={media} onSelect={handleSelect} />

		<div class="py-4">
			<Pagination {total} {limit} {offset} onPageChange={handlePageChange} />
		</div>
	{/if}
</div>

<MediaModal item={selectedItem} onClose={handleCloseModal} />
