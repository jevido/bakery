import { json } from '@sveltejs/kit';
import { sql } from 'bun';

export const GET = async () => {
	await sql`SELECT 1`;
	return json({ status: 'ok', time: new Date().toISOString() });
};
