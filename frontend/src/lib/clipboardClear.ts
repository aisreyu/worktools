// Sensitive-clipboard hygiene: after copying a secret or OTP code we clear the
// OS clipboard again after a configurable delay, but only if it still holds the
// text we put there (so we never wipe something the user copied later).
import { ref } from 'vue'
import { Clipboard } from '@wailsio/runtime'

const KEY = 'nb.clipClearSeconds'

/** 0 disables auto-clear. */
export const clipClearSeconds = ref<number>(readStored())

function readStored(): number {
  const n = Number(localStorage.getItem(KEY))
  return Number.isFinite(n) && n >= 0 ? n : 30
}

export function setClipClearSeconds(seconds: number) {
  clipClearSeconds.value = seconds
  localStorage.setItem(KEY, String(seconds))
}

/** Copy text, then clear it after the configured delay if unchanged. */
export async function copyWithAutoClear(text: string, seconds?: number): Promise<void> {
  await Clipboard.SetText(text)
  const delay = seconds ?? clipClearSeconds.value
  if (!delay) return
  const snapshot = text
  setTimeout(async () => {
    try {
      const cur = await Clipboard.Text()
      if (cur === snapshot) await Clipboard.SetText('')
    } catch {
      // clipboard can be unavailable in restricted webviews; ignore
    }
  }, delay * 1000)
}
