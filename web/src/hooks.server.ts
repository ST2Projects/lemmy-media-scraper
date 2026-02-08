import { auth } from '$lib/server/auth';
import { redirect, type Handle } from '@sveltejs/kit';

export const handle: Handle = async ({ event, resolve }) => {
	const session = await auth.api.getSession({ headers: event.request.headers });
	event.locals.session = session;

	// Protect settings page - require authentication
	if (event.url.pathname.startsWith('/settings')) {
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
