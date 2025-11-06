import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import {
	listUsers,
	findUserByEmail,
	createUser,
	getInitialUserId
} from '$lib/server/models/userModel.js';
import { hashPassword } from '$lib/server/auth.js';

const createUserSchema = z.object({
	email: z.string().email(),
	password: z.string().min(8),
	isAdmin: z.boolean().default(false)
});

const updateUserSchema = z
	.object({
		email: z.string().email().optional(),
		password: z.string().min(8).optional(),
		isAdmin: z.boolean().optional()
	})
	.refine((value) => Object.keys(value).length > 0, {
		message: 'No updates provided'
	});

function requireAdmin(locals) {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	if (!locals.user.is_admin) {
		throw error(403, 'Forbidden');
	}
}

function normalizeUser(user, initialUserId) {
	if (!user) return null;
	return {
		id: user.id,
		email: user.email,
		is_admin: user.is_admin,
		created_at: user.created_at,
		deployments: Number(user.deployments ?? 0),
		is_initial: user.id === initialUserId
	};
}

export const GET = async ({ locals }) => {
	requireAdmin(locals);
	const [users, initialUserId] = await Promise.all([listUsers(), getInitialUserId()]);
	const normalized = users.map((user) => normalizeUser(user, initialUserId));
	return json({ users: normalized });
};

export const POST = async ({ request, locals }) => {
	requireAdmin(locals);
	const body = await request.json().catch(() => ({}));
	const parsed = createUserSchema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Validation failed');
	}
	const data = parsed.data;
	const existing = await findUserByEmail(data.email);
	if (existing) {
		throw error(409, 'User already exists');
	}
	const passwordHash = await hashPassword(data.password);
	const created = await createUser({ email: data.email, passwordHash, isAdmin: data.isAdmin });
	const [users, initialUserId] = await Promise.all([listUsers(), getInitialUserId()]);
	const normalized = users.find((user) => user.id === created.id);
	return json({ user: normalizeUser(normalized, initialUserId) });
};
