import { json } from '@sveltejs/kit';
import { destroySession } from '$lib/server/auth.js';

export const POST = async ({ cookies, locals }) => {
  const token = cookies.get('bakery_session');
  if (token) {
    await destroySession(token);
  }
  cookies.delete('bakery_session', { path: '/' });
  locals.user = null;
  return json({ ok: true });
};
