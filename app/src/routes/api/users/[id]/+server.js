import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import {
  updateUser,
  setUserPassword,
  deleteUser,
  getInitialUserId,
  listUsers
} from '$lib/server/models/userModel.js';
import { hashPassword } from '$lib/server/auth.js';

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

export const PATCH = async ({ params, request, locals }) => {
  requireAdmin(locals);
  const id = params.id;
  const body = await request.json().catch(() => ({}));
  const parsed = updateUserSchema.safeParse(body);
  if (!parsed.success) {
    throw error(422, 'Validation failed');
  }
  const data = parsed.data;

  if (Object.prototype.hasOwnProperty.call(data, 'password')) {
    const passwordHash = await hashPassword(data.password);
    await setUserPassword(id, passwordHash);
  }

  const updates = {};
  if (Object.prototype.hasOwnProperty.call(data, 'email')) {
    updates.email = data.email;
  }
  if (Object.prototype.hasOwnProperty.call(data, 'isAdmin')) {
    updates.isAdmin = data.isAdmin;
  }
  if (Object.keys(updates).length) {
    await updateUser(id, updates);
  }

  const [users, initialUserId] = await Promise.all([listUsers(), getInitialUserId()]);
  const user = users.find((row) => row.id === id);
  if (!user) {
    throw error(404, 'User not found');
  }
  return json({ user: normalizeUser(user, initialUserId) });
};

export const DELETE = async ({ params, locals }) => {
  requireAdmin(locals);
  const id = params.id;
  const initialUserId = await getInitialUserId();
  if (id === initialUserId) {
    throw error(400, 'The initial administrator account cannot be deleted');
  }
  await deleteUser(id);
  return json({ ok: true });
};
