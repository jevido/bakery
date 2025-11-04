import { nanoid } from 'nanoid';
import { sql } from 'bun';

export async function createOAuthState(userId, provider, state) {
  await sql`
    INSERT INTO oauth_states (id, user_id, provider, state)
    VALUES (${nanoid()}, ${userId}, ${provider}, ${state})
  `;
}

export async function consumeOAuthState(state, provider) {
  const rows = await sql`
    DELETE FROM oauth_states
    WHERE state = ${state} AND provider = ${provider}
    RETURNING *
  `;
  return rows[0] ?? null;
}
