// API client for the Lemmy Media Scraper backend.
// All requests use relative URLs - in dev mode, Vite proxies to the Go backend.
// In production, SvelteKit server routes proxy to the Go backend.

export interface MediaItem {
	id: number;
	post_id: number;
	post_title: string;
	community_name: string;
	community_id: number;
	author_name: string;
	author_id: number;
	media_url: string;
	media_hash: string;
	file_name: string;
	file_path: string;
	file_size: number;
	media_type: string;
	post_url: string;
	post_score: number;
	post_created: string;
	downloaded_at: string;
	serve_url: string;
}

export interface MediaResponse {
	media: MediaItem[];
	total: number;
	limit: number;
	offset: number;
}

export interface Community {
	name: string;
	count: number;
}

export interface CommunitiesResponse {
	communities: Community[];
}

export interface Comment {
	comment_id: number;
	post_id: number;
	creator_name: string;
	creator_id: number;
	content: string;
	score: number;
	upvotes: number;
	downvotes: number;
	comment_path: string;
	created_at: string;
}

export interface CommentsResponse {
	comments: Comment[];
	post_id: number;
}

export interface Stats {
	total_media: number;
	by_type: Record<string, number>;
	top_communities: Record<string, number>;
}

export interface TimelineEntry {
	period: string;
	count: number;
}

export interface TopCreator {
	name: string;
	count: number;
}

export interface StorageBreakdown {
	by_community: { name: string; size: number; count: number }[];
	by_type: { type: string; size: number; count: number }[];
}

export interface AppConfig {
	lemmy: {
		instance: string;
		username: string;
		password: string;
		communities: string[];
	};
	storage: {
		base_directory: string;
	};
	database: {
		path: string;
	};
	scraper: {
		max_posts_per_run: number;
		stop_at_seen_posts: boolean;
		skip_seen_posts: boolean;
		enable_pagination: boolean;
		seen_posts_threshold: number;
		sort_type: string;
		include_images: boolean;
		include_videos: boolean;
		include_other_media: boolean;
	};
	run_mode: {
		mode: string;
		interval: number;
	};
	web_server: {
		enabled: boolean;
		host: string;
		port: number;
	};
	thumbnails: {
		enabled: boolean;
		max_width: number;
		max_height: number;
		quality: number;
		directory: string;
		video_method: string;
	};
	search: {
		rebuild_index: boolean;
	};
}

export interface SearchResponse {
	media: MediaItem[];
	total: number;
	limit: number;
	offset: number;
}

export interface ProgressUpdate {
	status: string;
	community: string;
	posts_processed: number;
	media_downloaded: number;
	errors: number;
	eta_seconds: number;
	is_running: boolean;
}

export async function getMedia(
	params?: {
		community?: string;
		type?: string;
		sort?: string;
		order?: string;
		limit?: number;
		offset?: number;
	},
	fetchFn: typeof fetch = fetch
): Promise<MediaResponse> {
	const searchParams = new URLSearchParams();
	if (params?.community) searchParams.set('community', params.community);
	if (params?.type) searchParams.set('type', params.type);
	if (params?.sort) searchParams.set('sort', params.sort);
	if (params?.order) searchParams.set('order', params.order);
	if (params?.limit) searchParams.set('limit', String(params.limit));
	if (params?.offset) searchParams.set('offset', String(params.offset));
	const qs = searchParams.toString();
	const res = await fetchFn(`/api/media${qs ? `?${qs}` : ''}`);
	if (!res.ok) throw new Error(`Failed to fetch media: ${res.statusText}`);
	return res.json();
}

export async function getMediaById(
	id: number,
	fetchFn: typeof fetch = fetch
): Promise<MediaItem> {
	const res = await fetchFn(`/api/media/${id}`);
	if (!res.ok) throw new Error(`Failed to fetch media item: ${res.statusText}`);
	return res.json();
}

export async function getComments(
	mediaId: number,
	fetchFn: typeof fetch = fetch
): Promise<CommentsResponse> {
	const res = await fetchFn(`/api/comments/${mediaId}`);
	if (!res.ok) throw new Error(`Failed to fetch comments: ${res.statusText}`);
	return res.json();
}

export async function getStats(fetchFn: typeof fetch = fetch): Promise<Stats> {
	const res = await fetchFn(`/api/stats`);
	if (!res.ok) throw new Error(`Failed to fetch stats: ${res.statusText}`);
	return res.json();
}

export async function getCommunities(
	fetchFn: typeof fetch = fetch
): Promise<CommunitiesResponse> {
	const res = await fetchFn(`/api/communities`);
	if (!res.ok) throw new Error(`Failed to fetch communities: ${res.statusText}`);
	return res.json();
}

export async function getConfig(fetchFn: typeof fetch = fetch): Promise<AppConfig> {
	const res = await fetchFn(`/api/config`);
	if (!res.ok) throw new Error(`Failed to fetch config: ${res.statusText}`);
	return res.json();
}

export async function updateConfig(
	config: AppConfig,
	fetchFn: typeof fetch = fetch
): Promise<void> {
	const res = await fetchFn(`/api/config`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(config)
	});
	if (!res.ok) throw new Error(`Failed to update config: ${res.statusText}`);
}

export async function searchMedia(
	query: string,
	params?: { limit?: number; offset?: number },
	fetchFn: typeof fetch = fetch
): Promise<SearchResponse> {
	const searchParams = new URLSearchParams({ q: query });
	if (params?.limit) searchParams.set('limit', String(params.limit));
	if (params?.offset) searchParams.set('offset', String(params.offset));
	const res = await fetchFn(`/api/search?${searchParams}`);
	if (!res.ok) throw new Error(`Failed to search: ${res.statusText}`);
	return res.json();
}

export async function getTimeline(
	period: 'day' | 'week' | 'month' = 'day',
	fetchFn: typeof fetch = fetch
): Promise<TimelineEntry[]> {
	const res = await fetchFn(`/api/stats/timeline?period=${period}`);
	if (!res.ok) throw new Error(`Failed to fetch timeline: ${res.statusText}`);
	return res.json();
}

export async function getTopCreators(
	limit = 10,
	fetchFn: typeof fetch = fetch
): Promise<TopCreator[]> {
	const res = await fetchFn(`/api/stats/top-creators?limit=${limit}`);
	if (!res.ok) throw new Error(`Failed to fetch top creators: ${res.statusText}`);
	return res.json();
}

export async function getStorageBreakdown(
	fetchFn: typeof fetch = fetch
): Promise<StorageBreakdown> {
	const res = await fetchFn(`/api/stats/storage`);
	if (!res.ok) throw new Error(`Failed to fetch storage: ${res.statusText}`);
	return res.json();
}

export function formatFileSize(bytes: number): string {
	if (bytes === 0) return '0 B';
	const k = 1024;
	const sizes = ['B', 'KB', 'MB', 'GB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export function formatDate(dateStr: string): string {
	const d = new Date(dateStr);
	return d.toLocaleDateString('en-US', {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	});
}

export function getMediaTypeIcon(type: string): string {
	if (type === 'image') return 'image';
	if (type === 'video') return 'video';
	return 'file';
}
