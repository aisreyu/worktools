<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { Clipboard } from '@wailsio/runtime'
import { VaultService, type SecretEntry } from '../lib/api'
import { t, errText } from '../lib/i18n'

const message = useMessage()

type Row = SecretEntry

const categories = computed(() => [
  { label: t('secrets.catServer'), value: 'server' },
  { label: t('secrets.catDatabase'), value: 'database' },
  { label: t('secrets.catCloud'), value: 'cloud' },
  { label: t('secrets.catOther'), value: 'other' },
])

const rows = ref<Row[]>([])
const loading = ref(false)
const showForm = ref(false)
const saving = ref(false)
const reveal = ref<Set<number>>(new Set())
const form = ref<Row>(blank())

function blank(): Row {
  return {
    id: 0, title: '', category: 'server', host: '', port: 22, username: '', password: '', extra: '', note: '',
    createdAt: 0, updatedAt: 0,
  }
}

async function load() {
  loading.value = true
  try {
    rows.value = ((await VaultService.ListSecrets()) ?? []) as Row[]
  } catch (e) {
    message.error(errText(e))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = blank()
  showForm.value = true
}

function openEdit(row: Row) {
  form.value = { ...row }
  showForm.value = true
}

async function save() {
  if (!form.value.title.trim()) {
    message.warning(t('secrets.titleRequired'))
    return
  }
  saving.value = true
  try {
    await VaultService.SaveSecret(form.value)
    showForm.value = false
    await load()
    message.success(t('secrets.saved'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    saving.value = false
  }
}

async function remove(row: Row) {
  try {
    await VaultService.DeleteSecret(row.id)
    await load()
    message.success(t('secrets.deleted'))
  } catch (e) {
    message.error(errText(e))
  }
}

async function copy(text: string | undefined, label: string) {
  if (!text) {
    message.warning(t('secrets.fieldMissing', { v: label }))
    return
  }
  await Clipboard.SetText(text)
  message.success(t('secrets.copiedField', { v: label }))
  const snapshot = text
  setTimeout(async () => {
    const cur = await Clipboard.Text()
    if (cur === snapshot) await Clipboard.SetText('')
  }, 30_000)
}

/** Copy every field of the entry as a readable block. */
async function copyAll(row: Row) {
  const lines: string[] = []
  if (row.title) lines.push(`${t('secrets.fTitle')}: ${row.title}`)
  const catLabel = categories.value.find((c) => c.value === row.category)?.label ?? row.category
  if (catLabel) lines.push(`${t('secrets.fCategory')}: ${catLabel}`)
  if (row.host) lines.push(`${t('secrets.fHost')}: ${row.host}`)
  if (row.port) lines.push(`${t('secrets.fPort')}: ${row.port}`)
  if (row.username) lines.push(`${t('secrets.fUsername')}: ${row.username}`)
  if (row.password) lines.push(`${t('secrets.fPassword')}: ${row.password}`)
  if (row.extra) lines.push(`${t('secrets.fExtra')}: ${row.extra}`)
  if (row.note) lines.push(`${t('secrets.fNote')}: ${row.note}`)
  if (lines.length === 0) {
    message.warning(t('secrets.nothingToCopy'))
    return
  }
  const text = lines.join('\n')
  await Clipboard.SetText(text)
  message.success(t('secrets.copiedAll'))
  const snapshot = text
  setTimeout(async () => {
    const cur = await Clipboard.Text()
    if (cur === snapshot) await Clipboard.SetText('')
  }, 30_000)
}

function toggleReveal(id: number) {
  const s = new Set(reveal.value)
  s.has(id) ? s.delete(id) : s.add(id)
  reveal.value = s
}

const columns = computed<DataTableColumns<Row>>(() => [
  { title: t('secrets.colTitle'), key: 'title', ellipsis: { tooltip: true } },
  { title: t('secrets.colCategory'), key: 'category', width: 90, render: (r) => categories.value.find((c) => c.value === r.category)?.label ?? r.category },
  { title: t('secrets.colHost'), key: 'host', width: 160, ellipsis: { tooltip: true } },
  { title: t('secrets.colPort'), key: 'port', width: 70 },
  { title: t('secrets.colUsername'), key: 'username', width: 140, ellipsis: { tooltip: true } },
  {
    title: t('secrets.colPassword'),
    key: 'password',
    width: 120,
    render: (r) =>
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => toggleReveal(r.id) }, () =>
        reveal.value.has(r.id) ? r.password || t('secrets.empty') : '••••••',
      ),
  },
  {
    title: t('secrets.colActions'),
    key: 'actions',
    width: 330,
    render: (row) =>
      h(NSpace, { size: 6 }, () => [
        h(NButton, { size: 'tiny', onClick: () => copy(row.password, t('secrets.fPassword')) }, () => t('secrets.copyPassword')),
        h(NButton, { size: 'tiny', onClick: () => copy(row.extra, t('secrets.fExtra')) }, () => t('secrets.copyExtra')),
        h(NButton, { size: 'tiny', onClick: () => copyAll(row) }, () => t('secrets.copyAll')),
        h(NButton, { size: 'tiny', quaternary: true, onClick: () => openEdit(row) }, () => t('secrets.edit')),
        h(
          NPopconfirm,
          { onPositiveClick: () => remove(row) },
          {
            trigger: () => h(NButton, { size: 'tiny', type: 'error', quaternary: true }, () => t('secrets.remove')),
            default: () => t('secrets.removeConfirm'),
          },
        ),
      ]),
  },
])

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="row-between page-title">
      <span>{{ t('menu.secrets') }}</span>
      <n-button type="primary" size="small" @click="openCreate">{{ t('secrets.add') }}</n-button>
    </div>
    <n-data-table
      :columns="columns"
      :data="rows"
      :loading="loading"
      :row-key="(r: Row) => r.id"
      size="small"
      :bordered="false"
    />

    <n-modal v-model:show="showForm" preset="card" style="width: 560px" :title="form.id ? t('secrets.editEntry') : t('secrets.newEntry')">
      <n-form label-placement="left" label-width="84">
        <n-form-item :label="t('secrets.fTitle')"><n-input v-model:value="form.title" :placeholder="t('secrets.titlePlaceholder')" /></n-form-item>
        <n-form-item :label="t('secrets.fCategory')">
          <n-select v-model:value="form.category" :options="categories" />
        </n-form-item>
        <n-form-item :label="t('secrets.fHost')"><n-input v-model:value="form.host" :placeholder="t('secrets.hostPlaceholder')" /></n-form-item>
        <n-form-item :label="t('secrets.fPort')"><n-input-number v-model:value="form.port" :min="0" :max="65535" style="width: 100%" /></n-form-item>
        <n-form-item :label="t('secrets.fUsername')"><n-input v-model:value="form.username" /></n-form-item>
        <n-form-item :label="t('secrets.fPassword')">
          <n-input v-model:value="form.password" type="password" show-password-on="click" />
        </n-form-item>
        <n-form-item :label="t('secrets.fExtra')">
          <n-input
            v-model:value="form.extra"
            type="textarea"
            :autosize="{ minRows: 2 }"
            :placeholder="t('secrets.extraPlaceholder')"
          />
        </n-form-item>
        <n-form-item :label="t('secrets.fNote')"><n-input v-model:value="form.note" type="textarea" :autosize="{ minRows: 2 }" /></n-form-item>
      </n-form>
      <template #footer>
        <div class="row-between">
          <span />
          <n-space>
            <n-button @click="showForm = false">{{ t('secrets.cancel') }}</n-button>
            <n-button type="primary" :loading="saving" @click="save">{{ t('secrets.save') }}</n-button>
          </n-space>
        </div>
      </template>
    </n-modal>
  </div>
</template>
