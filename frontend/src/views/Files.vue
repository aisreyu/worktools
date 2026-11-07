<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NEmpty,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  useMessage,
} from 'naive-ui'
import { Dialogs } from '@wailsio/runtime'
import { FileService, type FileCategory, type FileEntry } from '../lib/api'
import { t, errText } from '../lib/i18n'
import PdfViewer from '../components/PdfViewer.vue'

const message = useMessage()

// Bodies up to this size are kept inside the encrypted SQLite database; larger
// files are encrypted to a blob on disk (see internal/vault/repo_files.go).
const INLINE_MAX = 512 * 1024

const files = ref<FileEntry[]>([])
const cats = ref<FileCategory[]>([])
const loading = ref(false)
const search = ref('')
const categoryFilter = ref<string>('')
const selectedId = ref<number | null>(null)
const adding = ref(false)

const catManageOpen = ref(false)
const catNewName = ref('')
const catEditingId = ref<number | null>(null)
const catEditingName = ref('')

const editorTitle = ref('')
const editorCategory = ref('other')
const editorNote = ref('')
const saving = ref(false)

const previewData = ref('')
const previewMime = ref('')
const previewLoading = ref(false)

const categories = computed(() => [
  { label: t('files.allCategory'), value: '' },
  ...cats.value.map((c) => ({ label: c.name, value: c.code })),
])

const editCategories = computed(() => categories.value.filter((c) => c.value !== ''))

function categoryLabel(v: string): string {
  return categories.value.find((c) => c.value === v)?.label ?? v
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return files.value.filter((f) => {
    if (categoryFilter.value && f.category !== categoryFilter.value) return false
    if (!q) return true
    return f.title.toLowerCase().includes(q) || f.note.toLowerCase().includes(q)
  })
})

const selected = computed(() => files.value.find((f) => f.id === selectedId.value) ?? null)

async function load() {
  loading.value = true
  try {
    files.value = ((await FileService.List()) ?? []) as FileEntry[]
    cats.value = ((await FileService.ListCategories()) ?? []) as FileCategory[]
    if (selectedId.value !== null && !files.value.some((f) => f.id === selectedId.value)) {
      selectedId.value = null
      resetEditor()
    }
  } catch (e) {
    message.error(errText(e))
  } finally {
    loading.value = false
  }
}

async function createCategory() {
  const name = catNewName.value.trim()
  if (!name) {
    message.warning(t('files.catNameRequired'))
    return
  }
  try {
    await FileService.CreateCategory(name)
    catNewName.value = ''
    await load()
    message.success(t('files.catCreated'))
  } catch (e) {
    message.error(errText(e))
  }
}

function startRenameCategory(c: FileCategory) {
  catEditingId.value = c.id
  catEditingName.value = c.name
}

async function submitRenameCategory() {
  if (catEditingId.value === null) return
  const name = catEditingName.value.trim()
  if (!name) {
    message.warning(t('files.catNameRequired'))
    return
  }
  try {
    await FileService.RenameCategory(catEditingId.value, name)
    catEditingId.value = null
    await load()
    message.success(t('files.catRenamed'))
  } catch (e) {
    message.error(errText(e))
  }
}

async function deleteCategory(id: number) {
  try {
    await FileService.DeleteCategory(id)
    await load()
    message.success(t('files.catDeleted'))
  } catch (e) {
    if (String(e).includes('category is in use')) {
      message.warning(t('files.catInUse'))
    } else {
      message.error(errText(e))
    }
  }
}

function resetEditor() {
  editorTitle.value = ''
  editorCategory.value = 'other'
  editorNote.value = ''
  previewData.value = ''
  previewMime.value = ''
}

