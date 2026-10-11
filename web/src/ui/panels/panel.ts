/** A side-panel tab. Panels render lazily on first show and may refresh when the selected area changes. */
export interface Panel {
  id: string;
  label: string;
  icon: string;
  render(body: HTMLElement): void | Promise<void>;
  onAreaChange?(area: string | null): void;
}
