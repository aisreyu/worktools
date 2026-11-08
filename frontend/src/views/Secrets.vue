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
import { VaultService, type SecretEntry, type Category } from '../lib/api'
import { copyWithAutoClear } from '../lib/clipboardClear'
import { t, errText } from '../lib/i18n'
import { useRouteHighlight } from '../lib/routeHighlight'

useRouteHighlight()

const message = useMessage()

type Row = SecretEntry

const cats = ref<Category[]>([])
const categories = computed(() => cats.value.map((c) => ({ label: c.name, value: c.code })))

// Category management modal
const catManageOpen = ref(false)
const catNewName = ref('')
const catEditingId = ref<number | null>(null)
const catEditingName = ref('')

const rows = ref<Row[]>([])
const loading = ref(false)
const showForm = ref(false)
const saving = ref(false)
const reveal = ref<Set<number>>(new Set())
const form = ref<Row>(blank())

function blank(): Row {
  return {
    id: 0, title: '', category: 'server', host: '', port: 22, username: '', password: '', extra: '', note: '', pinned: false,
    createdAt: 0, updatedAt: 0,
  }
}

async function load() {
  loading.value = true
  try {
    cats.value = ((await VaultService.ListCategories()) ?? []) as Category[]
    rows.value = ((await VaultService.ListSecrets()) ?? []) as Row[]
  } catch (e) {
    message.error(errText(e))
  } finally {
    loading.value = false
  }
}

// ---- category management ----
async function createCategory() {
  const name = catNewName.value.trim()
  if (!name) return
  try {
    await VaultService.CreateCategory(name)
    catNewName.value = ''
    await load()
    message.success(t('secrets.catAdded'))
  } catch (e) {
    message.error(errText(e))
  }
}

function startRenameCategory(c: Category) {
  catEditingId.value = c.id
  catEditingName.value = c.name
}

async function submitRenameCategory() {
  const name = catEditingName.value.trim()
  if (!name || catEditingId.value === null) return
  try {
    await VaultService.RenameCategory(catEditingId.value, name)
    catEditingId.value = null
    catEditingName.value = ''
    await load()
    message.success(t('secrets.catRenamed'))
  } catch (e) {
    message.error(errText(e))
  }
}

