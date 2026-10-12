import { expect, test } from '@playwright/test';

test('load the map, search for CST, open the card and route there', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));

  await page.goto('/');
  await expect(page.locator('#map canvas.maplibregl-canvas')).toBeVisible();
  await expect(page.locator('#status-data')).toContainText('OSM data');

  const search = page.getByRole('searchbox', { name: 'Search' });
  await search.fill('CST');
  const first = page.locator('#search-results li[role="option"]').first();
  await expect(first).toContainText('CSMT');
  await first.click();

  const card = page.locator('#inspector');
  await expect(card).toBeVisible();
  await expect(card.locator('h2')).toContainText('CSMT');
  await expect(page).toHaveURL(/sel=place:\d+/);

  await card.getByRole('button', { name: 'Route here' }).click();
  await expect(page.locator('#tab-distance')).toBeVisible();
  await expect(page.getByRole('searchbox', { name: 'To place' })).toHaveValue(/CSMT/);

  expect(errors, 'uncaught errors in the page').toEqual([]);
});

test('switch to the dark theme and remember it', async ({ page }) => {
  await page.goto('/');
  const theme = page.locator('#tool-theme');
  // The button cycles system → light → dark.
  await theme.click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  await theme.click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
});

test('open a shared walking route with directions', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));
  page.on('console', (m) => m.type() === 'error' && errors.push(m.text()));

  // Andheri station to the metro station across the road, on foot.
  await page.goto('/#dist=19.11971,72.84643~19.12090,72.84810&mode=foot');
  await expect(page.locator('#tab-distance')).toBeVisible();
  await expect(page.locator('.travel-modes button[data-travel="foot"]')).toHaveAttribute('aria-pressed', 'true');
  await expect(page.locator('.route-summary')).toContainText('on foot');
  const steps = page.locator('.route-steps li');
  await expect(steps.last()).toContainText('Arrive at your destination');
  expect(await steps.count()).toBeGreaterThan(1);

  // Switching to a car recomputes and updates the link.
  await page.locator('.travel-modes button[data-travel="car"]').click();
  await expect(page.locator('.route-summary')).toContainText('by road');
  await expect(page).not.toHaveURL(/mode=foot/);
  expect(errors).toEqual([]);
});
