import makiIcons from 'virtual:bc-icons';
import { categoryById } from '../taxonomy.ts';
import type { ThemeName } from './style/index.ts';

const RADIUS = 10;
const STROKE = 1.75;

/**
 * Renders the map badge for a POI category: a disc in the group colour with a white Maki glyph and a contrasting
 * ring. Rendered at device resolution so icons stay crisp on high-DPI screens.
 */
export function renderBadge(categoryId: string, theme: ThemeName, pixelRatio = Math.min(3, Math.max(1, window.devicePixelRatio || 1))): { image: ImageData; pixelRatio: number } | null {
  const category = categoryById.get(categoryId);
  if (!category) return null;
  const glyph = makiIcons[category.icon];
  const size = Math.ceil((RADIUS + STROKE) * 2 + 2) * pixelRatio;
  const canvas = document.createElement('canvas');
  canvas.width = canvas.height = size;
  const ctx = canvas.getContext('2d');
  if (!ctx) return null;
  ctx.scale(pixelRatio, pixelRatio);
  const c = size / pixelRatio / 2;

  ctx.beginPath();
  ctx.arc(c, c, RADIUS + STROKE, 0, Math.PI * 2);
  ctx.fillStyle = theme === 'dark' ? 'rgba(10,13,18,0.95)' : 'rgba(255,255,255,0.97)';
  ctx.fill();

  ctx.beginPath();
  ctx.arc(c, c, RADIUS, 0, Math.PI * 2);
  ctx.fillStyle = category.colour;
  ctx.fill();

  if (glyph) {
    const scale = (RADIUS * 1.18) / 15;
    ctx.save();
    ctx.translate(c - 7.5 * scale, c - 7.5 * scale);
    ctx.scale(scale, scale);
    ctx.fillStyle = '#ffffff';
    ctx.fill(new Path2D(glyph.d), glyph.evenodd ? 'evenodd' : 'nonzero');
    ctx.restore();
  }
  return { image: ctx.getImageData(0, 0, size, size), pixelRatio };
}
