import { defineConfig } from 'vitest/config';

// Unit tests run in Node, without the app's build plugins (docs site, icons, precompression).
export default defineConfig({
  test: {
    include: ['test/**/*.test.ts'],
    environment: 'node',
  },
});