async function deleteCategory(id: number) {
  try {
    await VaultService.DeleteCategory(id)
    await load()
    message.success(t('secrets.catDeleted'))
  } catch (e) {
    if (String(e).includes('category is in use')) {
      message.error(t('secrets.catInUse'))
    } else {
      message.error(errText(e))
    }
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

async function togglePin(row: Row) {
  try {
    await VaultService.SetSecretPinned(row.id, !row.pinned)
    await load()
  } catch (e) {
    message.error(errText(e))
  }
}

async function copy(text: string | undefined, label: string) {
  if (!text) {
    message.warning(t('secrets.fieldMissing', { v: label }))
    return
  }
  await copyWithAutoClear(text)
  message.success(t('secrets.copiedField', { v: label }))
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
  await copyWithAutoClear(text)
  message.success(t('secrets.copiedAll'))
}

function toggleReveal(id: number) {
  const s = new Set(reveal.value)
  s.has(id) ? s.delete(id) : s.add(id)
  reveal.value = s
}

// --- content-adaptive column widths ---
// Naive's auto table layout equalises leftover space instead of fitting content,
// so measure the widest cell per column and pin explicit widths. Cells use the
// 14px v-sans body font; tiny buttons use 12px with 6px horizontal padding.
const CELL_FONT = '14px v-sans, system-ui, -apple-system, "system-ui", "Segoe UI", sans-serif'
const BTN_FONT = '12px v-sans, system-ui, -apple-system, "system-ui", "Segoe UI", sans-serif'
let measureCtx: CanvasRenderingContext2D | null = null

function textWidth(text: string, font: string): number {
  if (!text) return 0
  if (!measureCtx) measureCtx = document.createElement('canvas').getContext('2d')
  if (!measureCtx) return text.length * 14
  measureCtx.font = font
  return measureCtx.measureText(text).width
}

/** Column width that fits the widest of header/cells, clamped to [min,max] (8px padding each side). */
function fitWidth(header: string, values: string[], min: number, max: number): number {
  let w = textWidth(header, CELL_FONT)
  for (const v of values) w = Math.max(w, textWidth(v, CELL_FONT))
  return Math.max(min, Math.min(max, Math.ceil(w) + 16))
}

/** Width the action buttons need: buttons + 6px gaps + cell padding. */
function actionsWidth(labels: string[]): number {
  const buttons = labels.reduce((sum, l) => sum + Math.ceil(textWidth(l, BTN_FONT)) + 12, 0)
  return Math.ceil(buttons + 6 * (labels.length - 1) + 16) + 4
}

const columns = computed<DataTableColumns<Row>>(() => {
  const categoryLabel = (r: Row) => categories.value.find((c) => c.value === r.category)?.label ?? r.category
  const catValues = [...new Set([...rows.value.map(categoryLabel), ...categories.value.map((c) => c.label)])]
  const actionLabels = [t('secrets.copyPassword'), t('secrets.copyExtra'), t('secrets.copyAll'), t('secrets.pin'), t('secrets.unpin'), t('secrets.edit'), t('secrets.remove')]
  return [
    // Title stays flexible so the table keeps filling its container; the rest fit their content.
    { title: t('secrets.colTitle'), key: 'title', ellipsis: { tooltip: true }, minWidth: 120 },
    { title: t('secrets.colCategory'), key: 'category', width: fitWidth(t('secrets.colCategory'), catValues, 76, 160), render: (r) => categoryLabel(r) },
    { title: t('secrets.colHost'), key: 'host', width: fitWidth(t('secrets.colHost'), rows.value.map((r) => r.host), 70, 220), ellipsis: { tooltip: true } },
    { title: t('secrets.colPort'), key: 'port', width: fitWidth(t('secrets.colPort'), rows.value.map((r) => String(r.port || '')), 56, 100) },
    { title: t('secrets.colUsername'), key: 'username', width: fitWidth(t('secrets.colUsername'), rows.value.map((r) => r.username), 96, 220), ellipsis: { tooltip: true } },
    {
      title: t('secrets.colPassword'),
      key: 'password',
      width: fitWidth(t('secrets.colPassword'), ['••••••'], 70, 120),
      render: (r) =>
        h(NButton, { size: 'tiny', quaternary: true, onClick: () => toggleReveal(r.id) }, () =>
          reveal.value.has(r.id) ? r.password || t('secrets.empty') : '••••••',
        ),
    },
    {
      title: t('secrets.colActions'),
      key: 'actions',
      width: actionsWidth(actionLabels),
      render: (row) =>
        h(NSpace, { size: 6 }, () => [
          h(NButton, { size: 'tiny', onClick: () => copy(row.password, t('secrets.fPassword')) }, () => t('secrets.copyPassword')),
          h(NButton, { size: 'tiny', onClick: () => copy(row.extra, t('secrets.fExtra')) }, () => t('secrets.copyExtra')),
          h(NButton, { size: 'tiny', onClick: () => copyAll(row) }, () => t('secrets.copyAll')),
          h(NButton, { size: 'tiny', quaternary: true, type: row.pinned ? 'warning' : 'default', onClick: () => togglePin(row) }, () => (row.pinned ? t('secrets.unpin') : t('secrets.pin'))),
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
  ]
})

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="row-between page-title">
      <span>{{ t('menu.secrets') }}</span>
      <n-space size="small">
        <n-button size="small" @click="catManageOpen = true">{{ t('secrets.catManage') }}</n-button>
        <n-button type="primary" size="small" @click="openCreate">{{ t('secrets.add') }}</n-button>
      </n-space>
    </div>
    <n-data-table
      :columns="columns"
      :data="rows"
      :loading="loading"
      :row-key="(r: Row) => r.id"
      :row-props="(r: Row) => ({ 'data-hid': String(r.id) } as any)"
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

    <n-modal v-model:show="catManageOpen">
      <n-card :title="t('secrets.catManage')" style="width: 420px" :bordered="false" size="small">
        <n-space align="center" size="small" style="margin-bottom: 12px">
          <n-input
            v-model:value="catNewName"
            :placeholder="t('secrets.catNamePlaceholder')"
            size="small"
            @keyup.enter="createCategory"
          />
          <n-button size="small" type="primary" @click="createCategory">{{ t('secrets.catAdd') }}</n-button>
        </n-space>
        <div v-for="c in cats" :key="c.id" class="cat-row">
          <n-input
            v-if="catEditingId === c.id"
            v-model:value="catEditingName"
            size="small"
            @keyup.enter="submitRenameCategory"
          />
          <span v-else class="cat-name">{{ c.name }}</span>
          <n-space size="small" :wrap="false">
            <template v-if="catEditingId === c.id">
              <n-button size="tiny" type="primary" @click="submitRenameCategory">{{ t('secrets.save') }}</n-button>
            </template>
            <template v-else>
              <n-button size="tiny" quaternary @click="startRenameCategory(c)">{{ t('secrets.catRename') }}</n-button>
              <n-popconfirm @positive-click="deleteCategory(c.id)">
                <template #trigger>
                  <n-button size="tiny" quaternary type="error">{{ t('secrets.catDelete') }}</n-button>
                </template>
                {{ t('secrets.catDeleteConfirm') }}
              </n-popconfirm>
            </template>
          </n-space>
        </div>
      </n-card>
    </n-modal>
  </div>
</template>

<style scoped>
.cat-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}
.cat-row:last-child {
  border-bottom: none;
}
.cat-name {
  font-size: 13px;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
