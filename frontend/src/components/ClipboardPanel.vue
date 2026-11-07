<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { NButton, NEmpty, NModal, NSwitch, NTag, useMessage } from 'naive-ui'
import { Clipboard } from '@wailsio/runtime'
import { WorkbenchService, type ClipboardItem } from '../lib/api'
import { overlays, closeClipboard } from '../lib/uiState'
import { t } from '../lib/i18n'

const message = useMessage()

const items = ref<ClipboardItem[]>([])
const loading = ref(false)
const purgeDays = ref(3)

const purgeSeconds = computed(() => purgeDays.value * 24 * 3600)

async function load() {
  loading.value = true
  try {
    items.value = ((await WorkbenchService.ClipboardList()) ?? []) as ClipboardItem[]
  } catch (e) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

/** Capture whatever is in the OS clipboard right now. */
async function capture() {
  try {
    const text = await Clipboard.Text()
    if (!text) {
      message.warning(t('clipboard.nothing'))
      return
    }
    await WorkbenchService.ClipboardAdd(text)
    await load()
  } catch (e) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function copy(item: ClipboardItem) {
  await Clipboard.SetText(item.text)
  message.success(t('clipboard.copied'))
}

async function togglePin(item: ClipboardItem) {
  await WorkbenchService.ClipboardTogglePin(item.id, !item.pinned)
  await load()
}

async function remove(item: ClipboardItem) {
  await WorkbenchService.ClipboardDelete(item.id)
  await load()
}

async function clearUnpinned() {
  const pinned = await WorkbenchService.ClipboardClear()
  await load()
  message.success(t('clipboard.clearedPinned', { n: pinned }))
}

async function purgeOlder() {
  await WorkbenchService.ClipboardPurgeOlderThan(purgeSeconds.value)
  await load()
}

function fmt(ts: number): string {
  const d = new Date(ts * 1000)
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

watch(
  () => overlays.clipboard,
  (open) => {
    if (open) load()
  },
)

onMounted(load)
</script>

<template>
  <n-modal
    v-model:show="overlays.clipboard"
    transform-origin="center"
    style="width: 520px; margin-top: 10vh; align-self: flex-start"
  >
    <div class="cb-panel">
      <div class="cb-head">
        <span class="cb-title">{{ t('clipboard.title') }}</span>
        <div class="cb-actions">
          <n-button size="tiny" quaternary @click="capture">{{ t('clipboard.capture') }}</n-button>
          <n-button size="tiny" quaternary @click="clearUnpinned">{{ t('clipboard.clear') }}</n-button>
          <n-button size="tiny" quaternary @click="closeClipboard">{{ t('common.close') }}</n-button>
        </div>
      </div>

      <div class="cb-purge">
        <span>{{ t('clipboard.purgeOlderThan') }}</span>
        <n-switch v-model:value="purgeDays" :checked-value="3" :unchecked-value="0" size="small" />
        <n-button v-if="purgeDays > 0" size="tiny" tertiary @click="purgeOlder">
          {{ t('clipboard.purgeNow', { n: purgeDays }) }}
        </n-button>
      </div>

      <div class="cb-body">
        <div v-if="loading" class="cb-hint">{{ t('common.loading') }}</div>
        <n-empty v-else-if="items.length === 0" :description="t('clipboard.empty')" style="padding: 28px 0" />
        <div v-for="it in items" :key="it.id" class="cb-item" @click="copy(it)">
          <div class="cb-info">
            <div class="cb-preview">{{ it.preview || it.text.slice(0, 80) }}</div>
            <div class="cb-meta">
              <span>{{ fmt(it.createdAt) }}</span>
              <n-tag v-if="it.pinned" size="tiny" type="success" round>{{ t('clipboard.pinned') }}</n-tag>
            </div>
          </div>
          <div class="cb-item-actions">
            <n-button size="tiny" quaternary @click.stop="togglePin(it)">
              {{ it.pinned ? t('clipboard.unpin') : t('clipboard.pin') }}
            </n-button>
            <n-button size="tiny" quaternary type="error" @click.stop="remove(it)">
              {{ t('clipboard.delete') }}
            </n-button>
          </div>
        </div>
      </div>
      <div class="cb-foot">{{ t('clipboard.footNote') }}</div>
    </div>
  </n-modal>
</template>

<style scoped>
.cb-panel {
  border-radius: 12px;
  background: #1f1f2b;
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  overflow: hidden;
}
.cb-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}
.cb-title {
  font-weight: 600;
  font-size: 14px;
}
.cb-actions {
  display: flex;
  gap: 4px;
}
.cb-purge {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 16px;
  font-size: 12px;
  opacity: 0.75;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.cb-body {
  max-height: 48vh;
  overflow: auto;
  padding: 6px;
}
.cb-hint {
  opacity: 0.55;
  padding: 16px;
  font-size: 13px;
}
.cb-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  cursor: pointer;
}
.cb-item:hover {
  background: rgba(255, 255, 255, 0.06);
}
.cb-info {
  flex: 1;
  min-width: 0;
}
.cb-preview {
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cb-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  opacity: 0.5;
  margin-top: 2px;
}
.cb-item-actions {
  display: flex;
  gap: 2px;
  opacity: 0;
  transition: opacity 0.15s ease;
}
.cb-item:hover .cb-item-actions {
  opacity: 1;
}
.cb-foot {
  padding: 8px 16px;
  font-size: 11px;
  opacity: 0.45;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}
</style>
