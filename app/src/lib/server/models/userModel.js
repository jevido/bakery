import { nanoid } from 'nanoid';
import { sql } from 'bun';

export async function createUser({ email, passwordHash, isAdmin = false }) {
  const id = nanoid();
  await sql`
    INSERT INTO users (id, email, password_hash, is_admin)
    VALUES (${id}, ${email}, ${passwordHash}, ${isAdmin})
  `;
  return findUserById(id);
}

export async function findUserByEmail(email) {
  const rows = await sql`SELECT * FROM users WHERE email = ${email}`;
  return rows[0] ?? null;
}

export async function findUserById(id) {
  const rows = await sql`SELECT * FROM users WHERE id = ${id}`;
  return rows[0] ?? null;
}

export async function updateUserGithubLink(userId, githubId, encryptedToken) {
  await sql`
    INSERT INTO github_accounts (user_id, github_id, access_token)
    VALUES (${userId}, ${githubId}, ${encryptedToken})
    ON CONFLICT (user_id) DO UPDATE
    SET github_id = EXCLUDED.github_id,
        access_token = EXCLUDED.access_token,
        updated_at = NOW()
  `;
}

export async function getGithubAccount(userId) {
  const rows = await sql`SELECT * FROM github_accounts WHERE user_id = ${userId}`;
  return rows[0] ?? null;
}

export async function listUsers() {
  return sql`
    SELECT u.id, u.email, u.is_admin, u.created_at,
           (SELECT COUNT(*) FROM deployments d WHERE d.owner_id = u.id) AS deployments
    FROM users u
    ORDER BY u.created_at DESC
  `;
}

export async function updateUser(id, updates) {
  const payload = {};
  if (Object.prototype.hasOwnProperty.call(updates, 'email')) {
    payload.email = updates.email;
  }
  if (Object.prototype.hasOwnProperty.call(updates, 'isAdmin')) {
    payload.is_admin = updates.isAdmin;
  }
  if (!Object.keys(payload).length) {
    return findUserById(id);
  }
  const rows = await sql`
    UPDATE users
    SET ${sql(payload)}, updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
  return rows[0] ?? null;
}

export async function setUserPassword(id, passwordHash) {
  await sql`
    UPDATE users
    SET password_hash = ${passwordHash}, updated_at = NOW()
    WHERE id = ${id}
  `;
}

export async function deleteUser(id) {
  await sql`DELETE FROM users WHERE id = ${id}`;
}

export async function getInitialUserId() {
  const rows = await sql`SELECT id FROM users ORDER BY created_at ASC LIMIT 1`;
  return rows[0]?.id ?? null;
}
