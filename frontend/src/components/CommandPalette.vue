<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { NEmpty, NModal, useMessage } from 'naive-ui'
import { WorkbenchService, type SearchHit } from '../lib/api'
import { vault } from '../lib/vaultState'
import { overlays, closePalette } from '../lib/uiState'
import { t } from '../lib/i18n'

const router = useRouter()
const message = useMessage()

const query = ref('')
const hits = ref<SearchHit[]>([])
const active = ref(0)
const loading = ref(false)
const inputRef = ref<{ focus: () => void } | null>(null)

const kindIcons: Record<string, string> = {
  url: '🔗',
  secret: '🔑',
  otp: '🔢',
  note: '📝',
  schedule: '📅',
  tool: '🧰',
}

const kindLabels = computed<Record<string, string>>(() => ({
  url: t('menu.urls'),
  secret: t('menu.secrets'),
  otp: t('menu.otp'),
  note: t('menu.notes'),
  schedule: t('menu.schedule'),
  tool: t('menu.toolbox'),
}))

async function run() {
  if (!vault.unlocked) {
    hits.value = []
    return
  }
  loading.value = true
  try {
    hits.value = ((await WorkbenchService.Search(query.value, 40)) ?? []) as SearchHit[]
    active.value = 0
  } catch (e) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

watch(query, run)

watch(
  () => overlays.palette,
  async (open) => {
    if (open) {
      active.value = 0
      await run()
      await nextTick()
      inputRef.value?.focus()
    } else {
      query.value = ''
      hits.value = []
    }
  },
)

function move(delta: number) {
  if (hits.value.length === 0) return
  active.value = (active.value + delta + hits.value.length) % hits.value.length
}

function choose(hit: SearchHit | undefined) {
  const h = hit ?? hits.value[active.value]
  if (!h) return
  closePalette()
  if (h.kind === 'tool') {
    router.push({ name: 'toolbox' })
  } else {
    router.push({ name: h.route.replace(/^\//, '') })
  }
}
</script>

<template>
  <n-modal
    v-model:show="overlays.palette"
    :mask-closable="true"
    transform-origin="center"
    style="width: 560px; margin-top: 12vh; align-self: flex-start"
  >
    <div class="palette">
      <input
        ref="inputRef"
        v-model="query"
        class="palette-input"
        :placeholder="t('palette.placeholder')"
        @keydown.down.prevent="move(1)"
        @keydown.up.prevent="move(-1)"
        @keydown.enter.prevent="choose(undefined)"
        @keydown.esc="closePalette()"
      />
      <div class="palette-body">
        <div v-if="!vault.unlocked" class="palette-hint">{{ t('palette.locked') }}</div>
        <div v-else-if="loading && hits.length === 0" class="palette-hint">{{ t('palette.searching') }}</div>
        <n-empty v-else-if="hits.length === 0" :description="t('palette.empty')" style="padding: 28px 0" />
        <div
          v-for="(h, i) in hits"
          :key="h.kind + '-' + h.id + '-' + i"
          class="palette-item"
          :class="{ active: i === active }"
          @mouseenter="active = i"
          @click="choose(h)"
        >
          <span class="palette-icon">{{ kindIcons[h.kind] ?? '•' }}</span>
          <div class="palette-info">
            <div class="palette-title">{{ h.title || '—' }}</div>
            <div v-if="h.subtitle" class="palette-sub">{{ h.subtitle }}</div>
          </div>
          <span class="palette-kind">{{ kindLabels[h.kind] ?? h.kind }}</span>
        </div>
      </div>
      <div class="palette-foot">
        <span>↑↓ {{ t('palette.navigate') }}</span>
        <span>↵ {{ t('palette.open') }}</span>
        <span>esc {{ t('palette.close') }}</span>
      </div>
    </div>
  </n-modal>
</template>

<style scoped>
.palette {
  border-radius: 12px;
  background: #1f1f2b;
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  overflow: hidden;
}
.palette-input {
  width: 100%;
  border: none;
  outline: none;
  background: transparent;
  color: inherit;
  font-size: 16px;
  padding: 16px 18px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}
.palette-body {
  max-height: 50vh;
  overflow: auto;
  padding: 6px;
}
.palette-hint {
  opacity: 0.55;
  padding: 20px;
  font-size: 13px;
}
.palette-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border-radius: 8px;
  cursor: pointer;
}
.palette-item.active {
  background: rgba(99, 226, 183, 0.14);
}
.palette-icon {
  font-size: 15px;
  width: 20px;
  text-align: center;
}
.palette-info {
  flex: 1;
  min-width: 0;
}
.palette-title {
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.palette-sub {
  font-size: 12px;
  opacity: 0.55;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.palette-kind {
  font-size: 11px;
  opacity: 0.45;
  flex-shrink: 0;
}
.palette-foot {
  display: flex;
  gap: 14px;
  padding: 8px 16px;
  font-size: 11px;
  opacity: 0.45;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}
</style>
