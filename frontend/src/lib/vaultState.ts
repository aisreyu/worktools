// A minimal pub/sub store for vault lock state shared across views.
import { reactive } from 'vue'
import { VaultService } from './api'

export const vault = reactive({
  initialised: false,
  unlocked: false,
})

export async function refreshStatus() {
  const s = await VaultService.Status()
  vault.initialised = s.initialised
  vault.unlocked = s.unlocked
}

export async function lock() {
  await VaultService.Lock()
  await refreshStatus()
}
