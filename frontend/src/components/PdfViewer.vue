<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { NButton, NSpin, NSpace } from 'naive-ui'
// Use the legacy build: it bundles core-js polyfills (e.g. Promise.withResolvers)
// that the Wails webview runtime lacks, unlike the modern build.
import * as pdfjsLib from 'pdfjs-dist/legacy/build/pdf.mjs'
import workerUrl from 'pdfjs-dist/legacy/build/pdf.worker.min.mjs?url'

pdfjsLib.GlobalWorkerOptions.workerSrc = workerUrl

const props = defineProps<{ data: string }>()

const canvas = ref<HTMLCanvasElement | null>(null)
const loading = ref(false)
const error = ref('')
const page = ref(1)
const pages = ref(0)
const scale = ref(1.1)

// Holds the live PDFDocumentProxy so we can free its worker on teardown.
type PdfDoc = { numPages: number; getPage: (n: number) => Promise<unknown>; destroy: () => void }
let doc: PdfDoc | null = null

function base64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}

async function render() {
  if (!doc || !canvas.value) return
  loading.value = true
  try {
    const pdfPage = (await doc.getPage(page.value)) as {
      getViewport: (o: { scale: number }) => { width: number; height: number }
      render: (o: unknown) => { promise: Promise<void> }
    }
    const viewport = pdfPage.getViewport({ scale: scale.value })
    const el = canvas.value
    el.width = viewport.width
    el.height = viewport.height
    const ctx = el.getContext('2d')
    if (!ctx) return
    ctx.clearRect(0, 0, el.width, el.height)
    await pdfPage.render({ canvasContext: ctx, viewport }).promise
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

async function load() {
  error.value = ''
  pages.value = 0
  page.value = 1
  if (doc) {
    try {
      doc.destroy()
    } catch {
      /* ignore */
    }
    doc = null
  }
  if (!props.data) return
  loading.value = true
  try {
    const task = pdfjsLib.getDocument({ data: base64ToBytes(props.data) })
    doc = (await task.promise) as unknown as PdfDoc
    pages.value = doc?.numPages ?? 0
    await render()
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

function prev() {
  if (page.value > 1) {
    page.value--
    render()
  }
}

function next() {
  if (page.value < pages.value) {
    page.value++
    render()
  }
}

function zoom(delta: number) {
  const s = Math.min(3, Math.max(0.4, scale.value + delta))
  scale.value = Math.round(s * 100) / 100
  render()
}

watch(() => props.data, load, { immediate: true })
onBeforeUnmount(() => {
  if (doc) {
    try {
      doc.destroy()
    } catch {
      /* ignore */
    }
  }
})
</script>

<template>
  <div class="pdf-viewer">
    <div v-if="error" class="pdf-error">{{ error }}</div>
    <template v-else>
      <n-space align="center" size="small" class="pdf-bar">
        <n-button size="tiny" :disabled="page <= 1" @click="prev">‹</n-button>
        <span class="pdf-counter">{{ page }} / {{ pages || '…' }}</span>
        <n-button size="tiny" :disabled="page >= pages" @click="next">›</n-button>
        <n-button size="tiny" @click="zoom(-0.2)">−</n-button>
        <span class="pdf-counter">{{ Math.round(scale * 100) }}%</span>
        <n-button size="tiny" @click="zoom(0.2)">＋</n-button>
      </n-space>
      <div class="pdf-canvas-wrap">
        <n-spin v-if="loading" size="small" />
        <canvas ref="canvas" class="pdf-canvas"></canvas>
      </div>
    </template>
  </div>
</template>

<style scoped>
.pdf-viewer {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
}
.pdf-bar {
  margin-bottom: 8px;
}
.pdf-counter {
  font-size: 12px;
  opacity: 0.7;
  min-width: 44px;
  text-align: center;
}
.pdf-canvas-wrap {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 120px;
}
.pdf-canvas {
  max-width: 100%;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.35);
  border-radius: 4px;
}
.pdf-error {
  font-size: 12px;
  opacity: 0.6;
}
</style>
