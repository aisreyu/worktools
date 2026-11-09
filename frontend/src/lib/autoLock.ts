// Auto-lock setting: minutes of inactivity before the vault re-locks itself.
// Stored in localStorage (non-sensitive) and read by the idle poll in App.vue.
import { ref } from 'vue'

const KEY = 'nb.autoLockMinutes'

/** 0 disables auto-lock. */
export const autoLockMinutes = ref<number>(readStored())

function readStored(): number {
  const raw = localStorage.getItem(KEY)
  // Distinguish "never set" from a real value: Number(null) is 0, which would
  // otherwise be treated as a valid "auto-lock off" setting on first run.
  if (raw === null) return 15
  const n = Number(raw)
  return Number.isFinite(n) && n >= 0 ? n : 15
}

export function setAutoLockMinutes(minutes: number) {
  autoLockMinutes.value = minutes
  localStorage.setItem(KEY, String(minutes))
}
