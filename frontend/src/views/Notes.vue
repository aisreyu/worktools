<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NButtonGroup,
  NEmpty,
  NInput,
  NPopconfirm,
  NSpace,
  NSwitch,
  useMessage,
} from 'naive-ui'
import { NoteService, ExportService, type NoteEntry } from '../lib/api'
import { requestSavePath, publishSaved } from '../lib/saveFile'
import { renderMarkdown, wrapHtmlDocument } from '../lib/markdown'
import { t, errText } from '../lib/i18n'
import { useRouteHighlight } from '../lib/routeHighlight'

useRouteHighlight()

const message = useMessage()

const notes = ref<NoteEntry[]>([])
const loading = ref(false)
const search = ref('')
const selectedId = ref<number | null>(null)
const editorTitle = ref('')
const editorContent = ref('')
const creating = ref(false)
const saving = ref(false)
const showDeleteConfirm = ref(false)
const viewMode = ref<'edit' | 'preview' | 'split'>('split')
const editorSensitive = ref(false)
const editorAttachment = ref('')
const editorAttachmentName = ref('')
const editorAttachmentMime = ref('')
const attachInput = ref<HTMLInputElement | null>(null)

async function load() {
  loading.value = true
  try {
    notes.value = ((await NoteService.List()) ?? []) as NoteEntry[]
    if (selectedId.value !== null && !notes.value.some((n) => n.id === selectedId.value)) {
      selectedId.value = notes.value[0]?.id ?? null
      syncEditorFromSelection()
    }
  } catch (e) {
    message.error(errText(e))
  } finally {
    loading.value = false
  }
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return notes.value
  return notes.value.filter(
    (n) => n.title.toLowerCase().includes(q) || n.content.toLowerCase().includes(q),
  )
})

const selected = computed(() => notes.value.find((n) => n.id === selectedId.value) ?? null)

const previewHtml = computed(() => renderMarkdown(editorContent.value))

function select(n: NoteEntry) {
  selectedId.value = n.id
  creating.value = false
  syncEditorFromSelection()
}

function syncEditorFromSelection() {
  const s = selected.value
  editorTitle.value = s?.title ?? ''
  editorContent.value = s?.content ?? ''
  editorSensitive.value = s?.sensitive ?? false
  editorAttachment.value = s?.attachment ?? ''
  editorAttachmentName.value = s?.attachmentName ?? ''
  editorAttachmentMime.value = s?.attachmentMime ?? ''
}

function createNote() {
  selectedId.value = null
  editorTitle.value = ''
  editorContent.value = ''
  editorSensitive.value = false
  editorAttachment.value = ''
  editorAttachmentName.value = ''
  editorAttachmentMime.value = ''
  creating.value = true
}

const attachmentUrl = computed(() => {
  if (!editorAttachment.value) return ''
  return `data:${editorAttachmentMime.value || 'image/png'};base64,${editorAttachment.value}`
})

