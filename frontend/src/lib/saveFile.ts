// Save-destination helpers.
//
// Desktop platforms show a native save dialog. Android has no such dialog:
// Wails reports "save file dialogs are not supported on Android", so exports
// are written into the app sandbox first and then copied into the shared
// Downloads collection through the WebView bridge (see WailsJSBridge.java).
import { Dialogs } from '@wailsio/runtime'
import { AppService } from './api'

export interface SaveFilter {
  DisplayName: string
  Pattern: string
}

interface AndroidBridge {
  saveToDownloads?: (json: string) => string
}

function androidBridge(): AndroidBridge | undefined {
  const w = window as unknown as { wails?: AndroidBridge }
  return typeof w.wails?.saveToDownloads === 'function' ? w.wails : undefined
}

/** True when the app must bypass the native save dialog (Android). */
export function usesSandboxSave(): boolean {
  return androidBridge() !== undefined
}

/**
 * Returns the path a service should write its export to, or "" if the user
 * cancelled. On Android this is a private sandbox path that publishSaved()
 * later moves into Downloads.
 */
export async function requestSavePath(
  title: string,
  filename: string,
  filters?: SaveFilter[],
): Promise<string> {
  if (!usesSandboxSave()) {
    const picked = await Dialogs.SaveFile(
      filters && filters.length > 0
        ? { Title: title, Filename: filename, Filters: filters }
        : { Title: title, Filename: filename },
    )
    return (picked as string | null) ?? ''
  }
  return (await AppService.SandboxPath(filename)) as string
}

const MIME_BY_EXT: Record<string, string> = {
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  gif: 'image/gif',
  webp: 'image/webp',
  bmp: 'image/bmp',
  pdf: 'application/pdf',
  zip: 'application/zip',
  json: 'application/json',
  txt: 'text/plain',
  csv: 'text/csv',
  md: 'text/markdown',
  ttf: 'font/ttf',
  ttc: 'font/collection',
}

function mimeOf(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  return MIME_BY_EXT[ext] ?? 'application/octet-stream'
}

/**
 * Publishes a file that a service just wrote. Returns where the file ended up
 * ("" on desktop, where the dialog path already is the final location).
 */
export async function publishSaved(path: string, filename: string): Promise<string> {
  const bridge = androidBridge()
  if (!bridge?.saveToDownloads) return ''
  const raw = bridge.saveToDownloads(
    JSON.stringify({ path, name: filename, mime: mimeOf(filename) }),
  )
  const res = JSON.parse(raw || '{}') as { ok?: boolean; path?: string; error?: string }
  if (!res.ok) throw new Error(res.error || '保存失败')
  return res.path ?? filename
}

/** Suffix appended to success messages so mobile users know where the file went. */
export function savedLocationSuffix(location: string): string {
  return location ? ` → ${location}` : ''
}
