export interface WorldTheme {
  bg: string;
  fg: string;
  accent: string;
  dim: string;
  border: string;
  error: string;
  success: string;
}

export interface WorldMeta {
  name: string;
  tagline: string;
  theme: WorldTheme;
  ui_profile?: string;
  map_rooms?: Array<{ id: string; name: string; biome: string; x: number; y: number }>;
}

/**
 * Applies a world's theme to the document root as CSS custom properties.
 * Only non-empty values are written to avoid clobbering existing defaults.
 */
export function applyTheme(meta: WorldMeta): void {
  const root = document.documentElement;
  const t = meta.theme;
  const pairs: [string, string][] = [
    ['--bg',      t.bg],
    ['--fg',      t.fg],
    ['--accent',  t.accent],
    ['--dim',     t.dim],
    ['--border',  t.border],
    ['--error',   t.error],
    ['--success', t.success],
  ];
  for (const [prop, val] of pairs) {
    if (val) {
      root.style.setProperty(prop, val);
    }
  }
}
