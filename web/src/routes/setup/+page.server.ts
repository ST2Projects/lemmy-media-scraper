import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import Database from 'better-sqlite3';
import { env } from '$env/dynamic/private';

export const load: PageServerLoad = async () => {
	const dbPath = env.AUTH_DB_PATH || 'auth.db';
	try {
		const db = new Database(dbPath, { readonly: true });
		const row = db.prepare('SELECT COUNT(*) as count FROM user').get() as { count: number } | undefined;
		db.close();

		if (row && row.count > 0) {
			throw redirect(302, '/login');
		}
	} catch (e) {
		// If error is a redirect, rethrow it
		if (e && typeof e === 'object' && 'status' in e) throw e;
		// Database doesn't exist yet or table not created - allow setup
	}

	return {};
};
