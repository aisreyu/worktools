// Global schedule reminder poller. Runs app-wide (not only while the Schedule
// view is open) so due-time reminders fire no matter which page is showing.
import { createDiscreteApi } from 'naive-ui'
import { ScheduleService } from './api'
import { vault } from './vaultState'
import { t } from './i18n'

// A detached message API lets us show an in-app toast even though this module
// lives outside the component tree that renders the message provider.
const { message } = createDiscreteApi(['message'])

const POLL_MS = 30_000
let timer: ReturnType<typeof setInterval> | null = null

async function poll() {
  if (!vault.unlocked) return
  try {
    const due = (await ScheduleService.DueForNotify()) ?? []
    for (const e of due) {
      const body = t('schedule.remindMsg', { title: e.title })
      try {
        await ScheduleService.Notify(e.title, body)
      } catch {
        // Fall back to an in-app toast when OS notifications are unavailable.
        message.info(body, { duration: 8000 })
      }
    }
  } catch {
    /* vault may be locked mid-poll */
  }
}

export function startScheduleReminders() {
  if (timer) return
  void poll()
  timer = setInterval(poll, POLL_MS)
}

export function stopScheduleReminders() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}
