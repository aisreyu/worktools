<script setup lang="ts">
// Full-screen transparent overlay used for region capture.
//
// One overlay window is created per display by the Go ScreenshotService at the
// URL /#/overlay?idx=N. The user drags a rectangle, which is translated from
// window-local coordinates into global DIP coordinates (by adding this screen's
// origin) and handed back to CommitRegion. Esc cancels.
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { ScreenshotService, type ScreenInfo } from '../lib/api'

const screen = ref<ScreenInfo | null>(null)
const dragging = ref(false)
const hasSel = ref(false)
const hint = ref(true)
const sel = reactive({ x: 0, y: 0, w: 0, h: 0 })

let startX = 0
let startY = 0
let hintTimer: number | null = null

function clamp(v: number, lo: number, hi: number): number {
    return Math.max(lo, Math.min(hi, v))
}

function readIndex(): number {
    const hash = window.location.hash
    const q = hash.indexOf('?')
    if (q < 0) return 0
    const params = new URLSearchParams(hash.slice(q + 1))
    const n = Number(params.get('idx'))
    return Number.isFinite(n) ? n : 0
}

async function loadScreen() {
    const idx = readIndex()
    try {
        const list = (await ScreenshotService.Screens()) ?? []
        screen.value = list[idx] ?? list[0] ?? null
    } catch {
        screen.value = null
    }
}

function beginDrag(e: MouseEvent) {
    if (e.button !== 0) return
    dragging.value = true
    hasSel.value = false
    startX = e.clientX
    startY = e.clientY
    sel.x = startX
    sel.y = startY
    sel.w = 0
    sel.h = 0
}

function moveDrag(e: MouseEvent) {
    if (!dragging.value) return
    const x = clamp(e.clientX, 0, window.innerWidth)
    const y = clamp(e.clientY, 0, window.innerHeight)
    sel.x = Math.min(startX, x)
    sel.y = Math.min(startY, y)
    sel.w = Math.abs(x - startX)
    sel.h = Math.abs(y - startY)
    hasSel.value = sel.w > 1 || sel.h > 1
}

async function endDrag() {
    if (!dragging.value) return
    dragging.value = false
    if (sel.w < 3 || sel.h < 3) {
        // A click without a real drag is treated as "cancel this selection".
        await cancel()
        return
    }
    const s = screen.value
    const rect = {
        x: Math.round(sel.x + (s?.x ?? 0)),
        y: Math.round(sel.y + (s?.y ?? 0)),
        width: Math.round(sel.w),
        height: Math.round(sel.h),
    }
    try {
        await ScreenshotService.CommitRegion(rect, s?.scale ?? 1)
    } catch {
        /* the main window will surface any error */
    }
}

async function cancel() {
    try {
        await ScreenshotService.CancelRegion()
    } catch {
        /* ignore */
    }
}

function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') {
        e.preventDefault()
        void cancel()
    }
}

// Keep the whole page transparent so only the selection UI is visible.
let prevBodyBg = ''
let prevHtmlBg = ''

onMounted(() => {
    prevBodyBg = document.body.style.background
    prevHtmlBg = document.documentElement.style.background
    document.documentElement.style.background = 'transparent'
    document.body.style.background = 'transparent'
    document.body.style.cursor = 'crosshair'
    window.addEventListener('keydown', onKey)
    void loadScreen()
    hintTimer = window.setTimeout(() => (hint.value = false), 2600)
})

onUnmounted(() => {
    document.documentElement.style.background = prevHtmlBg
    document.body.style.background = prevBodyBg
    document.body.style.cursor = ''
    window.removeEventListener('keydown', onKey)
    if (hintTimer !== null) window.clearTimeout(hintTimer)
})
</script>

<template>
    <div class="overlay-root" @mousedown="beginDrag" @mousemove="moveDrag" @mouseup="endDrag">
        <!-- Dimming mask with a punched-out selection rectangle. -->
        <template v-if="hasSel">
            <div class="mask" :style="{ left: 0, top: 0, width: '100%', height: sel.y + 'px' }" />
            <div class="mask" :style="{ left: 0, top: sel.y + 'px', width: sel.x + 'px', height: sel.h + 'px' }" />
            <div class="mask" :style="{
                left: sel.x + 'px',
                top: sel.y + 'px',
                width: sel.w + 'px',
                height: sel.h + 'px',
                background: 'transparent',
            }" />
            <div class="mask" :style="{
                left: sel.x + 'px',
                top: sel.y + sel.h + 'px',
                width: '100%',
                height: 'calc(100% - ' + (sel.y + sel.h) + 'px)',
            }" />
            <div class="mask" :style="{
                left: sel.x + sel.w + 'px',
                top: sel.y + 'px',
                width: 'calc(100% - ' + (sel.x + sel.w) + 'px)',
                height: sel.h + 'px',
            }" />
            <div class="sel-border"
                :style="{ left: sel.x + 'px', top: sel.y + 'px', width: sel.w + 'px', height: sel.h + 'px' }">
                <div class="sel-size" :style="{
                    left: '0px',
                    top: sel.y < 26 ? (sel.h + 6) + 'px' : '-24px',
                }">
                    {{ Math.round(sel.w) }} × {{ Math.round(sel.h) }}
                </div>
            </div>
        </template>
        <div v-else class="mask" style="left: 0; top: 0; width: 100%; height: 100%" />

        <div v-if="hint && !hasSel" class="hint">
            <div class="hint-title">{{ screen ? screen.name : '' }}</div>
            <div class="hint-sub">拖动鼠标框选区域 · Esc 取消 / Drag to select · Esc to cancel</div>
        </div>
    </div>
</template>

<style scoped>
.overlay-root {
    position: fixed;
    inset: 0;
    overflow: hidden;
    user-select: none;
    cursor: crosshair;
}

.mask {
    position: absolute;
    background: rgba(0, 0, 0, 0.35);
    pointer-events: none;
}

.sel-border {
    position: absolute;
    border: 1px solid rgba(64, 158, 255, 0.95);
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.25);
    pointer-events: none;
}

.sel-size {
    position: absolute;
    padding: 1px 6px;
    font-size: 12px;
    line-height: 18px;
    color: #fff;
    background: rgba(64, 158, 255, 0.95);
    border-radius: 4px;
    white-space: nowrap;
}

.hint {
    position: absolute;
    top: 24px;
    left: 50%;
    transform: translateX(-50%);
    padding: 10px 18px;
    text-align: center;
    color: #fff;
    background: rgba(20, 20, 28, 0.82);
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 10px;
    pointer-events: none;
}

.hint-title {
    font-size: 13px;
    font-weight: 600;
    opacity: 0.9;
}

.hint-sub {
    margin-top: 2px;
    font-size: 12px;
    opacity: 0.75;
}
</style>
