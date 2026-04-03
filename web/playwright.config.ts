import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  outputDir: './test-results',
  workers: 1,
  use: {
    baseURL: 'http://localhost:8091',
    screenshot: 'on',
    viewport: { width: 1280, height: 800 },
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
  webServer: {
    command: 'go run . -serve -world blockhaven -port 8091',
    cwd: '../',
    url: 'http://localhost:8091',
    reuseExistingServer: false,
    timeout: 30_000,
  },
  reporter: [
    ['json', { outputFile: './test-results/results.json' }],
    ['html', { outputFolder: './test-results-html', open: 'never' }],
  ],
});
