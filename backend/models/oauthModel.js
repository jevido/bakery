import { nanoid } from 'nanoid';
import { query, single } from '../lib/db.js';

export async function createOAuthState(userId, provider, state) {
  await query(
    `
      INSERT INTO oauth_states (id, user_id, provider, state)
      VALUES ($1, $2, $3, $4)
    `,
    [nanoid(), userId, provider, state]
  );
}

export async function consumeOAuthState(state, provider) {
  const record = await single(
    `
      DELETE FROM oauth_states
      WHERE state = $1 AND provider = $2
      RETURNING *
    `,
    [state, provider]
  );
  return record;
}
