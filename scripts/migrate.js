import { migrate } from '../app/src/lib/server/migrate.js';

migrate()
  .then(() => {
    console.log('Migrations applied');
    process.exit(0);
  })
  .catch((error) => {
    console.error('Migration failed', error);
    process.exit(1);
  });
