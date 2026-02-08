import { betterAuth } from 'better-auth';
import Database from 'better-sqlite3';
import { env } from '$env/dynamic/private';

if (!env.BETTER_AUTH_SECRET || env.BETTER_AUTH_SECRET === 'build-placeholder') {
	throw new Error(
		'BETTER_AUTH_SECRET environment variable must be set to a secure random string (at least 32 characters). ' +
		'Generate one with: openssl rand -base64 32'
	);
}

export const auth = betterAuth({
	database: new Database(env.AUTH_DB_PATH || 'auth.db'),
	secret: env.BETTER_AUTH_SECRET,
	baseURL: env.BETTER_AUTH_URL || 'http://localhost:8080',
	emailAndPassword: {
		enabled: true
	}
});
