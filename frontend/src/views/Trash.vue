<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NButton, NEmpty, NList, NListItem, NPopconfirm, NTag, useMessage } from 'naive-ui'
import { TrashService, type TrashItem } from '../lib/api'
import { t, errText } from '../lib/i18n'

const message = useMessage()
const items = ref<TrashItem[]>([])
const loading = ref(false)

const kindLabels = computed<Record<string, string>>(() => ({
  url: t('menu.urls'),
  secret: t('menu.secrets'),
  otp: t('menu.otp'),
  note: t('menu.notes'),
  file: t('menu.files'),
}))

const kindType: Record<string, 'info' | 'success' | 'warning' | 'error' | 'default'> = {
  url: 'info',
  secret: 'warning',
  otp: 'success',
  note: 'default',
  file: 'error',
}

async function load() {
  loading.value = true
  try {
    items.value = ((await TrashService.List()) ?? []) as TrashItem[]
  } catch (e) {
    message.error(errText(e))
  } finally {
    loading.value = false
  }
}

async function restore(it: TrashItem) {
  try {
    await TrashService.Restore(it.kind, it.id)
    message.success(t('trash.restored'))
    await load()
  } catch (e) {
    message.error(errText(e))
  }
}

async function purge(it: TrashItem) {
  try {
    await TrashService.Purge(it.kind, it.id)
    message.success(t('trash.purged'))
    await load()
  } catch (e) {
    message.error(errText(e))
  }
}

async function emptyAll() {
  try {
    await TrashService.Empty()
    message.success(t('trash.emptied'))
    await load()
  } catch (e) {
    message.error(errText(e))
  }
}

function fmtTime(ts: number): string {
  return new Date(ts * 1000).toLocaleString()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="head">
      <h2 class="page-title">{{ t('trash.title') }}</h2>
      <n-popconfirm v-if="items.length > 0" @positive-click="emptyAll">
        <template #trigger>
          <n-button type="error" quaternary size="small">{{ t('trash.empty') }}</n-button>
        </template>
        {{ t('trash.emptyConfirm') }}
      </n-popconfirm>
    </div>

    <div v-if="loading" style="opacity: 0.6; padding: 24px 4px">{{ t('trash.loading') }}</div>
    <n-empty v-else-if="items.length === 0" :description="t('trash.emptyHint')" style="margin-top: 60px" />

    <n-list v-else bordered>
      <n-list-item v-for="it in items" :key="it.kind + '-' + it.id">
        <div class="trash-row">
          <n-tag size="small" :type="kindType[it.kind] ?? 'default'" :bordered="false">
            {{ kindLabels[it.kind] ?? it.kind }}
          </n-tag>
          <div class="trash-main">
            <div class="trash-title">{{ it.title || '—' }}</div>
            <div class="trash-meta">
              <span v-if="it.subtitle">{{ it.subtitle }}</span>
              <span class="trash-time">{{ t('trash.deletedAt') }}: {{ fmtTime(it.deletedAt) }}</span>
            </div>
          </div>
          <div class="trash-actions">
            <n-button size="small" @click="restore(it)">{{ t('trash.restore') }}</n-button>
            <n-popconfirm @positive-click="purge(it)">
              <template #trigger>
                <n-button size="small" type="error" quaternary>{{ t('trash.purge') }}</n-button>
              </template>
              {{ t('trash.purgeConfirm') }}
            </n-popconfirm>
          </div>
        </div>
      </n-list-item>
    </n-list>
  </div>
</template>

<style scoped>
.page { padding: 16px; }
.head { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.page-title { margin: 0; font-size: 18px; flex: 1; }
.trash-row { display: flex; align-items: center; gap: 12px; width: 100%; }
.trash-main { flex: 1; min-width: 0; }
.trash-title { font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.trash-meta { font-size: 12px; opacity: 0.7; margin-top: 2px; display: flex; gap: 12px; flex-wrap: wrap; }
.trash-time { opacity: 0.8; }
.trash-actions { display: flex; gap: 6px; flex-shrink: 0; }
</style>
