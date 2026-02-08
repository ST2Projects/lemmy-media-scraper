import { auth, authReady } from '$lib/server/auth';
import { redirect, type Handle } from '@sveltejs/kit';
import Database from 'better-sqlite3';
import { env } from '$env/dynamic/private';

function needsSetup(): boolean {
	const dbPath = env.AUTH_DB_PATH || 'auth.db';
	try {
		const db = new Database(dbPath, { readonly: true });
		const row = db.prepare('SELECT COUNT(*) as count FROM user').get() as { count: number } | undefined;
		db.close();
		return !row || row.count === 0;
	} catch {
		// Database doesn't exist yet or table not created - needs setup
		return true;
	}
}

export const handle: Handle = async ({ event, resolve }) => {
	// Ensure auth tables exist before handling requests
	await authReady;
	const session = await auth.api.getSession({ headers: event.request.headers });
	event.locals.session = session;

	const pathname = event.url.pathname;

	// Allow auth API routes and setup page to pass through
	const isAuthRoute = pathname.startsWith('/api/auth');
	const isSetupPage = pathname === '/setup';

	// Force setup if no admin account exists
	if (!isAuthRoute && !isSetupPage && needsSetup()) {
		throw redirect(302, '/setup');
	}

	// Protect settings page - require authentication
	if (pathname.startsWith('/settings')) {
		if (!session) {
			throw redirect(302, '/login');
		}
	}

	const response = await resolve(event);

	// Security headers
	response.headers.set('X-Frame-Options', 'DENY');
	response.headers.set('X-Content-Type-Options', 'nosniff');
	response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
	response.headers.set('Permissions-Policy', 'geolocation=(), microphone=(), camera=()');
	response.headers.set('Content-Security-Policy',
		"default-src 'self'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"media-src 'self' blob:; " +
		"connect-src 'self' ws: wss:; " +
		"font-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'"
	);

	return response;
};
