<script lang="ts">
	import { ChevronLeft, ChevronRight } from 'lucide-svelte';

	interface Props {
		total: number;
		limit: number;
		offset: number;
		onPageChange: (offset: number) => void;
	}

	let { total, limit, offset, onPageChange }: Props = $props();

	let currentPage = $derived(Math.floor(offset / limit) + 1);
	let totalPages = $derived(Math.ceil(total / limit));

	let visiblePages = $derived.by(() => {
		const pages: number[] = [];
		const start = Math.max(1, currentPage - 2);
		const end = Math.min(totalPages, currentPage + 2);
		for (let i = start; i <= end; i++) {
			pages.push(i);
		}
		return pages;
	});

	function goToPage(page: number) {
		if (page < 1 || page > totalPages) return;
		onPageChange((page - 1) * limit);
	}
</script>

{#if totalPages > 1}
	<div class="flex items-center justify-center gap-1">
		<button
			class="rounded p-2 text-[#999] transition-colors hover:bg-[#2a2a2a] hover:text-white disabled:opacity-30 disabled:hover:bg-transparent"
			disabled={currentPage === 1}
			onclick={() => goToPage(currentPage - 1)}
		>
			<ChevronLeft class="h-4 w-4" />
		</button>

		{#if visiblePages[0] > 1}
			<button
				class="rounded px-3 py-1 text-sm text-[#999] transition-colors hover:bg-[#2a2a2a] hover:text-white"
				onclick={() => goToPage(1)}
			>
				1
			</button>
			{#if visiblePages[0] > 2}
				<span class="px-1 text-[#666]">...</span>
			{/if}
		{/if}

		{#each visiblePages as page}
			<button
				class="rounded px-3 py-1 text-sm transition-colors {page === currentPage
					? 'bg-[#6366f1] text-white'
					: 'text-[#999] hover:bg-[#2a2a2a] hover:text-white'}"
				onclick={() => goToPage(page)}
			>
				{page}
			</button>
		{/each}

		{#if visiblePages[visiblePages.length - 1] < totalPages}
			{#if visiblePages[visiblePages.length - 1] < totalPages - 1}
				<span class="px-1 text-[#666]">...</span>
			{/if}
			<button
				class="rounded px-3 py-1 text-sm text-[#999] transition-colors hover:bg-[#2a2a2a] hover:text-white"
				onclick={() => goToPage(totalPages)}
			>
				{totalPages}
			</button>
		{/if}

		<button
			class="rounded p-2 text-[#999] transition-colors hover:bg-[#2a2a2a] hover:text-white disabled:opacity-30 disabled:hover:bg-transparent"
			disabled={currentPage === totalPages}
			onclick={() => goToPage(currentPage + 1)}
		>
			<ChevronRight class="h-4 w-4" />
		</button>

		<span class="ml-3 text-xs text-[#666]">
			{offset + 1}-{Math.min(offset + limit, total)} of {total}
		</span>
	</div>
{/if}
