<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton, NCard, NEmpty, NInput, NList, NListItem, NModal, NSelect,
  NSwitch, NTag, NTimePicker, NCheckbox, useMessage,
} from 'naive-ui'
import { ScheduleService } from '../lib/api'
import type { ScheduleEntry } from '../../bindings/github.com/aisreyu/worktools/internal/store/models'
import { t } from '../lib/i18n'
import { useRouteHighlight } from '../lib/routeHighlight'

useRouteHighlight()

const message = useMessage()
const entries = ref<ScheduleEntry[]>([])
const showDone = ref(false)
const showModal = ref(false)
const editing = ref<ScheduleEntry | null>(null)

const form = ref({
  title: '',
  detail: '',
  dueAt: Math.floor(Date.now() / 1000) + 3600,
  remind: 0,
  recur: '',
})

const recurOptions = computed(() => [
  { label: t('schedule.recurNone'), value: '' },
  { label: t('schedule.recurDaily'), value: 'daily' },
  { label: t('schedule.recurWeekly'), value: 'weekly' },
  { label: t('schedule.recurMonthly'), value: 'monthly' },
])

const remindOptions = computed(() => [
  { label: t('schedule.remindOff'), value: 0 },
  { label: t('schedule.remindMin', { n: 5 }), value: 5 },
  { label: t('schedule.remindMin', { n: 15 }), value: 15 },
  { label: t('schedule.remindMin', { n: 30 }), value: 30 },
  { label: t('schedule.remindHour', { n: 1 }), value: 60 },
  { label: t('schedule.remindHour', { n: 2 }), value: 120 },
  { label: t('schedule.remindDay', { n: 1 }), value: 1440 },
])

async function load() {
  entries.value = (await ScheduleService.List()) ?? []
}

function openNew() {
  editing.value = null
  form.value = {
    title: '',
    detail: '',
    dueAt: Math.floor(Date.now() / 1000) + 3600,
    remind: 0,
    recur: '',
  }
  showModal.value = true
}

function openEdit(e: ScheduleEntry) {
  editing.value = e
  form.value = { title: e.title, detail: e.detail, dueAt: e.dueAt, remind: e.remind, recur: e.recur ?? '' }
  showModal.value = true
}

async function save() {
  if (!form.value.title.trim()) return
  const e = editing.value
    ? new (editing.value.constructor as any)({ ...editing.value, ...form.value })
    : form.value
  await ScheduleService.Save(e)
  message.success(t('schedule.saved'))
  showModal.value = false
  await load()
}

async function toggleDone(e: ScheduleEntry) {
  await ScheduleService.SetDone(e.id, !e.done)
  await load()
}

async function remove(e: ScheduleEntry) {
  await ScheduleService.Delete(e.id)
  message.success(t('schedule.deleted'))
  await load()
}

const visible = computed(() =>
  showDone.value ? entries.value : entries.value.filter((e) => !e.done)
)

const sorted = computed(() =>
  [...visible.value].sort((a, b) => a.dueAt - b.dueAt)
)

function fmt(ts: number): string {
  return new Date(ts * 1000).toLocaleString()
}

function isOverdue(e: ScheduleEntry): boolean {
  return !e.done && e.dueAt * 1000 < Date.now()
}

// Reminder polling is handled globally in lib/scheduleReminders.ts (App.vue),
// so reminders fire regardless of the active view. This view only loads data.

onMounted(() => {
  load()
})
</script>

<template>
  <div class="page">
    <div class="head">
      <h2 class="page-title">{{ t('schedule.title') }}</h2>
      <n-checkbox v-model:checked="showDone">{{ t('schedule.showDone') }}</n-checkbox>
      <n-button type="primary" @click="openNew">{{ t('schedule.new') }}</n-button>
    </div>

    <n-empty v-if="sorted.length === 0" :description="t('schedule.empty')" style="margin-top: 60px" />

    <n-list v-else bordered>
      <n-list-item v-for="e in sorted" :key="e.id" :data-hid="e.id">
        <div class="row">
          <div class="main">
            <div class="title-line">
              <span class="title" :class="{ done: e.done }">{{ e.title }}</span>
              <n-tag v-if="e.done" size="small" type="success" :bordered="false">{{ t('schedule.done') }}</n-tag>
              <n-tag v-else-if="isOverdue(e)" size="small" type="error" :bordered="false">{{ t('schedule.overdue') }}</n-tag>
              <n-tag v-if="e.recur" size="small" type="info" :bordered="false">{{ t('schedule.recur_' + e.recur) }}</n-tag>
            </div>
            <div class="meta">{{ fmt(e.dueAt) }}</div>
            <div v-if="e.detail" class="detail">{{ e.detail }}</div>
          </div>
          <div class="actions">
            <n-button size="small" @click="toggleDone(e)">{{ e.done ? t('schedule.undone') : t('schedule.done') }}</n-button>
            <n-button size="small" @click="openEdit(e)">{{ t('schedule.edit') }}</n-button>
            <n-button size="small" type="error" quaternary @click="remove(e)">{{ t('schedule.delete') }}</n-button>
          </div>
        </div>
      </n-list-item>
    </n-list>

    <n-modal v-model:show="showModal" preset="card" :title="editing ? t('schedule.edit') : t('schedule.new')" style="width: 480px">
      <n-form label-placement="left" label-width="90">
        <n-form-item :label="t('schedule.titleField')">
          <n-input v-model:value="form.title" :placeholder="t('schedule.titlePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('schedule.detail')">
          <n-input v-model:value="form.detail" type="textarea" :rows="3" :placeholder="t('schedule.detailPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('schedule.dueAt')">
          <n-time-picker
            :value="form.dueAt * 1000"
            :actions="['confirm']"
            @update:value="(v: number | null) => { if (v != null) form.dueAt = Math.floor(v / 1000) }"
          />
        </n-form-item>
        <n-form-item :label="t('schedule.remind')">
          <n-select v-model:value="form.remind" :options="remindOptions" />
        </n-form-item>
        <n-form-item :label="t('schedule.recur')">
          <n-select v-model:value="form.recur" :options="recurOptions" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="modal-footer">
          <n-button @click="showModal = false">{{ t('schedule.cancel') }}</n-button>
          <n-button type="primary" @click="save">{{ t('schedule.save') }}</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.page { padding: 16px; }
.head { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.page-title { margin: 0; font-size: 18px; flex: 1; }
.row { display: flex; align-items: center; gap: 12px; width: 100%; }
.main { flex: 1; min-width: 0; }
.title-line { display: flex; align-items: center; gap: 8px; }
.title { font-weight: 500; }
.title.done { text-decoration: line-through; opacity: 0.6; }
.meta { font-size: 12px; opacity: 0.7; margin-top: 2px; }
.detail { font-size: 13px; opacity: 0.8; margin-top: 4px; white-space: pre-wrap; }
.actions { display: flex; gap: 6px; flex-shrink: 0; }
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; }
</style>
