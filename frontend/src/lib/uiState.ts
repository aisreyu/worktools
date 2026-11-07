// Shared, dependency-free UI flags for cross-cutting overlays (command
// palette + clipboard history) that can be opened from anywhere in the app.
import { reactive } from 'vue'

export const overlays = reactive({
  palette: false,
  clipboard: false,
})

export function openPalette() {
  overlays.palette = true
}

export function closePalette() {
  overlays.palette = false
}

export function togglePalette() {
  overlays.palette = !overlays.palette
}

export function openClipboard() {
  overlays.clipboard = true
}

export function closeClipboard() {
  overlays.clipboard = false
}
