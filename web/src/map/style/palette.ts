// Map colour palettes. Every colour used by the style comes from here, so a new theme is a new palette.

export type ThemeName = 'light' | 'dark';

export interface Palette {
  name: ThemeName;
  land: string;
  water: string;
  ocean: string;
  waterway: string;
  waterLabel: string;
  landuse: Record<string, string>;
  protectedOutline: string;
  building: string;
  buildingOutline: string;
  buildingLow: string;
  buildingMid: string;
  buildingHigh: string;
  road: Record<'motorway' | 'trunk' | 'primary' | 'secondary' | 'tertiary' | 'minor' | 'service' | 'pedestrian' | 'track' | 'path' | 'construction', string>;
  roadCasing: Record<'motorway' | 'trunk' | 'primary' | 'secondary' | 'tertiary' | 'minor' | 'service' | 'pedestrian' | 'track', string>;
  bridgeCasing: string;
  tunnelFill: string;
  rail: string;
  railDash: string;
  railService: string;
  transitCasing: string;
  /** Casing for line colours with a `tone` that would vanish on this map (see contrast.ts). */
  transitToneCasing: string;
  aeroway: string;
  apron: string;
  platform: string;
  boundary: string;
  boundaryMajor: string;
  areaHighlight: string;
  text: string;
  textMuted: string;
  textFaint: string;
  halo: string;
  roadLabel: string;
  poiStroke: string;
  selection: string;
  bridgeHighlight: Record<'road' | 'rail' | 'metro' | 'foot' | 'construction' | 'aqueduct', string>;
}

export const light: Palette = {
  name: 'light',
  land: '#f3efe7',
  water: '#a6d1ea',
  ocean: '#9ccae6',
  waterway: '#8cc0df',
  waterLabel: '#2f6f99',
  landuse: {
    protected: '#d9ecd0',
    forest: '#bddbae',
    mangrove: '#a3d2b4',
    wetland: '#c6e4d8',
    salt_pan: '#dfeaf0',
    grass: '#d8ebc6',
    farmland: '#ebe8d2',
    beach: '#f4e5bd',
    rock: '#e1dbd2',
    quarry: '#dcd4c9',
    aquaculture: '#cbe4ee',
    residential: '#ece7de',
    commercial: '#f1e3df',
    industrial: '#e8e1ec',
    port: '#e3e0ec',
    railway: '#e6e1da',
    military: '#efdfd6',
    construction: '#ebe2d3',
    landfill: '#e2dccb',
    airport: '#e7e5ef',
    parking: '#e9e7e1',
    religious: '#ebe4d8',
    park: '#c9e6bd',
    golf: '#cbe7bd',
    zoo: '#d3eacb',
    sports: '#cfe9cd',
    playground: '#dcefd4',
    cemetery: '#dae6d1',
    education: '#f3ead0',
    hospital: '#f6dddd',
  },
  protectedOutline: '#8fbf7d',
  building: '#dfd8cc',
  buildingOutline: '#cdc5b7',
  buildingLow: '#e4ddd1',
  buildingMid: '#d6cdbf',
  buildingHigh: '#bfb3a2',
  road: {
    motorway: '#f4ad5b',
    trunk: '#f8c77b',
    primary: '#fce1a0',
    secondary: '#ffffff',
    tertiary: '#ffffff',
    minor: '#ffffff',
    service: '#fbfaf7',
    pedestrian: '#f2eee6',
    track: '#c9b99b',
    path: '#a89a85',
    construction: '#ddd5c7',
  },
  roadCasing: {
    motorway: '#d4843a',
    trunk: '#d79f58',
    primary: '#d4b36e',
    secondary: '#cfc5b3',
    tertiary: '#d3cbbd',
    minor: '#d8d1c5',
    service: '#ddd7cc',
    pedestrian: '#d8d0c2',
    track: '#e5dccb',
  },
  bridgeCasing: '#8d8270',
  tunnelFill: '#f4f1eb',
  rail: '#9a938a',
  railDash: '#ffffff',
  railService: '#bdb6ac',
  transitCasing: '#ffffff',
  transitToneCasing: '#6b5f45',
  aeroway: '#d4d1df',
  apron: '#e2e0ea',
  platform: '#d9d3cc',
  boundary: '#a99fbd',
  boundaryMajor: '#8e83a8',
  areaHighlight: '#e8590c',
  text: '#2c2925',
  textMuted: '#5c564d',
  textFaint: '#857e73',
  halo: '#fbf9f4',
  roadLabel: '#4a453d',
  poiStroke: '#ffffff',
  selection: '#1c7ed6',
  bridgeHighlight: {
    road: '#7048e8',
    rail: '#a0522d',
    metro: '#0c8599',
    foot: '#2b8a3e',
    construction: '#e67700',
    aqueduct: '#1971c2',
  },
};

