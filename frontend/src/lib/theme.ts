// Custom theme state: mode (system / light / dark) plus an accent colour.
// Persisted in localStorage and applied to both the Naive UI config provider
// and the raw DOM (for our own CSS variables).
import { computed, ref, watch } from 'vue'
import { darkTheme, type GlobalThemeOverrides } from 'naive-ui'

export type ThemeMode = 'system' | 'light' | 'dark'

const MODE_KEY = 'nb.themeMode'
const ACCENT_KEY = 'nb.accentColor'

// Default accent matches Naive UI's default primary so an untouched app is
// indistinguishable from before this feature existed.
export const defaultAccent = '#2080f0'

// A handful of pleasant presets offered in Settings.
export const accentPresets = [
  '#2080f0',
  '#18a058',
  '#7c5cff',
  '#f0a020',
  '#d03050',
  '#0ea5e9',
]

function readStoredMode(): ThemeMode {
  const v = localStorage.getItem(MODE_KEY)
  return v === 'light' || v === 'dark' ? v : 'system'
}

const media = typeof window !== 'undefined' && window.matchMedia
  ? window.matchMedia('(prefers-color-scheme: dark)')
  : null
const prefersDark = ref(media ? media.matches : true)

export const themeMode = ref<ThemeMode>(readStoredMode())
export const accentColor = ref<string>(localStorage.getItem(ACCENT_KEY) || defaultAccent)

export const isDark = computed(() =>
  themeMode.value === 'system' ? prefersDark.value : themeMode.value === 'dark',
)

export const naiveTheme = computed(() => (isDark.value ? darkTheme : null))

export const themeOverrides = computed<GlobalThemeOverrides>(() => {
  const c = accentColor.value
  return {
    common: {
      primaryColor: c,
      primaryColorHover: shift(c, 0.14, 'light'),
      primaryColorPressed: shift(c, 0.16, 'dark'),
      primaryColorSuppl: shift(c, 0.08, 'light'),
    },
  }
})

export function setThemeMode(mode: ThemeMode) {
  themeMode.value = mode
  localStorage.setItem(MODE_KEY, mode)
}

export function setAccentColor(color: string) {
  accentColor.value = color
  localStorage.setItem(ACCENT_KEY, color)
}

function clampByte(v: number): number {
  return Math.max(0, Math.min(255, Math.round(v)))
}

function hexToRgb(hex: string): [number, number, number] {
  let h = hex.replace('#', '').trim()
  if (h.length === 3) h = h.split('').map((c) => c + c).join('')
  const n = parseInt(h || '2080f0', 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

function rgbToHex(r: number, g: number, b: number): string {
  return (
    '#' +
    [r, g, b].map((v) => clampByte(v).toString(16).padStart(2, '0')).join('')
  )
}

// shift moves a colour toward white ('light') or black ('dark') by ratio.
function shift(hex: string, ratio: number, toward: 'light' | 'dark'): string {
  const [r, g, b] = hexToRgb(hex)
  const target = toward === 'light' ? 255 : 0
  return rgbToHex(
    r + (target - r) * ratio,
    g + (target - g) * ratio,
    b + (target - b) * ratio,
  )
}

// Reflect the effective light/dark choice on <html> so style.css can swap its
// own CSS variables alongside Naive UI.
function applyDom() {
  document.documentElement.dataset.theme = isDark.value ? 'dark' : 'light'
}

watch(isDark, applyDom, { immediate: true })

media?.addEventListener('change', (e) => {
  prefersDark.value = e.matches
})
