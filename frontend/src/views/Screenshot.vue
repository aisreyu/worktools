<script setup lang="ts">
// Screenshot workbench: full-screen capture, region capture (via the
// transparent overlay windows), scrolling capture with stitching and OCR.
//
// Region selections happen in a separate overlay window; its result arrives on
// the app event bus (wt:shot:result / wt:shot:cancel / wt:shot:error), which we
// subscribe to here.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NInput,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSpace,
  NSpin,
  useMessage,
} from 'naive-ui'
import { Events } from '@wailsio/runtime'
import { ScreenshotService, type ShotResult, type ScreenshotInfo, type OCRResult } from '../lib/api'
import { errText, t } from '../lib/i18n'
import { requestSavePath, publishSaved, savedLocationSuffix } from '../lib/saveFile'

const message = useMessage()

const info = ref<ScreenshotInfo | null>(null)
const busy = ref(false)
const shot = ref<ShotResult | null>(null)

function shotUrl(s: ShotResult | null): string {
  return s ? `data:image/png;base64,${s.data}` : ''
}

async function loadInfo() {
  try {
    info.value = await ScreenshotService.Info()
  } catch (e) {
    message.error(errText(e))
  }
}

function setShot(s: ShotResult) {
  shot.value = s
}

/* ---- full screen ---- */
async function capturePrimary() {
  busy.value = true
  try {
    setShot(await ScreenshotService.CapturePrimary())
  } catch (e) {
    message.error(errText(e))
  } finally {
    busy.value = false
  }
}

/* ---- region (overlay) ---- */
const regionMode = ref<'image' | 'ocr'>('image')