export const dark: Palette = {
  name: 'dark',
  land: '#11151b',
  water: '#0e2638',
  ocean: '#0b2132',
  waterway: '#15384f',
  waterLabel: '#6fa3c7',
  landuse: {
    protected: '#132419',
    forest: '#13281b',
    mangrove: '#0f2a29',
    wetland: '#10262a',
    salt_pan: '#162129',
    grass: '#16251a',
    farmland: '#191d17',
    beach: '#2a2618',
    rock: '#1c1f23',
    quarry: '#1e1d1b',
    aquaculture: '#0f242c',
    residential: '#151a21',
    commercial: '#1c1820',
    industrial: '#1a1826',
    port: '#181a27',
    railway: '#1a1b20',
    military: '#241a1b',
    construction: '#201e18',
    landfill: '#1f1d16',
    airport: '#1b1c27',
    parking: '#191c22',
    religious: '#1e1c1a',
    park: '#14281b',
    golf: '#15291c',
    zoo: '#15271b',
    sports: '#14271c',
    playground: '#18291c',
    cemetery: '#16211a',
    education: '#221f16',
    hospital: '#2a181b',
  },
  protectedOutline: '#2f5a3a',
  building: '#1e252e',
  buildingOutline: '#2a333e',
  buildingLow: '#222a35',
  buildingMid: '#2b3644',
  buildingHigh: '#3a4a5e',
  road: {
    motorway: '#c0782f',
    trunk: '#9b7136',
    primary: '#6b6147',
    secondary: '#3f4653',
    tertiary: '#363d49',
    minor: '#2f3540',
    service: '#282e37',
    pedestrian: '#2a2f37',
    track: '#3d3a33',
    path: '#4d5462',
    construction: '#3a3833',
  },
  roadCasing: {
    motorway: '#5e3a14',
    trunk: '#4d3818',
    primary: '#3a3424',
    secondary: '#191d24',
    tertiary: '#181c22',
    minor: '#161a20',
    service: '#14181d',
    pedestrian: '#191d23',
    track: '#1d1c19',
  },
  bridgeCasing: '#05070a',
  tunnelFill: '#1d2129',
  rail: '#5a6170',
  railDash: '#11151b',
  railService: '#3a404b',
  transitCasing: '#0b0e12',
  transitToneCasing: '#aab4c3',
  aeroway: '#2c2e3e',
  apron: '#1d1f2a',
  platform: '#2a2f38',
  boundary: '#5c5675',
  boundaryMajor: '#7a7398',
  areaHighlight: '#ff922b',
  text: '#dfe3ea',
  textMuted: '#a8b0bc',
  textFaint: '#7d8694',
  halo: '#0d1015',
  roadLabel: '#b3bcc8',
  poiStroke: '#0d1015',
  selection: '#4dabf7',
  bridgeHighlight: {
    road: '#9775fa',
    rail: '#d08c60',
    metro: '#22b8cf',
    foot: '#51cf66',
    construction: '#ffa94d',
    aqueduct: '#4dabf7',
  },
};

export const palettes: Record<ThemeName, Palette> = { light, dark };
