import { json, error } from '@sveltejs/kit';
import { listDatabasesForUser } from '$lib/server/models/databaseModel.js';

export const GET = async ({ locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}

	const databases = await listDatabasesForUser(locals.user.id);
	return json({ databases });
};
