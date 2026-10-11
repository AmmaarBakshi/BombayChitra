import type { SearchResult } from '../api/types.ts';
import { categoryById } from '../taxonomy.ts';
import type { AppContext } from './context.ts';
import { $, clear, debounce, h } from './dom.ts';
import { formatDistance } from './format.ts';
import { categoryBadge, icon, kindBadge } from './icons.ts';

const KIND_COLOURS: Record<string, string> = {
  area: '#e8590c', locality: '#495057', line: '#1c7ed6', street: '#868e96', bridge: '#7048e8',
};

export class Search {
  private readonly input: HTMLInputElement;
  private readonly list: HTMLElement;
  private results: SearchResult[] = [];
  private active = -1;
  private abort: AbortController | null = null;

  constructor(private readonly ctx: AppContext) {
    this.input = $('#search-input') as HTMLInputElement;
    this.list = $('#search-results');
    $('.search-icon').append(icon('search', 18));

    const run = debounce(() => this.query(), 140);
    this.input.addEventListener('input', () => (this.input.value.trim().length >= 2 ? run() : this.hide()));
    this.input.addEventListener('focus', () => this.results.length && this.show());
    this.input.addEventListener('keydown', (e) => this.onKey(e));
    document.addEventListener('pointerdown', (e) => {
      if (!(e.target as Element).closest('.search')) this.hide();
    });
    document.addEventListener('keydown', (e) => {
      const target = e.target as HTMLElement;
      const typing = target.closest('input, textarea, [contenteditable]');
      if (e.key === '/' && !typing) {
        e.preventDefault();
        this.input.focus();
        this.input.select();
      }
    });
  }

  private async query(): Promise<void> {
    const q = this.input.value.trim();
    this.abort?.abort();
    this.abort = new AbortController();
    const center = this.ctx.map.map.getCenter();
    try {
      const res = await this.ctx.api.search(q, { limit: 12, near: [center.lng, center.lat] }, this.abort.signal);
      if (q !== this.input.value.trim()) return;
      this.results = res.items;
      this.active = res.items.length ? 0 : -1;
      this.render(q);
    } catch (err) {
      if ((err as Error).name !== 'AbortError') {
        this.results = [];
        clear(this.list);
        this.list.append(h('li', { class: 'empty' }, 'Search is unavailable — is the server running?'));
        this.show();
      }
    }
  }

  private render(q: string): void {
    clear(this.list);
    if (!this.results.length) {
      this.list.append(h('li', { class: 'empty', role: 'option' }, `No results for “${q}”`));
      this.show();
      return;
    }
    this.results.forEach((r, i) => {
      const badge = r.kind === 'place' && r.category ? categoryBadge(r.category, 30) : kindBadge(r.kind, KIND_COLOURS[r.kind], 30);
      const li = h('li', { role: 'option', id: `sr-${i}`, 'aria-selected': i === this.active ? 'true' : 'false' },
        badge,
        h('span', { class: 'grow' }, h('div', { class: 'title' }, highlight(r.name, q)), h('div', { class: 'sub' }, r.detail ?? describe(r))),
        r.distance_m !== undefined ? h('span', { class: 'meta' }, formatDistance(r.distance_m)) : null,
      );
      li.addEventListener('mousedown', (e) => e.preventDefault());
      li.addEventListener('click', () => this.choose(i));
      this.list.append(li);
    });
    this.show();
  }

  private onKey(e: KeyboardEvent): void {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (!this.results.length) return;
      this.active = (this.active + (e.key === 'ArrowDown' ? 1 : -1) + this.results.length) % this.results.length;
      this.list.querySelectorAll('li').forEach((li, i) => li.setAttribute('aria-selected', String(i === this.active)));
      this.input.setAttribute('aria-activedescendant', `sr-${this.active}`);
      this.list.querySelector(`#sr-${this.active}`)?.scrollIntoView({ block: 'nearest' });
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (this.active >= 0) this.choose(this.active);
    } else if (e.key === 'Escape') {
      this.hide();
      this.input.blur();
    }
  }

  private choose(i: number): void {
    const r = this.results[i];
    if (!r) return;
    this.hide();
    this.input.blur();
    if (r.bbox && r.kind !== 'place') this.ctx.map.fitBounds(r.bbox, r.kind === 'area' ? 13 : 16);
    void this.ctx.inspector.searchResult(r);
  }

  private show(): void {
    this.list.hidden = false;
    this.input.setAttribute('aria-expanded', 'true');
  }

  private hide(): void {
    this.list.hidden = true;
    this.input.setAttribute('aria-expanded', 'false');
  }
}

function describe(r: SearchResult): string {
  if (r.kind === 'place' && r.category) return categoryById.get(r.category)?.label ?? '';
  return r.kind.charAt(0).toUpperCase() + r.kind.slice(1);
}

/** Bold the matched prefix of each query word, without using innerHTML. */
function highlight(text: string, q: string): Node {
  const words = q.toLowerCase().split(/\s+/).filter(Boolean);
  const frag = document.createDocumentFragment();
  const lower = text.toLowerCase();
  let i = 0;
  while (i < text.length) {
    const word = words.find((w) => lower.startsWith(w, i) && (i === 0 || /[\s\-(,./]/.test(text[i - 1])));
    if (word) {
      frag.append(h('b', {}, text.slice(i, i + word.length)));
      i += word.length;
    } else {
      frag.append(text[i]);
      i++;
    }
  }
  return frag;
}
