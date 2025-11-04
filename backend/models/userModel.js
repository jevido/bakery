import { nanoid } from 'nanoid';
import { query, single } from '../lib/db.js';

export async function createUser({ email, passwordHash, isAdmin = false }) {
  const id = nanoid();
  await query(
    `
      INSERT INTO users (id, email, password_hash, is_admin)
      VALUES ($1, $2, $3, $4)
    `,
    [id, email, passwordHash, isAdmin]
  );
  return findUserById(id);
}

export async function findUserByEmail(email) {
  return single('SELECT * FROM users WHERE email = $1', [email]);
}

export async function findUserById(id) {
  return single('SELECT * FROM users WHERE id = $1', [id]);
}

export async function updateUserGithubLink(userId, githubId, encryptedToken) {
  await query(
    `
      INSERT INTO github_accounts (user_id, github_id, access_token)
      VALUES ($1, $2, $3)
      ON CONFLICT (user_id) DO UPDATE
      SET github_id = EXCLUDED.github_id,
          access_token = EXCLUDED.access_token,
          updated_at = NOW()
    `,
    [userId, githubId, encryptedToken]
  );
}

export async function getGithubAccount(userId) {
  return single('SELECT * FROM github_accounts WHERE user_id = $1', [userId]);
}

export async function listUsers() {
  return query(
    `
      SELECT u.id, u.email, u.is_admin, u.created_at,
             (SELECT COUNT(*) FROM deployments d WHERE d.owner_id = u.id) AS deployments
      FROM users u
      ORDER BY u.created_at DESC
    `
  );
}
