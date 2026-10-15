import { describe, expect, it } from 'vitest';
import { reportUrl } from '../src/ui/report.ts';

describe('reportUrl', () => {
  it('opens a new OpenStreetMap note at the place, zoomed in', () => {
    const url = new URL(reportUrl(72.8355192, 18.9398564));
    expect(url.origin + url.pathname).toBe('https://www.openstreetmap.org/note/new');
    expect(url.searchParams.get('lat')).toBe('18.939856');
    expect(url.searchParams.get('lon')).toBe('72.835519');
    expect(url.hash).toBe('#map=19/18.939856/72.835519');
  });
});
