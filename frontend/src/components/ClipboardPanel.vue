<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  NButton,
  NEmpty,
  NInput,
  NInputNumber,
  NModal,
  NSelect,
  NSwitch,
  NTag,
  useMessage,
} from 'naive-ui'
import { Events } from '@wailsio/runtime'
import { requestSavePath, publishSaved } from '../lib/saveFile'
import { ClipboardService, ExportService, type ClipboardItem } from '../lib/api'
import { overlays, closeClipboard } from '../lib/uiState'
import { vault } from '../lib/vaultState'
import { t } from '../lib/i18n'

const message = useMessage()

const items = ref<ClipboardItem[]>([])
const loading = ref(false)
const query = ref('')
const kind = ref('all')
const autoCapture = ref(true)
const purgeDays = ref(3)

// Inline edit state; editingId is the item currently being edited.
const editingId = ref(0)
const editingText = ref('')
const saving = ref(false)

const kindOptions = computed(() => [
  { label: t('clipboard.all'), value: 'all' },
  { label: t('clipboard.kind.text'), value: 'text' },
  { label: t('clipboard.kind.link'), value: 'link' },
  { label: t('clipboard.kind.code'), value: 'code' },
  { label: t('clipboard.kind.image'), value: 'image' },
  { label: t('clipboard.kind.file'), value: 'file' },
])

/** Inline data URL for an image capture, or '' when its payload was cleared. */
function imageSrc(it: ClipboardItem): string {
  if (!it.data) return ''
  return `data:${it.mime || 'image/png'};base64,${it.data}`
}

async function load() {
  loading.value = true
  try {
    items.value = ((await ClipboardService.List(query.value, kind.value)) ?? []) as ClipboardItem[]
  } catch (e) {
    message.error(errText(e))
  } finally {
    loading.value = false
  }
}

async function loadSettings() {
  try {
    autoCapture.value = await ClipboardService.AutoCapture()
  } catch {
    // vault locked or unavailable; leave the default.
  }
}

/** Capture whatever is in the OS clipboard right now. */
async function capture() {
  try {
    await ClipboardService.CaptureNow()
    await load()
  } catch (e) {
    message.warning(errText(e))
  }
}

async function copy(item: ClipboardItem) {
  try {
    await ClipboardService.Copy(item.id)
    message.success(t('clipboard.copied'))
  } catch (e) {
    message.error(errText(e))
  }
}

async function togglePin(item: ClipboardItem) {
  try {
    await ClipboardService.TogglePin(item.id, !item.pinned)
    await load()
  } catch (e) {
    message.error(errText(e))
  }
}

async function remove(item: ClipboardItem) {
  try {
    await ClipboardService.Delete(item.id)
    await load()
  } catch (e) {
    message.error(errText(e))
  }
}

function startEdit(item: ClipboardItem) {
  editingId.value = item.id
  editingText.value = item.text
}

function cancelEdit() {
  editingId.value = 0
  editingText.value = ''
}

async function saveEdit(item: ClipboardItem) {
  if (!editingText.value.trim()) {
    message.warning(t('clipboard.nothing'))
    return
  }
  saving.value = true
  try {
    await ClipboardService.UpdateText(item.id, editingText.value)
    cancelEdit()
    await load()
  } catch (e) {
    message.error(errText(e))
  } finally {
    saving.value = false
  }
}

async function clearData(item: ClipboardItem) {
  try {
    await ClipboardService.ClearData(item.id)
    await load()
    message.success(t('clipboard.dataCleared'))
  } catch (e) {
    message.error(errText(e))
  }
}

async function clearUnpinned() {
  try {
    await ClipboardService.Clear(true)
    await load()
    message.success(t('clipboard.clearedPinned', { n: 0 }))
  } catch (e) {
    message.error(errText(e))
  }
}

async function clearAll() {
  try {
    await ClipboardService.Clear(false)
    await load()
    message.success(t('clipboard.cleared'))
  } catch (e) {
    message.error(errText(e))
  }
}

