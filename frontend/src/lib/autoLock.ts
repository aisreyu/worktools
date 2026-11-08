// Auto-lock setting: minutes of inactivity before the vault re-locks itself.
// Stored in localStorage (non-sensitive) and read by the idle poll in App.vue.
import { ref } from 'vue'

const KEY = 'nb.autoLockMinutes'

/** 0 disables auto-lock. */
export const autoLockMinutes = ref<number>(readStored())

function readStored(): number {
  const n = Number(localStorage.getItem(KEY))
  return Number.isFinite(n) && n >= 0 ? n : 15
}

export function setAutoLockMinutes(minutes: number) {
  autoLockMinutes.value = minutes
  localStorage.setItem(KEY, String(minutes))
}
