import { hashPassword } from '../../app/src/lib/server/auth.js';
import { findUserByEmail, createUser } from '../../app/src/lib/server/models/userModel.js';

const email = process.argv[2];
const password = process.argv[3];

if (!email || !password) {
  console.error('Usage: bun backend/scripts/create-admin.js <email> <password>');
  process.exit(1);
}

const passwordHash = await hashPassword(password);
const existing = await findUserByEmail(email);
if (existing) {
  console.log('Admin user already exists, skipping.');
  process.exit(0);
}

await createUser({ email, passwordHash, isAdmin: true });
console.log(`Admin user created for ${email}`);