async function purgeOlder() {
  try {
    await ClipboardService.PurgeOlderThan(purgeDays.value)
    await load()
    message.success(t('clipboard.purged'))
  } catch (e) {
    message.error(errText(e))
  }
}

async function toggleAuto(on: boolean) {
  try {
    await ClipboardService.SetAutoCapture(on)
  } catch (e) {
    autoCapture.value = !on
    message.error(errText(e))
  }
}

async function exportHistory() {
  const name = 'clipboard.txt'
  const path = await requestSavePath(t('clipboard.export'), name, [
    { DisplayName: 'Text', Pattern: '*.txt' },
  ])
  if (!path) return
  try {
    const content = await ClipboardService.ExportText(query.value, kind.value)
    await ExportService.SaveText(path, content)
    const where = await publishSaved(path, name)
    message.success(t('clipboard.exportSaved', { path: where || path }))
  } catch (e) {
    message.error(t('clipboard.exportFailed') + ': ' + errText(e))
  }
}

function fmt(ts: number): string {
  const d = new Date(ts * 1000)
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function errText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

let offClip: (() => void) | null = null
let debounce: ReturnType<typeof setTimeout> | null = null

watch(query, () => {
  if (debounce) clearTimeout(debounce)
  debounce = setTimeout(load, 200)
})
watch(kind, load)

watch(
  () => overlays.clipboard,
  (open) => {
    if (open) {
      load()
      loadSettings()
    }
    cancelEdit()
  },
)

onMounted(() => {
  // The panel is only reachable once unlocked; loading eagerly while locked
  // would surface a "vault is locked" error on the unlock screen. Opening the
  // panel reloads everything anyway (see the overlays.clipboard watcher).
  if (vault.unlocked) {
    load()
    loadSettings()
  }
  // Refresh live when the background poller captures new clipboard text.
  offClip = Events.On('nb:clipboard', () => {
    if (overlays.clipboard) load()
  })
})

onUnmounted(() => {
  offClip?.()
  if (debounce) clearTimeout(debounce)
})
</script>

<template>
  <n-modal
    v-model:show="overlays.clipboard"
    transform-origin="center"
    style="width: 640px; margin-top: 8vh; align-self: flex-start"
  >
    <div class="cb-panel">
      <div class="cb-head">
        <span class="cb-title">{{ t('clipboard.title') }}</span>
        <div class="cb-actions">
          <n-button size="tiny" quaternary @click="capture">{{ t('clipboard.captureNow') }}</n-button>
          <n-button size="tiny" quaternary @click="exportHistory">{{ t('clipboard.export') }}</n-button>
          <n-button size="tiny" quaternary type="error" @click="clearAll">{{ t('clipboard.clearAll') }}</n-button>
          <n-button size="tiny" quaternary @click="closeClipboard">{{ t('common.close') }}</n-button>
        </div>
      </div>

      <div class="cb-toolbar">
        <n-input
          v-model:value="query"
          size="small"
          clearable
          :placeholder="t('clipboard.search')"
          style="flex: 1"
        />
        <n-select v-model:value="kind" size="small" :options="kindOptions" style="width: 120px" />
      </div>

      <div class="cb-settings">
        <div class="cb-auto">
          <n-switch v-model:value="autoCapture" size="small" @update:value="toggleAuto" />
          <span class="cb-auto-label">{{ t('clipboard.autoCapture') }}</span>
        </div>
        <div class="cb-purge">
          <span>{{ t('clipboard.purgeOlderThan', { n: purgeDays }) }}</span>
          <n-input-number v-model:value="purgeDays" size="tiny" :min="1" style="width: 90px" />
          <n-button size="tiny" tertiary @click="purgeOlder">{{ t('clipboard.purgeNow') }}</n-button>
          <n-button size="tiny" tertiary @click="clearUnpinned">{{ t('clipboard.clear') }}</n-button>
        </div>
      </div>
      <div class="cb-auto-hint">{{ t('clipboard.autoCaptureHint') }}</div>

      <div class="cb-body">
        <div v-if="loading" class="cb-hint">{{ t('common.loading') }}</div>
        <n-empty v-else-if="items.length === 0" :description="t('clipboard.empty')" style="padding: 28px 0" />
        <div v-for="it in items" :key="it.id" class="cb-item">
          <template v-if="editingId === it.id">
            <n-input
              v-model:value="editingText"
              type="textarea"
              size="small"
              :autosize="{ minRows: 2, maxRows: 8 }"
            />
            <div class="cb-edit-actions">
              <n-button size="tiny" type="primary" :loading="saving" @click="saveEdit(it)">
                {{ t('clipboard.save') }}
              </n-button>
              <n-button size="tiny" quaternary @click="cancelEdit">{{ t('common.cancel') }}</n-button>
            </div>
          </template>
          <template v-else>
            <div class="cb-info" @click="copy(it)">
              <img
                v-if="it.kind === 'image' && imageSrc(it)"
                class="cb-thumb"
                :src="imageSrc(it)"
                :alt="t('clipboard.imageAlt')"
              />
              <div v-if="it.kind === 'image' && !imageSrc(it)" class="cb-preview cb-muted">
                {{ t('clipboard.noData') }}
              </div>
              <div v-else-if="it.kind !== 'image'" class="cb-preview">
                {{ it.preview || it.text.slice(0, 120) }}
              </div>
              <div class="cb-meta">
                <n-tag size="tiny" round>{{ t(`clipboard.kind.${it.kind}`) }}</n-tag>
                <span v-if="it.kind === 'image' && it.size">{{ Math.round(it.size / 1024) }} KB</span>
                <span>{{ fmt(it.createdAt) }}</span>
                <n-tag v-if="it.pinned" size="tiny" type="success" round>{{ t('clipboard.pinned') }}</n-tag>
              </div>
            </div>
            <div class="cb-item-actions">
              <n-button
                v-if="it.kind === 'image'"
                size="tiny"
                quaternary
                @click.stop="clearData(it)"
              >
                {{ t('clipboard.clearData') }}
              </n-button>
              <n-button size="tiny" quaternary @click.stop="togglePin(it)">
                {{ it.pinned ? t('clipboard.unpin') : t('clipboard.pin') }}
              </n-button>
              <n-button size="tiny" quaternary @click.stop="startEdit(it)">{{ t('clipboard.edit') }}</n-button>
              <n-button size="tiny" quaternary type="error" @click.stop="remove(it)">
                {{ t('clipboard.delete') }}
              </n-button>
            </div>
          </template>
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
.cb-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.cb-settings {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 16px 4px;
  font-size: 12px;
  opacity: 0.85;
}
.cb-auto {
  display: flex;
  align-items: center;
  gap: 8px;
}
.cb-auto-label {
  white-space: nowrap;
}
.cb-purge {
  display: flex;
  align-items: center;
  gap: 6px;
}
.cb-auto-hint {
  padding: 0 16px 8px;
  font-size: 11px;
  opacity: 0.45;
}
.cb-body {
  max-height: 46vh;
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
}
.cb-item:hover {
  background: rgba(255, 255, 255, 0.06);
}
.cb-info {
  flex: 1;
  min-width: 0;
  cursor: pointer;
}
.cb-preview {
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cb-thumb {
  max-height: 72px;
  max-width: 180px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  object-fit: contain;
  display: block;
}
.cb-muted {
  opacity: 0.5;
  font-style: italic;
}
.cb-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  opacity: 0.6;
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
.cb-edit-actions {
  display: flex;
  gap: 6px;
  margin-top: 8px;
}
.cb-foot {
  padding: 8px 16px;
  font-size: 11px;
  opacity: 0.45;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}
</style>
