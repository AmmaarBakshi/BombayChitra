/// <reference types="vite/client" />

declare module 'virtual:bc-icons' {
  const icons: Record<string, { d: string; evenodd?: boolean }>;
  export default icons;
}

interface ImportMetaEnv {
  /** Origin of the BombayChitra server when the web app is hosted elsewhere (default: same origin). */
  readonly VITE_API_BASE?: string;
}
