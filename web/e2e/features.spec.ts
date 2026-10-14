import { expect, test } from '@playwright/test';

test('switch the interface to Marathi and back', async ({ page }) => {
  await page.goto('/');
  const lang = page.locator('#tool-lang');
  await expect(lang).toHaveText('EN');
  await lang.click(); // reloads in Marathi
  await expect(page.locator('html')).toHaveAttribute('lang', 'mr');
  await expect(page.getByRole('tab', { name: 'अंतर' })).toBeVisible();
  await expect(page.getByRole('searchbox')).toHaveAttribute('placeholder', /शोधा/);
  await page.locator('#tool-lang').click(); // Hindi
  await expect(page.locator('html')).toHaveAttribute('lang', 'hi');
  await page.locator('#tool-lang').click(); // back to English
  await expect(page.getByRole('tab', { name: 'Distance' })).toBeVisible();
});

test('move between panel tabs with the keyboard', async ({ page }) => {
  await page.goto('/');
  const explore = page.getByRole('tab', { name: 'Explore' });
  await explore.click();
  await expect(explore).toHaveAttribute('aria-selected', 'true');
  await page.keyboard.press('ArrowRight');
  const layers = page.getByRole('tab', { name: 'Layers' });
  await expect(layers).toBeFocused();
  await expect(layers).toHaveAttribute('aria-selected', 'true');
  await expect(page.locator('#tab-layers')).toBeVisible();
  await page.keyboard.press('End');
  await expect(page.getByRole('tab', { name: 'Distance' })).toHaveAttribute('aria-selected', 'true');
  await page.keyboard.press('Home');
  await expect(explore).toHaveAttribute('aria-selected', 'true');
});

test('show the heritage overlay', async ({ page }) => {
  await page.goto('/#map=15/18.9399/72.8355');
  await page.getByRole('tab', { name: 'Layers' }).click();
  await page.getByText('Heritage sites', { exact: true }).click();
  // The World Heritage Site at CSMT is drawn at the centre of the view.
  await page.waitForTimeout(1500);
  const ids = await page.evaluate(() => {
    const map = (window as unknown as { bombaychitra: { map: { queryRenderedFeatures: (o: object) => { properties: Record<string, unknown> }[] } } }).bombaychitra.map;
    return map.queryRenderedFeatures({ layers: ['heritage-sites'] }).map((f) => f.properties.id);
  });
  expect(ids).toContain('csmt');
});

test('plan a train journey and open a station from it', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));

  // Dadar to Ghatkopar: one Central line train.
  await page.goto('/#dist=19.0190,72.8430~19.0864,72.9087&mode=transit');
  await expect(page.locator('.travel-modes button[data-travel="transit"]')).toHaveAttribute('aria-pressed', 'true');
  await expect(page.locator('.route-summary')).toContainText('1 train');
  const rides = page.locator('.journey-leg.ride');
  await expect(rides).toHaveCount(1);
  await expect(rides.first()).toContainText('Central Line');

  // Metro only: Line 3, then Line 1 from Marol Naka.
  await page.locator('.ride-modes button[data-ride="train"]').click();
  await expect(page).toHaveURL(/ride=metro/);
  await expect(page.locator('.route-summary')).toContainText('2 trains');
  await expect(rides.first()).toContainText('Line 3');
  await expect(rides.last()).toContainText('Line 1');

  // The station to board at opens its card.
  await rides.first().getByRole('button', { name: /^Board at/ }).click();
  await expect(page.locator('#inspector h2')).toBeVisible();
  await expect(page).toHaveURL(/sel=place:\d+/);
  expect(errors).toEqual([]);
});

test.describe('near me', () => {
  // Standing outside Andheri station.
  test.use({ geolocation: { latitude: 19.1197, longitude: 72.8464 }, permissions: ['geolocation'] });

  test('find the nearest station and get directions there', async ({ page }) => {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    await page.goto('/');
    await page.locator('.near-me button[data-near="station"]').click();
    const first = page.locator('.near-me .near-item').first();
    await expect(first).toContainText('Andheri');
    await expect(first).toContainText('min walk');

    await first.getByRole('button', { name: 'Directions' }).click();
    await expect(page.locator('#tab-distance')).toBeVisible();
    await expect(page.getByRole('searchbox', { name: 'From place' })).toHaveValue('You are here');
    await expect(page.locator('.travel-modes button[data-travel="foot"]')).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('.route-summary')).toContainText('on foot');
    expect(errors).toEqual([]);
  });
});
