/**
 * The dashboard's colour theme, as Coolify keeps it: `localStorage.theme` is
 * `dark`, `light` or `system`, dark by default, and the `dark` class on <html>
 * switches the Tailwind `dark:` variant. index.html applies the same rule
 * before first paint; this module keeps it in step afterwards.
 */
export type Theme = 'dark' | 'light' | 'system'

const prefersDark = matchMedia('(prefers-color-scheme: dark)')

function stored(): Theme {
  const value = localStorage.getItem('theme')
  return value === 'light' || value === 'system' ? value : 'dark'
}

function apply(theme: Theme) {
  const dark = theme === 'dark' || (theme === 'system' && prefersDark.matches)
  const root = document.documentElement
  root.classList.toggle('dark', dark)
  // As Paperclip's ThemeContext: native controls and the browser bar follow.
  root.style.colorScheme = dark ? 'dark' : 'light'
  document.querySelector('meta[name=theme-color]')?.setAttribute('content', dark ? '#18181b' : '#ffffff')
}

class ThemeState {
  current = $state<Theme>(stored())

  constructor() {
    apply(this.current)
    prefersDark.addEventListener('change', () => {
      if (this.current === 'system') apply('system')
    })
  }

  set(theme: Theme) {
    this.current = theme
    localStorage.setItem('theme', theme)
    apply(theme)
  }
}

export const theme = new ThemeState()