async function saveNote() {
  if (!editorTitle.value.trim() && !editorContent.value.trim()) {
    message.warning(t('notes.titleOrContentRequired'))
    return
  }
  saving.value = true
  try {
    const id = await NoteService.Save({
      id: selectedId.value ?? 0,
      title: editorTitle.value,
      content: editorContent.value,
      pinned: selected.value?.pinned ?? false,
      sensitive: editorSensitive.value,
      attachment: editorAttachment.value,
      attachmentName: editorAttachmentName.value,
      attachmentMime: editorAttachmentMime.value,
      createdAt: 0,
      updatedAt: 0,
    })
    selectedId.value = Number(id)
    creating.value = false
    await load()
    message.success(t('notes.saved'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    saving.value = false
  }
}

const MAX_ATTACH_BYTES = 4 * 1024 * 1024

/** Read an image file into a base64 attachment (data portion only). */
function readImageFile(file: File) {
  if (file.size > MAX_ATTACH_BYTES) {
    message.warning(t('notes.attachTooLarge'))
    return
  }
  if (!file.type.startsWith('image/')) return
  const reader = new FileReader()
  reader.onload = () => {
    const result = String(reader.result)
    const comma = result.indexOf(',')
    editorAttachment.value = comma >= 0 ? result.slice(comma + 1) : result
    editorAttachmentMime.value = file.type
    editorAttachmentName.value = file.name || 'pasted-image'
  }
  reader.readAsDataURL(file)
}

function onPasteImage(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return
  for (let i = 0; i < items.length; i++) {
    const it = items[i]
    if (it.type.startsWith('image/')) {
      const file = it.getAsFile()
      if (file) {
        readImageFile(file)
        e.preventDefault()
      }
      return
    }
  }
}

function onDropImage(e: DragEvent) {
  const file = e.dataTransfer?.files?.[0]
  if (file) {
    readImageFile(file)
    e.preventDefault()
  }
}

function onPickImage(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) readImageFile(file)
  input.value = ''
}

function removeAttachment() {
  editorAttachment.value = ''
  editorAttachmentName.value = ''
  editorAttachmentMime.value = ''
}

async function togglePin(n: NoteEntry) {
  try {
    await NoteService.Save({ ...n, pinned: !n.pinned })
    await load()
  } catch (e) {
    message.error(errText(e))
  }
}

async function removeNote() {
  if (selectedId.value === null) return
  try {
    await NoteService.Delete(selectedId.value)
    selectedId.value = null
    await load()
    message.success(t('notes.deleted'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    showDeleteConfirm.value = false
  }
}

function safeName(): string {
  return (editorTitle.value.trim() || 'note').replace(/[\\/:*?"<>|]/g, '_')
}

async function exportNote() {
  if (selectedId.value === null && !creating.value) return
  if (!editorContent.value.trim()) {
    message.warning(t('notes.emptyNoExport'))
    return
  }
  const name = safeName() + '.md'
  const path = await requestSavePath(t('notes.exportNote'), name, [
    { DisplayName: t('notes.markdownFile'), Pattern: '*.md' },
    { DisplayName: t('notes.htmlFile'), Pattern: '*.html' },
  ])
  if (!path) return
  try {
    let content = editorContent.value
    if (path.endsWith('.html') || path.endsWith('.htm')) {
      content = wrapHtmlDocument(editorTitle.value.trim() || t('notes.newNoteTitle'), previewHtml.value)
    }
    await ExportService.SaveText(path, content)
    const finalName = path.replace(/\\/g, '/').split('/').pop() || name
    const where = await publishSaved(path, finalName)
    message.success(t('notes.exported') + (where || path))
  } catch (e) {
    message.error(t('notes.exportFailed') + errText(e))
  }
}

function exportPdf() {
  if (selectedId.value === null && !creating.value) return
  if (!editorContent.value.trim()) {
    message.warning(t('notes.emptyNoExport'))
    return
  }
  const html = wrapHtmlDocument(editorTitle.value.trim() || t('notes.newNoteTitle'), previewHtml.value)
  const w = window.open('', '_blank')
  if (!w) {
    message.error(t('notes.printBlocked'))
    return
  }
  w.document.write(html)
  w.document.close()
  // Give the new window a moment to lay out, then open the print dialog
  // (user picks "Save as PDF" as the destination).
  w.focus()
  setTimeout(() => w.print(), 300)
}

function fmtTime(ts: number): string {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

onMounted(load)
</script>

<template>
  <div class="page notes-page">
    <div class="row-between page-title">
      <span>{{ t('notes.title') }}</span>
      <n-button type="primary" size="small" @click="createNote">{{ t('notes.newNote') }}</n-button>
    </div>

    <div class="notes-layout">
      <!-- Left: note list -->
      <div class="notes-list">
        <n-input v-model:value="search" :placeholder="t('notes.searchPlaceholder')" clearable size="small" round style="margin-bottom: 8px" />
        <div v-if="loading" style="opacity: 0.6; padding: 12px 4px">{{ t('notes.loading') }}</div>
        <n-empty v-else-if="filtered.length === 0" :description="t('notes.empty')" style="padding: 24px 0" />
        <div v-else class="notes-list-scroll">
          <div
            v-for="n in filtered"
            :key="n.id"
            class="note-item"
            :class="{ active: n.id === selectedId }"
            :data-hid="n.id"
            @click="select(n)"
          >
            <div class="note-item-title">
              <span class="note-pin" v-if="n.pinned" :title="t('notes.pinned')">📌</span>
              <span class="note-pin" v-if="n.sensitive" :title="t('notes.sensitive')">🔒</span>
              <span class="note-item-name">{{ n.title || t('notes.untitled') }}</span>
            </div>
            <div class="note-item-preview">
              {{ n.sensitive ? t('notes.sensitive') : (n.content.slice(0, 60) || t('notes.emptyContent')) }}
            </div>
            <div class="note-item-time">{{ fmtTime(n.updatedAt) }}</div>
          </div>
        </div>
      </div>

      <!-- Right: editor -->
      <div class="notes-editor">
        <template v-if="selectedId !== null || creating">
          <div class="editor-head">
            <n-input v-model:value="editorTitle" :placeholder="creating ? t('notes.newNoteTitle') : t('notes.titlePlaceholder')" size="small" style="max-width: 300px" />
            <n-space size="small" align="center" :wrap="false">
              <n-button-group size="small">
                <n-button size="tiny" :type="viewMode === 'edit' ? 'primary' : 'default'" @click="viewMode = 'edit'">{{ t('notes.edit') }}</n-button>
                <n-button size="tiny" :type="viewMode === 'split' ? 'primary' : 'default'" @click="viewMode = 'split'">{{ t('notes.split') }}</n-button>
                <n-button size="tiny" :type="viewMode === 'preview' ? 'primary' : 'default'" @click="viewMode = 'preview'">{{ t('notes.preview') }}</n-button>
              </n-button-group>
              <n-button v-if="selected" size="small" quaternary @click="togglePin(selected)">
                {{ selected.pinned ? t('notes.unpin') : t('notes.pin') }}
              </n-button>
              <span class="sensitive-toggle" :title="t('notes.sensitiveHint')">
                <n-switch v-model:value="editorSensitive" size="small" />
                🔒 {{ t('notes.sensitive') }}
              </span>
              <n-button size="small" quaternary @click="attachInput?.click()">
                🖼 {{ t('notes.attachAdd') }}
              </n-button>
              <n-button size="small" quaternary @click="removeAttachment" v-if="editorAttachment">
                {{ t('notes.attachRemove') }}
              </n-button>
              <input
                ref="attachInput"
                type="file"
                accept="image/*"
                style="display: none"
                @change="onPickImage"
              />
              <n-popconfirm v-if="selected" v-model:show="showDeleteConfirm" @positive-click="removeNote">
                <template #trigger>
                  <n-button size="small" quaternary type="error">{{ t('notes.delete') }}</n-button>
                </template>
                {{ t('notes.confirmDelete') }}
              </n-popconfirm>
              <n-button size="small" tertiary @click="exportNote">{{ t('notes.export') }}</n-button>
              <n-button size="small" tertiary @click="exportPdf">{{ t('notes.exportPdf') }}</n-button>
              <n-button size="small" type="primary" :loading="saving" @click="saveNote">{{ t('notes.save') }}</n-button>
            </n-space>
          </div>
          <div
            class="editor-body"
            :class="{ split: viewMode === 'split' }"
            @paste="onPasteImage"
            @dragover.prevent
            @drop="onDropImage"
          >
            <div v-if="viewMode !== 'preview'" class="editor-col">
              <n-input
                v-model:value="editorContent"
                type="textarea"
                :placeholder="creating ? t('notes.mdHint') : t('notes.mdHintPlain')"
                class="editor-input"
              />
              <div v-if="editorAttachment" class="attach-box">
                <div class="attach-head">
                  <span>{{ t('notes.attachment') }} · {{ editorAttachmentName || 'image' }}</span>
                  <n-button size="tiny" quaternary type="error" @click="removeAttachment">
                    {{ t('notes.attachRemove') }}
                  </n-button>
                </div>
                <img :src="attachmentUrl" class="attach-img" alt="attachment" />
              </div>
              <div v-else class="attach-hint">{{ t('notes.attachHint') }}</div>
            </div>
            <div v-if="viewMode !== 'edit'" class="md-preview" v-html="previewHtml"></div>
          </div>
          <div class="editor-meta" v-if="selected">
            {{ t('notes.createdAt') }} {{ fmtTime(selected.createdAt) }} · {{ t('notes.updatedAt') }} {{ fmtTime(selected.updatedAt) }}
          </div>
        </template>
        <n-empty v-else :description="t('notes.selectOrCreate')" style="padding: 60px 0" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.notes-page {
  display: flex;
  flex-direction: column;
}
.notes-layout {
  display: flex;
  gap: 14px;
  flex: 1;
  min-height: 0;
}
.notes-list {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.notes-list-scroll {
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-right: 2px;
}
.note-item {
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}
.note-item:hover {
  background: rgba(255, 255, 255, 0.07);
}
.note-item.active {
  background: rgba(99, 226, 183, 0.1);
  border-color: rgba(99, 226, 183, 0.4);
}
.note-item-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.note-pin {
  font-size: 12px;
}
.note-item-preview {
  font-size: 12px;
  opacity: 0.55;
  margin-top: 3px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.note-item-time {
  font-size: 11px;
  opacity: 0.4;
  margin-top: 3px;
}
.notes-editor {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
}
.editor-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.editor-body {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 10px;
}
.editor-col {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
}
.sensitive-toggle {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  opacity: 0.8;
}
.attach-box {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 8px 10px;
  background: rgba(255, 255, 255, 0.03);
  max-height: 30%;
  overflow: auto;
}
.attach-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
  opacity: 0.6;
  margin-bottom: 6px;
}
.attach-img {
  max-width: 100%;
  border-radius: 6px;
}
.attach-hint {
  font-size: 11px;
  opacity: 0.35;
}
.editor-input {
  flex: 1;
  min-width: 0;
}
.md-preview {
  flex: 1;
  min-width: 0;
  overflow: auto;
  padding: 4px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  font-size: 14px;
  line-height: 1.7;
}
.md-preview :deep(h1),
.md-preview :deep(h2) {
  border-bottom: 1px solid rgba(255, 255, 255, 0.12);
  padding-bottom: 6px;
}
.md-preview :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  background: rgba(255, 255, 255, 0.08);
  padding: 2px 5px;
  border-radius: 4px;
  font-size: 0.9em;
}
.md-preview :deep(pre) {
  background: rgba(0, 0, 0, 0.35);
  padding: 12px;
  border-radius: 8px;
  overflow-x: auto;
}
.md-preview :deep(pre code) {
  background: none;
  padding: 0;
}
.md-preview :deep(blockquote) {
  border-left: 4px solid rgba(99, 226, 183, 0.5);
  margin: 0;
  padding: 4px 14px;
  opacity: 0.8;
  background: rgba(255, 255, 255, 0.04);
}
.md-preview :deep(table) {
  border-collapse: collapse;
}
.md-preview :deep(th),
.md-preview :deep(td) {
  border: 1px solid rgba(255, 255, 255, 0.15);
  padding: 6px 12px;
}
.md-preview :deep(th) {
  background: rgba(255, 255, 255, 0.06);
}
.md-preview :deep(img) {
  max-width: 100%;
}
.md-preview :deep(a) {
  color: #63e2b7;
}
.editor-meta {
  font-size: 11px;
  opacity: 0.4;
}
</style>