async function beginRegion(mode: 'image' | 'ocr') {
  regionMode.value = mode
  try {
    await ScreenshotService.BeginRegion()
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- scroll capture ---- */
const scrollArmed = ref(false)
const frames = ref<ShotResult[]>([])
const stitchMode = ref<'scroll' | 'append'>('scroll')
const stitched = ref<string>('')

async function armScroll() {
  scrollArmed.value = true
  frames.value = []
  stitched.value = ''
  await beginRegion('image')
}

async function nextFrame() {
  if (!scrollArmed.value) return
  busy.value = true
  try {
    const [r, scale] = await ScreenshotService.LastRegion()
    // LastRegion returns [rect, 0] when nothing is recorded yet; fall back to
    // the primary screen so the very first frame always works.
    if (!r || r.width < 1 || r.height < 1) {
      setShot(await ScreenshotService.CapturePrimary())
      frames.value.push(shot.value as ShotResult)
    } else {
      const next = await ScreenshotService.CaptureRect(r, scale || 1)
      frames.value.push(next)
    }
  } catch (e) {
    message.error(errText(e))
  } finally {
    busy.value = false
  }
}

async function doStitch() {
  if (frames.value.length < 2) {
    message.warning(t('shot.frames', { n: frames.value.length }))
    return
  }
  busy.value = true
  try {
    const res = await ScreenshotService.StitchFrames(
      frames.value.map((f) => f.data),
      stitchMode.value,
    )
    stitched.value = res.data
    setShot({ data: res.data, width: res.width, height: res.height, scale: 1, path: '' })
    message.success(t('shot.done'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    busy.value = false
  }
}

function clearFrames() {
  frames.value = []
  stitched.value = ''
}

/* ---- OCR ---- */
const ocrText = ref('')
const ocrResult = ref<OCRResult | null>(null)
const ocrLang = ref('eng')
const ocrBusy = ref(false)

const ocrLangs = computed(() => {
  const langs = (info.value?.ocr?.langs as string[] | undefined) ?? []
  if (langs.length === 0) return [{ label: 'English', value: 'eng' }]
  return langs.map((l) => ({ label: l, value: l }))
})

const ocrAvailable = computed(() => Boolean(info.value?.ocr?.available))

function setOcr(res: OCRResult) {
  ocrResult.value = res
  ocrText.value = res.text
}

async function ocrCurrent() {
  if (!shot.value) return
  ocrBusy.value = true
  try {
    setOcr(await ScreenshotService.OCRText(shot.value.data, ocrLang.value))
  } catch (e) {
    message.error(errText(e))
  } finally {
    ocrBusy.value = false
  }
}

async function ocrRegion() {
  await beginRegion('ocr')
}

/* ---- save / copy ---- */
async function saveImage() {
  if (!shot.value) return
  try {
    const filename = `screenshot-${Date.now()}.png`
    const path = await requestSavePath(t('shot.save'), filename, [
      { DisplayName: 'PNG', Pattern: '*.png' },
    ])
    if (!path) return
    const written = await ScreenshotService.SaveImage(path, shot.value.data)
    const where = await publishSaved(written, filename)
    message.success(t('shot.saved') + savedLocationSuffix(where))
  } catch (e) {
    message.error(errText(e))
  }
}

async function copyImage() {
  if (!shot.value) return
  try {
    await ScreenshotService.CopyToClipboard(shot.value.data)
    message.success(t('shot.copied'))
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- events from the overlay windows ---- */
let offResult: (() => void) | null = null
let offCancel: (() => void) | null = null
let offError: (() => void) | null = null

function onShotResult(data: ShotResult) {
  setShot(data)
  if (regionMode.value === 'ocr') void runOcrOnCurrent()
}

async function runOcrOnCurrent() {
  if (!shot.value) return
  ocrBusy.value = true
  try {
    setOcr(await ScreenshotService.OCRText(shot.value.data, ocrLang.value))
  } catch (e) {
    message.error(errText(e))
  } finally {
    ocrBusy.value = false
  }
}

onMounted(async () => {
  offResult = Events.On('wt:shot:result', (ev: { data?: ShotResult } | ShotResult) => {
    const payload = (ev as { data?: ShotResult }).data ?? (ev as ShotResult)
    if (payload && payload.data) onShotResult(payload)
  })
  offCancel = Events.On('wt:shot:cancel', () => {
    /* selection cancelled; nothing to do */
  })
  offError = Events.On('wt:shot:error', (ev: { data?: string } | string) => {
    const msg = typeof ev === 'string' ? ev : ev?.data ?? t('shot.failed')
    message.error(msg)
  })
  await loadInfo()
})

onUnmounted(() => {
  offResult?.()
  offCancel?.()
  offError?.()
})
</script>

<template>
  <div class="page">
    <h2 class="page-title">{{ t('shot.title') }}</h2>
    <p class="muted">{{ t('shot.subtitle') }}</p>

    <n-alert v-if="info && !info.supported" type="warning" style="margin-bottom: 12px">
      {{ t('shot.unsupported') }}
    </n-alert>

    <n-card size="small" style="margin-bottom: 14px">
      <n-space>
        <n-button type="primary" :disabled="busy" @click="capturePrimary">
          🖥 {{ t('shot.fullScreen') }}
        </n-button>
        <n-button :disabled="busy" @click="beginRegion('image')">
          ▭ {{ t('shot.region') }}
        </n-button>
        <n-button :disabled="busy" @click="ocrRegion">
          🔤 {{ t('shot.ocr') }}
        </n-button>
        <n-button :disabled="busy" @click="armScroll">
          ⇕ {{ t('shot.scroll') }}
        </n-button>
      </n-space>
      <div class="muted small" style="margin-top: 8px">
        {{ t('shot.regionHint') }} · {{ t('shot.scrollHint') }}
      </div>
    </n-card>

    <div class="layout">
      <!-- preview -->
      <n-card size="small" :title="t('shot.preview')" class="preview-card">
        <n-spin :show="busy">
          <div v-if="shot" class="preview-wrap">
            <img class="preview-img" :src="shotUrl(shot)" :alt="t('shot.preview')" />
          </div>
          <div v-else class="empty">{{ t('shot.empty') }}</div>
        </n-spin>
        <template v-if="shot" #action>
          <n-space>
            <n-button size="small" @click="copyImage">📋 {{ t('shot.copy') }}</n-button>
            <n-button size="small" type="primary" @click="saveImage">💾 {{ t('shot.saveAs') }}</n-button>
          </n-space>
        </template>
      </n-card>

      <!-- side panel -->
      <div class="side">
        <!-- scrolling capture -->
        <n-card size="small" :title="t('shot.scroll')">
          <n-space vertical size="small">
            <n-radio-group v-model:value="stitchMode" size="small">
              <n-radio-button value="scroll">{{ t('shot.modeScroll') }}</n-radio-button>
              <n-radio-button value="append">{{ t('shot.modeAppend') }}</n-radio-button>
            </n-radio-group>
            <div class="muted small">{{ t('shot.frames', { n: frames.length }) }}</div>
            <n-space>
              <n-button size="small" :disabled="!scrollArmed || busy" @click="nextFrame">
                ⤓ {{ t('shot.addFrame') }}
              </n-button>
              <n-button size="small" type="primary" :disabled="frames.length < 2 || busy" @click="doStitch">
                🧵 {{ t('shot.stitch') }}
              </n-button>
              <n-button size="small" quaternary :disabled="busy" @click="clearFrames">
                {{ t('shot.clearFrames') }}
              </n-button>
            </n-space>
          </n-space>
        </n-card>

        <!-- OCR -->
        <n-card size="small" :title="t('shot.ocr')" style="margin-top: 14px">
          <n-alert v-if="!ocrAvailable" type="info" :show-icon="false" style="margin-bottom: 8px">
            {{ t('shot.noOcr') }}
          </n-alert>
          <n-space vertical size="small">
            <n-space align="center">
              <span class="muted small">{{ t('shot.ocrLang') }}</span>
              <n-select v-model:value="ocrLang" size="small" style="width: 140px" :options="ocrLangs" />
            </n-space>
            <n-space>
              <n-button size="small" :disabled="!shot || ocrBusy" @click="ocrCurrent">
                {{ ocrBusy ? t('shot.ocrRunning') : t('shot.ocrRun') }}
              </n-button>
              <n-button size="small" :disabled="ocrBusy" @click="ocrRegion">
                {{ t('shot.ocr') }}
              </n-button>
            </n-space>
            <div v-if="ocrResult" class="muted small">
              {{ t('shot.engine') }}: {{ ocrResult.engine }} ·
              {{ t('shot.ocrLines', { n: ocrResult.lines, m: ocrResult.chars }) }}
            </div>
            <n-input
              v-model:value="ocrText"
              type="textarea"
              :rows="6"
              :placeholder="t('shot.ocrEmpty')"
            />
          </n-space>
        </n-card>
      </div>
    </div>
  </div>
</template>

<style scoped>
.muted {
  color: var(--nb-muted, rgba(128, 128, 128, 0.9));
}
.small {
  font-size: 12px;
}
.layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 14px;
  align-items: start;
}
@media (max-width: 900px) {
  .layout {
    grid-template-columns: 1fr;
  }
}
.preview-card :deep(.n-card__content) {
  min-height: 240px;
}
.preview-wrap {
  display: flex;
  justify-content: center;
  max-height: 60vh;
  overflow: auto;
}
.preview-img {
  max-width: 100%;
  border: 1px solid rgba(128, 128, 128, 0.25);
  border-radius: 6px;
}
.empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 240px;
  color: rgba(128, 128, 128, 0.8);
}
</style>
