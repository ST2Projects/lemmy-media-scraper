import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';
import { auth } from '$lib/server/auth';

const BACKEND_URL = env.GO_BACKEND_URL || 'http://localhost:8081';

const PROTECTED_PATHS = ['config'];

function isProtectedPath(path: string): boolean {
	return PROTECTED_PATHS.some(p => path === p || path.startsWith(p + '/'));
}

function validatePath(path: string): boolean {
	if (path.includes('..') || path.includes('//')) return false;
	if (!/^[a-zA-Z0-9\/_\-\.]+$/.test(path)) return false;
	return true;
}

export const GET: RequestHandler = async ({ params, url }) => {
	if (!validatePath(params.path)) {
		return new Response(JSON.stringify({ error: 'Invalid path' }), {
			status: 400,
			headers: { 'Content-Type': 'application/json' }
		});
	}
	const targetUrl = `${BACKEND_URL}/api/${params.path}${url.search}`;
	const res = await fetch(targetUrl);
	return new Response(res.body, {
		status: res.status,
		headers: {
			'content-type': res.headers.get('content-type') || 'application/json'
		}
	});
};

export const PUT: RequestHandler = async ({ params, url, request }) => {
	if (!validatePath(params.path)) {
		return new Response(JSON.stringify({ error: 'Invalid path' }), {
			status: 400,
			headers: { 'Content-Type': 'application/json' }
		});
	}
	if (isProtectedPath(params.path)) {
		const session = await auth.api.getSession({ headers: request.headers });
		if (!session) {
			return new Response(JSON.stringify({ error: 'Authentication required' }), {
				status: 401,
				headers: { 'Content-Type': 'application/json' }
			});
		}
	}
	const targetUrl = `${BACKEND_URL}/api/${params.path}${url.search}`;
	const body = await request.text();
	const res = await fetch(targetUrl, {
		method: 'PUT',
		headers: {
			'content-type': request.headers.get('content-type') || 'application/json'
		},
		body
	});
	return new Response(res.body, {
		status: res.status,
		headers: {
			'content-type': res.headers.get('content-type') || 'application/json'
		}
	});
};

export const POST: RequestHandler = async ({ params, url, request }) => {
	if (!validatePath(params.path)) {
		return new Response(JSON.stringify({ error: 'Invalid path' }), {
			status: 400,
			headers: { 'Content-Type': 'application/json' }
		});
	}
	if (isProtectedPath(params.path)) {
		const session = await auth.api.getSession({ headers: request.headers });
		if (!session) {
			return new Response(JSON.stringify({ error: 'Authentication required' }), {
				status: 401,
				headers: { 'Content-Type': 'application/json' }
			});
		}
	}
	const targetUrl = `${BACKEND_URL}/api/${params.path}${url.search}`;
	const body = await request.text();
	const res = await fetch(targetUrl, {
		method: 'POST',
		headers: {
			'content-type': request.headers.get('content-type') || 'application/json'
		},
		body
	});
	return new Response(res.body, {
		status: res.status,
		headers: {
			'content-type': res.headers.get('content-type') || 'application/json'
		}
	});
};

export const DELETE: RequestHandler = async ({ params, url, request }) => {
	if (!validatePath(params.path)) {
		return new Response(JSON.stringify({ error: 'Invalid path' }), {
			status: 400,
			headers: { 'Content-Type': 'application/json' }
		});
	}
	if (isProtectedPath(params.path)) {
		const session = await auth.api.getSession({ headers: request.headers });
		if (!session) {
			return new Response(JSON.stringify({ error: 'Authentication required' }), {
				status: 401,
				headers: { 'Content-Type': 'application/json' }
			});
		}
	}
	const targetUrl = `${BACKEND_URL}/api/${params.path}${url.search}`;
	const res = await fetch(targetUrl, { method: 'DELETE' });
	return new Response(res.body, {
		status: res.status,
		headers: {
			'content-type': res.headers.get('content-type') || 'application/json'
		}
	});
};
