import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const BACKEND_URL = env.GO_BACKEND_URL || 'http://localhost:8081';

function validatePath(path: string): boolean {
	if (path.includes('..') || path.includes('//')) return false;
	if (!/^[a-zA-Z0-9\/_\-\.]+$/.test(path)) return false;
	return true;
}

export const GET: RequestHandler = async ({ params }) => {
	if (!validatePath(params.path)) {
		return new Response(JSON.stringify({ error: 'Invalid path' }), {
			status: 400,
			headers: { 'Content-Type': 'application/json' }
		});
	}
	const targetUrl = `${BACKEND_URL}/thumbnails/${params.path}`;
	const res = await fetch(targetUrl);
	return new Response(res.body, {
		status: res.status,
		headers: {
			'content-type': res.headers.get('content-type') || 'image/jpeg',
			'cache-control': res.headers.get('cache-control') || 'public, max-age=86400'
		}
	});
};
