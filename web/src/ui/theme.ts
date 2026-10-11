import type { ThemeName } from '../map/style/index.ts';
import { storageGet, storageSet } from './dom.ts';

export type ThemePreference = 'system' | 'light' | 'dark';

const KEY = 'bc-theme';
const media = window.matchMedia('(prefers-color-scheme: dark)');

export class ThemeManager {
  private preference: ThemePreference;
  private listeners: ((theme: ThemeName) => void)[] = [];

  constructor() {
    const saved = storageGet(KEY);
    this.preference = saved === 'light' || saved === 'dark' ? saved : 'system';
    media.addEventListener('change', () => this.preference === 'system' && this.apply());
    this.apply();
  }

  get resolved(): ThemeName {
    if (this.preference === 'system') return media.matches ? 'dark' : 'light';
    return this.preference;
  }

  get current(): ThemePreference {
    return this.preference;
  }

  /** Cycles system → light → dark. */
  cycle(): ThemePreference {
    const order: ThemePreference[] = ['system', 'light', 'dark'];
    this.set(order[(order.indexOf(this.preference) + 1) % order.length]);
    return this.preference;
  }

  set(pref: ThemePreference): void {
    this.preference = pref;
    storageSet(KEY, pref);
    this.apply();
  }

  onChange(fn: (theme: ThemeName) => void): void {
    this.listeners.push(fn);
  }

  private apply(): void {
    const theme = this.resolved;
    document.documentElement.dataset.theme = theme;
    document.querySelectorAll('meta[name="theme-color"]').forEach((m) => m.setAttribute('content', theme === 'dark' ? '#11151b' : '#f3efe7'));
    this.listeners.forEach((fn) => fn(theme));
  }
}
