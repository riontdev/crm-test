export function cn(...classes: unknown[]): string {
  return classes.filter((c): c is string => typeof c === 'string' && c.length > 0).join(' ')
}

export function initials(name?: string): string {
  if (!name || !name.trim()) return '?'
  const parts = name.trim().split(/\s+/)
  const first = parts[0]?.charAt(0) ?? ''
  const second = parts.length > 1 ? parts[parts.length - 1]?.charAt(0) ?? '' : ''
  return (first + second).toUpperCase() || '?'
}

const AVATAR_COLORS = [
  'bg-rose-100 text-rose-700 dark:bg-rose-500/20 dark:text-rose-300',
  'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300',
  'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300',
  'bg-teal-100 text-teal-700 dark:bg-teal-500/20 dark:text-teal-300',
  'bg-sky-100 text-sky-700 dark:bg-sky-500/20 dark:text-sky-300',
  'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-300',
  'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-500/20 dark:text-fuchsia-300',
  'bg-orange-100 text-orange-700 dark:bg-orange-500/20 dark:text-orange-300',
]

export function hashColor(name: string): string {
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = (hash * 31 + name.charCodeAt(i)) | 0
  }
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length]
}

export function relativeTime(dateStr?: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return ''
  const diffMs = Date.now() - d.getTime()
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return 'ahora'
  if (diffMin < 60) return `${diffMin}m`
  const diffH = Math.floor(diffMin / 60)
  if (diffH < 24) return `${diffH}h`
  const diffD = Math.floor(diffH / 24)
  if (diffD < 7) return `${diffD}d`
  return d.toLocaleDateString('es', { day: 'numeric', month: 'short' })
}

export function formatTime(dateStr?: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return ''
  return d.toLocaleTimeString('es', { hour: '2-digit', minute: '2-digit' })
}

export function formatDayLabel(dateStr?: string): string {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return ''
  const today = new Date()
  const yesterday = new Date(today)
  yesterday.setDate(today.getDate() - 1)
  const sameDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
  if (sameDay(d, today)) return 'Hoy'
  if (sameDay(d, yesterday)) return 'Ayer'
  return d.toLocaleDateString('es', { day: 'numeric', month: 'long', year: 'numeric' })
}
