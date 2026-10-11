// The POI taxonomy is defined once in config/categories.json and shared by the data pipeline, the API and the map.
import raw from '../../config/categories.json' with { type: 'json' };

export interface Category {
  id: string;
  label: string;
  icon: string;
  rank: number;
  minzoom: number;
  group: string;
  colour: string;
}

export interface CategoryGroup {
  id: string;
  label: string;
  colour: string;
  categories: Category[];
}

interface RawCategory {
  id: string;
  label: string;
  icon: string;
  rank: number;
  minzoom: number;
}

interface RawGroup {
  id: string;
  label: string;
  colour: string;
  categories: RawCategory[];
}

export const groups: CategoryGroup[] = (raw.groups as RawGroup[]).map((g) => ({
  id: g.id,
  label: g.label,
  colour: g.colour,
  categories: g.categories.map((c) => ({
    id: c.id,
    label: c.label,
    icon: c.icon,
    rank: c.rank,
    minzoom: c.minzoom,
    group: g.id,
    colour: g.colour,
  })),
}));

export const categories: Category[] = groups.flatMap((g) => g.categories);

export const categoryById = new Map(categories.map((c) => [c.id, c]));
export const groupById = new Map(groups.map((g) => [g.id, g]));

/** Distinct Maki icon names used by the taxonomy. */
export const iconNames = [...new Set(categories.map((c) => c.icon))];
