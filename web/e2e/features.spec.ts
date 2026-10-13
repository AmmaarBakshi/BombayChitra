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
