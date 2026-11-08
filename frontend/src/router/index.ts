import { createRouter, createWebHashHistory } from 'vue-router'
import { vault } from '../lib/vaultState'
import Unlock from '../views/Unlock.vue'
import Urls from '../views/Urls.vue'
import Secrets from '../views/Secrets.vue'
import Otp from '../views/Otp.vue'
import Notes from '../views/Notes.vue'
import Files from '../views/Files.vue'
import Calendar from '../views/Calendar.vue'
import Schedule from '../views/Schedule.vue'
import Toolbox from '../views/Toolbox.vue'
import Settings from '../views/Settings.vue'
import Trash from '../views/Trash.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/unlock' },
    { path: '/unlock', name: 'unlock', component: Unlock },
    { path: '/urls', name: 'urls', component: Urls, meta: { requiresUnlock: true } },
    { path: '/secrets', name: 'secrets', component: Secrets, meta: { requiresUnlock: true } },
    { path: '/otp', name: 'otp', component: Otp, meta: { requiresUnlock: true } },
    { path: '/notes', name: 'notes', component: Notes, meta: { requiresUnlock: true } },
    { path: '/files', name: 'files', component: Files, meta: { requiresUnlock: true } },
    { path: '/calendar', name: 'calendar', component: Calendar, meta: { requiresUnlock: true } },
    { path: '/schedule', name: 'schedule', component: Schedule, meta: { requiresUnlock: true } },
    { path: '/trash', name: 'trash', component: Trash, meta: { requiresUnlock: true } },
    { path: '/toolbox', name: 'toolbox', component: Toolbox },
    { path: '/settings', name: 'settings', component: Settings, meta: { requiresUnlock: true } },
  ],
})

// Keep sensitive pages behind the unlock screen.
router.beforeEach((to) => {
  if (to.meta.requiresUnlock && !vault.unlocked) {
    return { name: 'unlock' }
  }
  return true
})

export default router
