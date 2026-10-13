import makiIcons from 'virtual:bc-icons';
import { categoryById } from '../taxonomy.ts';
import { h, svg } from './dom.ts';

// Interface icons: 24×24, stroked with currentColor.
const UI: Record<string, string> = {
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>',
  explore: '<circle cx="12" cy="12" r="9"/><path d="m15.5 8.5-2 5-5 2 2-5z"/>',
  layers: '<path d="m12 3 9 5-9 5-9-5z"/><path d="m3 13 9 5 9-5"/>',
  train: '<rect x="5" y="3" width="14" height="14" rx="3"/><path d="M5 11h14M9 21l1.5-4M15 21l-1.5-4"/><circle cx="9" cy="14" r=".6"/><circle cx="15" cy="14" r=".6"/>',
  bridge: '<path d="M2 16h20M4 16V9M20 16V9M2 9c4 0 6 3 10 3s6-3 10-3M8 16v-3.5M12 16v-4M16 16v-3.5"/>',
  building: '<path d="M4 21V5l8-2v18M12 7l8 2v12M2 21h20M7 8h2M7 12h2M7 16h2M15 12h2M15 16h2"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
  moon: '<path d="M20 14.5A8.5 8.5 0 1 1 9.5 4a7 7 0 0 0 10.5 10.5z"/>',
  system: '<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>',
  cube: '<path d="m12 3 8 4.5v9L12 21l-8-4.5v-9z"/><path d="m4 7.5 8 4.5 8-4.5M12 12v9"/>',
  ruler: '<path d="m3 17 14-14 4 4L7 21z"/><path d="m7 13 2 2M10 10l2 2M13 7l2 2"/>',
  pin: '<path d="M12 21s-7-6.2-7-12a7 7 0 0 1 14 0c0 5.8-7 12-7 12z"/><circle cx="12" cy="9" r="2.5"/>',
  locate: '<circle cx="12" cy="12" r="3"/><circle cx="12" cy="12" r="8"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2"/>',
  close: '<path d="M6 6l12 12M18 6 6 18"/>',
  copy: '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>',
  link: '<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1"/><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/>',
  external: '<path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>',
  chevron: '<path d="m9 6 6 6-6 6"/>',
  back: '<path d="m15 6-6 6 6 6"/>',
  phone: '<path d="M5 4h4l2 5-2.5 1.5a11 11 0 0 0 5 5L15 13l5 2v4a2 2 0 0 1-2 2A16 16 0 0 1 3 6a2 2 0 0 1 2-2"/>',
  globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  route: '<circle cx="6" cy="19" r="2"/><circle cx="18" cy="5" r="2"/><path d="M8 19h8a3 3 0 0 0 0-6H8a3 3 0 0 1 0-6h8"/>',
  menu: '<path d="M4 7h16M4 12h16M4 17h16"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5v.5"/>',
  docs: '<path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5M9 12h7M9 16h7"/>',
  code: '<path d="m8 8-5 4 5 4M16 8l5 4-5 4M14 5l-4 14"/>',
  github: '<path d="M9 19c-4.3 1.4-4.3-2.5-6-3m12 5v-3.5c0-1 .1-1.4-.5-2 2.8-.3 5.5-1.4 5.5-6a4.6 4.6 0 0 0-1.3-3.2 4.2 4.2 0 0 0-.1-3.2s-1.1-.3-3.5 1.3a12.3 12.3 0 0 0-6.2 0C6.5 2.8 5.4 3.1 5.4 3.1a4.2 4.2 0 0 0-.1 3.2A4.6 4.6 0 0 0 4 9.5c0 4.6 2.7 5.7 5.5 6-.6.6-.6 1.2-.5 2V21"/>',
  wheelchair: '<circle cx="12" cy="4" r="1.6"/><path d="M12 7v6h5l2 5M12 10h4M9 9.5a6 6 0 1 0 7.5 8"/>',
  directions: '<path d="M12 2 2 12l10 10 10-10z"/><path d="M9 14v-2a2 2 0 0 1 2-2h4M13 8l2 2-2 2"/>',
  sidebar: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16M5.5 8h1.5M5.5 11h1.5"/>',
  swap: '<path d="M7 4v16M7 4 4 7M7 4l3 3M17 20V4M17 20l-3-3M17 20l3-3"/>',
  crosshair: '<circle cx="12" cy="12" r="7"/><path d="M12 2v4M12 18v4M2 12h4M18 12h4"/>',
  car: '<path d="M5 16V11l2-5h10l2 5v5M3 11h18M5 16h14v3H5zM7.5 13.5h1M15.5 13.5h1"/>',
  scooter: '<circle cx="6" cy="17" r="2.5"/><circle cx="18" cy="17" r="2.5"/><path d="M8.5 17h7l2-6h-3M14 7h3l1 4M6 14.5 9 9h4"/>',
  walk: '<circle cx="13" cy="4.5" r="1.8"/><path d="m9 21 2.5-6.5L14 17v4M11.5 14.5 12.5 9l3 3 3 1M12.5 9 9 10.5 7.5 14"/>',
  arrow: '<path d="M12 20V5M6 11l6-6 6 6"/>',
  flag: '<path d="M5 21V4M5 4h11l-2 4 2 4H5"/>',
  toll: '<path d="M4 20V8l8-4 8 4v12M4 12h16M9 20v-4h6v4"/>',
  drop: '<path d="M12 3s6 6.6 6 11a6 6 0 0 1-12 0c0-4.4 6-11 6-11z"/>',
};

export function icon(name: keyof typeof UI | string, size = 18): SVGElement {
  return svg(
    `<svg class="i" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${UI[name] ?? UI.info}</svg>`,
  );
}

/** A Maki glyph as inline SVG (15×15 grid), filled with currentColor. */
export function makiSvg(name: string, size = 15): SVGElement {
  const p = makiIcons[name] ?? makiIcons.marker ?? { d: 'M7.5 1a4.5 4.5 0 0 0-4.5 4.5c0 3.4 4.5 8.5 4.5 8.5s4.5-5.1 4.5-8.5A4.5 4.5 0 0 0 7.5 1z' };
  return svg(
    `<svg width="${size}" height="${size}" viewBox="0 0 15 15" fill="currentColor" aria-hidden="true"><path ${p.evenodd ? 'fill-rule="evenodd" ' : ''}d="${p.d}"/></svg>`,
  );
}

/** Round category badge (group colour + white glyph) for lists and cards. */
export function categoryBadge(categoryId: string, size = 26): HTMLElement {
  const c = categoryById.get(categoryId);
  const badge = h('span', { class: 'badge', style: `--badge:${c?.colour ?? '#888'};width:${size}px;height:${size}px` });
  badge.append(makiSvg(c?.icon ?? 'marker', Math.round(size * 0.58)));
  return badge;
}

/** Badge for non-POI search results. */
export function kindBadge(kind: string, colour = 'var(--muted)', size = 26): HTMLElement {
  const names: Record<string, string> = { area: 'explore', locality: 'pin', line: 'train', street: 'route', bridge: 'bridge', building: 'building' };
  const badge = h('span', { class: 'badge badge-outline', style: `--badge:${colour};width:${size}px;height:${size}px` });
  badge.append(icon(names[kind] ?? 'pin', Math.round(size * 0.6)));
  return badge;
}