async function addFiles() {
  const picked = await Dialogs.OpenFile({
    Title: t('files.add'),
    AllowsMultipleSelection: true,
    AllowsOtherFiletypes: true,
    Filters: [{ DisplayName: t('files.title'), Pattern: '*.pdf;*.jpg;*.jpeg;*.png;*.gif' }],
  })
  const paths = Array.isArray(picked) ? picked : picked ? [picked] : []
  if (paths.length === 0) return
  adding.value = true
  try {
    let lastId = 0
    for (const p of paths) {
      lastId = Number(await FileService.SaveFromFile(p, '', editorCategory.value, ''))
    }
    await load()
    if (lastId) selectById(lastId)
    message.success(t('files.added'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    adding.value = false
  }
}

function selectById(id: number) {
  const f = files.value.find((x) => x.id === id)
  if (f) select(f)
}

function select(f: FileEntry) {
  selectedId.value = f.id
  editorTitle.value = f.title
  editorCategory.value = f.category || 'other'
  editorNote.value = f.note
  loadPreview(f)
}

async function loadPreview(f: FileEntry) {
  previewData.value = ''
  previewMime.value = ''
  if (!isPreviewable(f.mime)) return
  previewLoading.value = true
  try {
    const full = (await FileService.Get(f.id)) as FileEntry
    previewMime.value = full.mime || f.mime
    previewData.value = full.data ?? ''
  } catch (e) {
    message.error(errText(e))
  } finally {
    previewLoading.value = false
  }
}

function isImage(mime: string): boolean {
  return mime.startsWith('image/')
}

function isPdf(mime: string): boolean {
  return mime === 'application/pdf'
}

function isPreviewable(mime: string): boolean {
  return isImage(mime) || isPdf(mime)
}

const previewUrl = computed(() => {
  if (!previewData.value || !isImage(previewMime.value)) return ''
  return `data:${previewMime.value || 'image/png'};base64,${previewData.value}`
})

async function saveMeta() {
  if (!editorTitle.value.trim()) {
    message.warning(t('files.titleRequired'))
    return
  }
  saving.value = true
  try {
    await FileService.SaveMeta({
      id: selectedId.value ?? 0,
      title: editorTitle.value,
      category: editorCategory.value,
      note: editorNote.value,
      mime: selected.value?.mime ?? '',
      size: selected.value?.size ?? 0,
      createdAt: 0,
      updatedAt: 0,
    })
    await load()
    message.success(t('files.saved'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    saving.value = false
  }
}

async function exportFile() {
  const f = selected.value
  if (!f) return
  const path = await Dialogs.SaveFile({ Title: t('files.export'), Filename: exportName(f) })
  if (!path) return
  try {
    await FileService.Export(f.id, path)
    message.success(t('files.exported') + path)
  } catch (e) {
    message.error(errText(e))
  }
}

function exportName(f: FileEntry): string {
  const base = (f.title || 'file').replace(/[\\/:*?"<>|]/g, '_')
  if (base.includes('.')) return base
  return base + extForMime(f.mime)
}

function extForMime(mime: string): string {
  const map: Record<string, string> = {
    'image/jpeg': '.jpg',
    'image/png': '.png',
    'image/gif': '.gif',
    'image/webp': '.webp',
    'application/pdf': '.pdf',
  }
  return map[mime] ?? ''
}

async function removeFile() {
  if (selectedId.value === null) return
  try {
    await FileService.Delete(selectedId.value)
    selectedId.value = null
    resetEditor()
    await load()
    message.success(t('files.deleted'))
  } catch (e) {
    message.error(errText(e))
  }
}

function fmtSize(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(2)} MB`
}

function fmtTime(ts: number): string {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function storageLabel(size: number): string {
  return size <= INLINE_MAX ? t('files.storageInline') : t('files.storageDisk')
}

onMounted(load)
</script>

<template>
  <div class="page files-page">
    <div class="row-between page-title">
      <span>{{ t('files.title') }}</span>
      <n-button type="primary" size="small" :loading="adding" @click="addFiles">{{ t('files.add') }}</n-button>
    </div>
    <div class="files-hint">{{ t('files.desc') }}</div>

    <div class="files-layout">
      <!-- Left: file list -->
      <div class="files-list">
        <n-input v-model:value="search" :placeholder="t('files.searchPlaceholder')" clearable size="small" round style="margin-bottom: 8px" />
        <div class="cat-filter-row">
          <n-select v-model:value="categoryFilter" :options="categories" size="small" />
          <n-button size="small" quaternary @click="catManageOpen = true">{{ t('files.catManage') }}</n-button>
        </div>
        <div v-if="loading" class="files-muted">{{ t('files.loading') }}</div>
        <n-empty v-else-if="filtered.length === 0" :description="t('files.empty')" style="padding: 24px 0" />
        <div v-else class="files-list-scroll">
          <div
            v-for="f in filtered"
            :key="f.id"
            class="file-item"
            :class="{ active: f.id === selectedId }"
            @click="select(f)"
          >
            <div class="file-item-title">🔒 {{ f.title }}</div>
            <div class="file-item-meta">
              <n-tag size="tiny" :bordered="false">{{ categoryLabel(f.category) }}</n-tag>
              <span>{{ fmtSize(f.size) }}</span>
            </div>
            <div class="file-item-time">{{ fmtTime(f.updatedAt) }}</div>
          </div>
        </div>
      </div>

      <!-- Right: details / editor -->
      <div class="files-detail">
        <template v-if="selected">
          <div class="detail-head">
            <n-input v-model:value="editorTitle" :placeholder="t('files.name')" size="small" style="max-width: 320px" />
            <n-space size="small" align="center" :wrap="false">
              <n-popconfirm @positive-click="removeFile">
                <template #trigger>
                  <n-button size="small" quaternary type="error">{{ t('files.delete') }}</n-button>
                </template>
                {{ t('files.deleteConfirm') }}
              </n-popconfirm>
              <n-button size="small" tertiary @click="exportFile">{{ t('files.export') }}</n-button>
              <n-button size="small" type="primary" :loading="saving" @click="saveMeta">{{ t('files.save') }}</n-button>
            </n-space>
          </div>

          <n-space align="center" size="small" style="margin-top: 8px">
            <n-select v-model:value="editorCategory" :options="editCategories" size="small" style="width: 180px" />
            <n-tag size="small" :bordered="false">🔒 {{ t('files.encrypted') }}</n-tag>
            <n-tag size="small" :bordered="false">{{ storageLabel(selected.size) }}</n-tag>
            <span class="files-muted">{{ fmtSize(selected.size) }}</span>
          </n-space>

          <n-input
            v-model:value="editorNote"
            type="textarea"
            :autosize="{ minRows: 2 }"
            :placeholder="t('files.notePlaceholder')"
            style="margin-top: 8px"
          />

          <div class="preview-box">
            <div v-if="previewLoading" class="files-muted">{{ t('files.loading') }}</div>
            <img v-else-if="previewUrl" :src="previewUrl" class="preview-img" alt="preview" />
            <PdfViewer v-else-if="isPdf(previewMime) && previewData" :data="previewData" />
            <div v-else class="files-muted">{{ t('files.previewUnsupported') }}</div>
          </div>

          <div class="files-muted detail-meta">
            {{ t('files.createdAt') }} {{ fmtTime(selected.createdAt) }} · {{ t('files.updatedAt') }} {{ fmtTime(selected.updatedAt) }}
          </div>
        </template>
        <n-empty v-else :description="t('files.pick')" style="padding: 60px 0" />
      </div>
    </div>

    <n-modal v-model:show="catManageOpen">
      <n-card :title="t('files.catManage')" style="width: 420px" :bordered="false" size="small">
        <n-space align="center" size="small" style="margin-bottom: 12px">
          <n-input
            v-model:value="catNewName"
            :placeholder="t('files.catNamePlaceholder')"
            size="small"
            @keyup.enter="createCategory"
          />
          <n-button size="small" type="primary" @click="createCategory">{{ t('files.catAdd') }}</n-button>
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
              <n-button size="tiny" type="primary" @click="submitRenameCategory">{{ t('files.save') }}</n-button>
            </template>
            <template v-else>
              <n-button size="tiny" quaternary @click="startRenameCategory(c)">{{ t('files.catRename') }}</n-button>
              <n-popconfirm @positive-click="deleteCategory(c.id)">
                <template #trigger>
                  <n-button size="tiny" quaternary type="error">{{ t('files.catDelete') }}</n-button>
                </template>
                {{ t('files.catDeleteConfirm') }}
              </n-popconfirm>
            </template>
          </n-space>
        </div>
      </n-card>
    </n-modal>
  </div>
</template>

<style scoped>
.files-page {
  display: flex;
  flex-direction: column;
}
.files-hint {
  font-size: 12px;
  opacity: 0.5;
  margin-bottom: 10px;
}
.files-muted {
  font-size: 12px;
  opacity: 0.5;
}
.cat-filter-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
}
.cat-filter-row :deep(.n-select) {
  flex: 1;
}
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
.files-layout {
  display: flex;
  gap: 14px;
  flex: 1;
  min-height: 0;
}
.files-list {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.files-list-scroll {
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-right: 2px;
}
.file-item {
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}
.file-item:hover {
  background: rgba(255, 255, 255, 0.07);
}
.file-item.active {
  background: rgba(99, 226, 183, 0.1);
  border-color: rgba(99, 226, 183, 0.4);
}
.file-item-title {
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.file-item-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  opacity: 0.6;
  margin-top: 4px;
}
.file-item-time {
  font-size: 11px;
  opacity: 0.4;
  margin-top: 3px;
}
.files-detail {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.preview-box {
  margin-top: 10px;
  flex: 1;
  min-height: 120px;
  overflow: auto;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  padding: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.preview-img {
  max-width: 100%;
  max-height: 60vh;
  border-radius: 6px;
}
.detail-meta {
  margin-top: 8px;
}
</style>
