<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
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
  darkTheme,
  zhCN,
  enUS,
  dateZhCN,
  dateEnUS,
} from 'naive-ui'
import { Events } from '@wailsio/runtime'
import { vault, refreshStatus, lock } from './lib/vaultState'
import { locale, t } from './lib/i18n'
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
  { label: t('menu.toolbox'), key: 'toolbox' },
  { label: t('menu.settings'), key: 'settings', disabled: !vault.unlocked },
])

const naiveLocale = computed(() => (locale.value === 'en' ? enUS : zhCN))
const naiveDateLocale = computed(() => (locale.value === 'en' ? dateEnUS : dateZhCN))

const activeKey = computed(() => String(route.name ?? ''))

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

onMounted(async () => {
  await refreshStatus()
  window.addEventListener('keydown', onKeydown)
  // The tray menu / global shortcut locks the vault in Go and emits this event.
  offLock = Events.On('nb:lock', () => {
    doLock()
  })
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  offLock?.()
})
</script>

<template>
  <n-config-provider :theme="darkTheme" :locale="naiveLocale" :date-locale="naiveDateLocale">
    <n-message-provider>
      <n-layout has-sider style="height: 100vh">
        <n-layout-sider :width="200" bordered>
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
          <n-layout-header bordered style="height: 44px">
            <VaultHeader @lock="doLock" />
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
