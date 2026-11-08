<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NConfigProvider,
  NMessageProvider,
  NLayout,
  NLayoutSider,
  NLayoutHeader,
  NLayoutContent,
  NMenu,
  NButton,
  zhCN,
  enUS,
  dateZhCN,
  dateEnUS,
} from 'naive-ui'
import { Events } from '@wailsio/runtime'
import { vault, refreshStatus, lock } from './lib/vaultState'
import { autoLockMinutes } from './lib/autoLock'
import { startScheduleReminders, stopScheduleReminders } from './lib/scheduleReminders'
import { locale, t } from './lib/i18n'
import { naiveTheme, themeOverrides } from './lib/theme'
import { overlays, togglePalette, openClipboard, closePalette, closeClipboard } from './lib/uiState'
import VaultHeader from './components/VaultHeader.vue'
import CommandPalette from './components/CommandPalette.vue'
import ClipboardPanel from './components/ClipboardPanel.vue'

const route = useRoute()
const router = useRouter()

const menuOptions = computed(() => [
  { label: t('menu.urls'), key: 'urls', disabled: !vault.unlocked },
  { label: t('menu.secrets'), key: 'secrets', disabled: !vault.unlocked },
  { label: t('menu.otp'), key: 'otp', disabled: !vault.unlocked },
  { label: t('menu.notes'), key: 'notes', disabled: !vault.unlocked },
  { label: t('menu.files'), key: 'files', disabled: !vault.unlocked },
  { label: t('menu.calendar'), key: 'calendar', disabled: !vault.unlocked },
  { label: t('menu.schedule'), key: 'schedule', disabled: !vault.unlocked },
  { label: t('menu.trash'), key: 'trash', disabled: !vault.unlocked },
  { label: t('menu.toolbox'), key: 'toolbox' },
  { label: t('menu.settings'), key: 'settings', disabled: !vault.unlocked },
])

const naiveLocale = computed(() => (locale.value === 'en' ? enUS : zhCN))
const naiveDateLocale = computed(() => (locale.value === 'en' ? dateEnUS : dateZhCN))

const activeKey = computed(() => String(route.name ?? ''))

// The sider can be collapsed to give the content the full width, which matters
// on narrow screens. The choice is remembered, and narrow windows start
// collapsed so the menu does not eat most of the viewport.
const SIDER_KEY = 'nb.siderCollapsed'
const collapsed = ref(false)

// While the vault is locked every menu entry is disabled, so the unlock screen
// always gets the full width; unlocking restores the remembered preference.
const siderCollapsed = computed(() => collapsed.value || !vault.unlocked)

function toggleSider() {
  collapsed.value = !collapsed.value
  try {
    localStorage.setItem(SIDER_KEY, collapsed.value ? '1' : '0')
  } catch {
    // storage can be unavailable in restricted webviews; ignore
  }
}

function restoreSider() {
  let stored: string | null = null
  try {
    stored = localStorage.getItem(SIDER_KEY)
  } catch {
    stored = null
  }
  collapsed.value = stored === null ? window.innerWidth < 760 : stored === '1'
}

function onMenu(key: string) {
  router.push({ name: key })
}

async function doLock() {
  await lock()
  closePalette()
  closeClipboard()
  router.push({ name: 'unlock' })
}

/** Global in-app shortcuts: Cmd/Ctrl+K palette, Cmd/Ctrl+Shift+V clipboard. */
function onKeydown(e: KeyboardEvent) {
  const mod = e.metaKey || e.ctrlKey
  if (!mod) return
  const key = e.key.toLowerCase()
  if (key === 'k') {
    e.preventDefault()
    if (!vault.unlocked) return
    togglePalette()
  } else if (key === 'v' && e.shiftKey) {
    e.preventDefault()
    if (!vault.unlocked) return
    if (overlays.clipboard) closeClipboard()
    else openClipboard()
  }
}

let offLock: (() => void) | null = null
let idleTimer: number | null = null

// Auto-lock tracks real user activity (mouse / keyboard / scroll / touch)
// rather than only vault writes, so simply reading does not trigger a lock.
let lastActivity = Date.now()
const ACTIVITY_EVENTS = ['mousemove', 'mousedown', 'keydown', 'wheel', 'touchstart'] as const

function markActivity() {
  lastActivity = Date.now()
}

function startIdleWatch() {
  for (const ev of ACTIVITY_EVENTS) window.addEventListener(ev, markActivity, { passive: true })
  idleTimer = window.setInterval(() => {
    if (!vault.unlocked) return
    const minutes = autoLockMinutes.value
    if (!minutes) return
    if (Date.now() - lastActivity >= minutes * 60_000) void doLock()
  }, 15000)
}

function stopIdleWatch() {
  for (const ev of ACTIVITY_EVENTS) window.removeEventListener(ev, markActivity)
  if (idleTimer !== null) clearInterval(idleTimer)
}

onMounted(async () => {
  restoreSider()
  await refreshStatus()
  window.addEventListener('keydown', onKeydown)
  startIdleWatch()
  startScheduleReminders()
  // The tray menu / global shortcut locks the vault in Go and emits this event.
  offLock = Events.On('nb:lock', () => {
    doLock()
  })
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  offLock?.()
  stopIdleWatch()
  stopScheduleReminders()
})
</script>

<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="themeOverrides" :locale="naiveLocale" :date-locale="naiveDateLocale">
    <n-message-provider>
      <n-layout has-sider style="height: 100vh">
        <n-layout-sider
          :width="200"
          :collapsed-width="0"
          :collapsed="siderCollapsed"
          collapse-mode="width"
          :show-trigger="false"
          :show-content="!siderCollapsed"
          bordered
        >
          <div class="nb-drag sider-brand">WorkTools</div>
          <div class="sider-search">
            <n-button class="nb-no-drag" size="small" block secondary @click="togglePalette">
              🔍 {{ t('palette.button') }}
            </n-button>
            <n-button class="nb-no-drag" size="small" block quaternary @click="openClipboard">
              📋 {{ t('clipboard.open') }}
            </n-button>
          </div>
          <n-menu :value="activeKey" :options="menuOptions" @update:value="onMenu" />
        </n-layout-sider>
        <n-layout>
          <n-layout-header bordered class="app-header">
            <n-button
              v-if="vault.unlocked"
              class="nb-no-drag sider-toggle"
              size="small"
              quaternary
              :title="collapsed ? t('nav.expand') : t('nav.collapse')"
              @click="toggleSider"
            >
              {{ collapsed ? '☰' : '«' }}
            </n-button>
            <VaultHeader class="header-rest" @lock="doLock" />
          </n-layout-header>
          <n-layout-content content-style="height: calc(100vh - 44px); overflow: auto">
            <router-view />
          </n-layout-content>
        </n-layout>
      </n-layout>
      <CommandPalette />
      <ClipboardPanel />
    </n-message-provider>
  </n-config-provider>
</template>

<style scoped>
.app-header {
  height: 44px;
  display: flex;
  align-items: center;
  gap: 4px;
}
.sider-toggle {
  flex: none;
  margin-left: 6px;
  font-size: 15px;
  line-height: 1;
}
.header-rest {
  flex: 1;
  min-width: 0;
}
.sider-brand {
  height: 44px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  font-weight: 600;
  letter-spacing: 0.3px;
}
.sider-search {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 4px 12px 10px;
}
</style>
