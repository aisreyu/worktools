<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NProgress,
  NSelect,
  NSpace,
  NTag,
  useMessage,
} from 'naive-ui'
import { Clipboard, Dialogs } from '@wailsio/runtime'
import { ExportService, OTPService, type OTPEntry } from '../lib/api'
import { errText } from '../lib/i18n'

const message = useMessage()

type Row = OTPEntry

/** Live code + remaining seconds, keyed by entry id. */
const codes = ref<Record<number, { code: string; remaining: number; period: number }>>({})
const rows = ref<Row[]>([])
const loading = ref(false)
const showForm = ref(false)
const saving = ref(false)
const showImport = ref(false)
const importURI = ref('')
const form = ref<Row>(blank())
const search = ref('')

/** Export dialog state. */
type ExportBatch = { uri: string; count: number; index: number; total: number }
const showExport = ref(false)
const exportBatches = ref<ExportBatch[]>([])
const exportBatchIdx = ref(0)
const exportQR = ref('')
const exporting = ref(false)

const algos = [
  { label: 'SHA1', value: 'SHA1' },
  { label: 'SHA256', value: 'SHA256' },
  { label: 'SHA512', value: 'SHA512' },
]
const types = [
  { label: 'TOTP（基于时间）', value: 'totp' },
  { label: 'HOTP（基于计数器）', value: 'hotp' },
]

function blank(): Row {
  return { id: 0, title: '', issuer: '', account: '', secret: '', algorithm: 'SHA1', digits: 6, period: 30, type: 'totp', counter: 0, createdAt: 0, updatedAt: 0 }
}

async function load() {
  loading.value = true
  try {
    rows.value = ((await OTPService.List()) ?? []) as Row[]
  } catch (e) {
    message.error(errText(e))
  } finally {
    loading.value = false
  }
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter((r) =>
    `${r.title} ${r.issuer} ${r.account}`.toLowerCase().includes(q),
  )
})

function openCreate() {
  form.value = blank()
  showForm.value = true
}

async function openEdit(row: Row) {
  try {
    const full = (await OTPService.Get(row.id)) as Row
    form.value = { ...full }
    showForm.value = true
  } catch (e) {
    message.error(errText(e))
  }
}

async function save() {
  if (!form.value.secret?.trim()) {
    message.warning('Secret 必填')
    return
  }
  saving.value = true
  try {
    await OTPService.Save(form.value)
    showForm.value = false
    await load()
    message.success('已保存')
  } catch (e) {
    message.error(errText(e))
  } finally {
    saving.value = false
  }
}

async function remove(row: Row) {
  try {
    await OTPService.Delete(row.id)
    await load()
    message.success('已删除')
  } catch (e) {
    message.error(errText(e))
  }
}

async function doImport() {
  if (!importURI.value.trim()) {
    message.warning('请粘贴 otpauth:// 或 otpauth-migration:// URI')
    return
  }
  try {
    const n = Number(await OTPService.ImportURI(importURI.value.trim()))
    importURI.value = ''
    showImport.value = false
    await load()
    message.success(n > 1 ? `已批量导入 ${n} 条` : '已导入')
  } catch (e) {
    message.error(errText(e))
  }
}

async function importQRImage() {
  const path = await Dialogs.OpenFile({
    Title: '选择二维码图片（Google Authenticator 导出截图）',
    Filters: [
      { DisplayName: '图片文件', Pattern: '*.png;*.jpg;*.jpeg;*.gif' },
    ],
    CanChooseFiles: true,
    AllowsMultipleSelection: false,
  })
  if (!path || Array.isArray(path)) return
  try {
    const n = Number(await OTPService.ImportQRImage(path))
    await load()
    message.success(n > 1 ? `二维码识别成功，已批量导入 ${n} 条` : '二维码识别成功，已导入')
  } catch (e) {
    message.error(errText(e))
  }
}

/** Open the export dialog: build migration batches and render the first QR. */
async function openExport() {
  if (rows.value.length === 0) {
    message.warning('暂无可导出的条目')
    return
  }
  exporting.value = true
  try {
    const batches = ((await OTPService.ExportMigrationURIs([])) ?? []) as ExportBatch[]
    if (batches.length === 0) {
      message.warning('暂无可导出的条目')
      return
    }
    exportBatches.value = batches
    exportBatchIdx.value = 0
    await renderBatchQR()
    showExport.value = true
  } catch (e) {
    message.error(errText(e))
  } finally {
    exporting.value = false
  }
}

const currentBatch = computed(() => exportBatches.value[exportBatchIdx.value])

async function renderBatchQR() {
  const b = currentBatch.value
  if (!b) return
  try {
    // []byte crosses the bridge as a base64 string (Go json.Marshal).
    const b64 = (await OTPService.ExportQRImage(b.uri)) as string
    exportQR.value = 'data:image/png;base64,' + b64
  } catch (e) {
    message.error(errText(e))
  }
}

async function nextBatch() {
  if (exportBatchIdx.value < exportBatches.value.length - 1) {
    exportBatchIdx.value++
    await renderBatchQR()
  }
}

async function prevBatch() {
  if (exportBatchIdx.value > 0) {
    exportBatchIdx.value--
    await renderBatchQR()
  }
}

async function copyBatchURI() {
  const b = currentBatch.value
  if (!b) return
  await Clipboard.SetText(b.uri)
  message.success('迁移 URI 已复制')
}

async function saveBatchPNG() {
  const b = currentBatch.value
  if (!b) return
  const path = await Dialogs.SaveFile({
    Title: '保存二维码图片',
    Filename: `otp-migration-${b.index}-of-${b.total}.png`,
    Filters: [{ DisplayName: 'PNG 图片', Pattern: '*.png' }],
  })
  if (!path) return
  try {
    const b64 = (await OTPService.ExportQRImage(b.uri)) as string
    await ExportService.SaveBinary(path, b64)
    message.success('已保存二维码图片')
  } catch (e) {
    message.error(errText(e))
  }
}

/** Copy a single entry as a standard otpauth:// URI. */
async function copyEntryURI(row: Row) {
  try {
    const uri = (await OTPService.ExportEntryURI(row.id)) as string
    await Clipboard.SetText(uri)
    message.success('otpauth:// URI 已复制')
  } catch (e) {
    message.error(errText(e))
  }
}

async function copyCode(row: Row) {
  const c = codes.value[row.id]?.code
  if (!c) return
  await Clipboard.SetText(c)
  message.success('验证码已复制')
}

async function nextHotp(row: Row) {
  try {
    const c = await OTPService.NextCode(row.id)
    codes.value = { ...codes.value, [row.id]: { code: c.code, remaining: c.remaining, period: c.period } }
  } catch (e) {
    message.error(errText(e))
  }
}

/** Poll TOTP codes once per second for the visible list. */
let timer: number | undefined
async function tick() {
  const next: Record<number, { code: string; remaining: number; period: number }> = { ...codes.value }
  for (const r of rows.value) {
    if (r.type === 'hotp') continue
    try {
      const c = await OTPService.Code(r.id)
      next[r.id] = { code: c.code, remaining: c.remaining, period: c.period }
    } catch {
      /* vault may have locked; ignore */
    }
  }
  codes.value = next
}

function codeOf(r: Row): string {
  const c = codes.value[r.id]
  if (!c) return '------'
  return c.code.replace(/(\d{3})(?=\d)/g, '$1 ')
}

function pctOf(r: Row): number {
  const c = codes.value[r.id]
  if (!c) return 0
  return Math.round((c.remaining / (c.period || 30)) * 100)
}

function remainingOf(r: Row): number {
  return codes.value[r.id]?.remaining ?? 0
}

onMounted(async () => {
  await load()
  await tick()
  timer = window.setInterval(tick, 1000)
})
onUnmounted(() => {
  if (timer) window.clearInterval(timer)
})
</script>

<template>
  <div class="page">
    <div class="row-between page-title">
      <span>OTP 验证码</span>
      <n-space>
        <n-button size="small" :loading="exporting" @click="openExport">导出迁移</n-button>
        <n-button size="small" @click="importQRImage">识别二维码图片</n-button>
        <n-button size="small" @click="showImport = true">导入 URI</n-button>
        <n-button type="primary" size="small" @click="openCreate">新增</n-button>
      </n-space>
    </div>

    <n-input
      v-model:value="search"
      placeholder="搜索名称 / 发行方 / 账号…"
      clearable
      round
      size="small"
      style="max-width: 320px; margin-bottom: 14px"
    />

    <div v-if="loading" style="opacity: 0.6; padding: 24px 4px">加载中…</div>
    <div v-else-if="filtered.length === 0" style="opacity: 0.6; padding: 24px 4px">
      暂无 OTP 条目，点击右上角「新增」或「导入 URI」。
    </div>
    <div v-else class="otp-grid">
      <div v-for="r in filtered" :key="r.id" class="otp-card">
        <div class="otp-card-head">
          <div class="otp-card-title">
            <span class="otp-card-name">{{ r.title || r.account }}</span>
            <n-tag size="tiny" round :bordered="false" :type="r.type === 'hotp' ? 'warning' : 'info'">
              {{ r.type.toUpperCase() }}
            </n-tag>
          </div>
          <div class="otp-card-account" v-if="r.account && r.title">{{ r.account }}</div>
        </div>
        <div class="otp-card-code" @click="copyCode(r)" title="点击复制">
          {{ codeOf(r) }}
        </div>
        <div class="otp-card-foot">
          <template v-if="r.type === 'totp'">
            <n-progress
              type="line"
              :percentage="pctOf(r)"
              :height="6"
              :show-indicator="false"
              :status="remainingOf(r) <= 5 ? 'warning' : 'success'"
              style="flex: 1"
            />
            <span class="otp-remaining" :class="{ urgent: remainingOf(r) <= 5 }">{{ remainingOf(r) }}s</span>
          </template>
          <template v-else>
            <span style="font-size: 12px; opacity: 0.5; flex: 1">计数器：{{ r.counter }}</span>
          </template>
          <n-space size="small">
            <n-button size="tiny" quaternary @click="copyCode(r)">复制</n-button>
            <n-button v-if="r.type === 'hotp'" size="tiny" quaternary @click="nextHotp(r)">下一个</n-button>
            <n-button size="tiny" quaternary @click="copyEntryURI(r)">URI</n-button>
            <n-button size="tiny" quaternary @click="openEdit(r)">编辑</n-button>
            <n-popconfirm @positive-click="remove(r)">
              <template #trigger>
                <n-button size="tiny" quaternary type="error">删除</n-button>
              </template>
              确认删除该条目？
            </n-popconfirm>
          </n-space>
        </div>
      </div>
    </div>

    <n-modal v-model:show="showForm" preset="card" style="width: 520px" :title="form.id ? '编辑 OTP' : '新增 OTP'">
      <n-form label-placement="left" label-width="84">
        <n-form-item label="名称"><n-input v-model:value="form.title" placeholder="例如 GitHub" /></n-form-item>
        <n-form-item label="发行方"><n-input v-model:value="form.issuer" /></n-form-item>
        <n-form-item label="账号"><n-input v-model:value="form.account" placeholder="user@example.com" /></n-form-item>
        <n-form-item label="Secret"><n-input v-model:value="form.secret" placeholder="Base32 密钥" /></n-form-item>
        <n-form-item label="类型"><n-select v-model:value="form.type" :options="types" /></n-form-item>
        <n-form-item label="算法"><n-select v-model:value="form.algorithm" :options="algos" /></n-form-item>
        <n-form-item label="位数"><n-input-number v-model:value="form.digits" :min="6" :max="8" :step="2" style="width: 100%" /></n-form-item>
        <n-form-item v-if="form.type === 'totp'" label="周期"><n-input-number v-model:value="form.period" :min="10" :max="120" style="width: 100%" /></n-form-item>
        <n-form-item v-else label="计数器"><n-input-number v-model:value="form.counter" :min="0" style="width: 100%" /></n-form-item>
      </n-form>
      <template #footer>
        <div class="row-between">
          <span />
          <n-space>
            <n-button @click="showForm = false">取消</n-button>
            <n-button type="primary" :loading="saving" @click="save">保存</n-button>
          </n-space>
        </div>
      </template>
    </n-modal>

    <n-modal v-model:show="showImport" preset="card" style="width: 520px" title="导入 otpauth:// URI">
      <n-input v-model:value="importURI" type="textarea" :autosize="{ minRows: 3 }" placeholder="otpauth://totp/... 或 otpauth-migration://offline?data=...（Google Authenticator 批量导出，自动导入全部条目）" />
      <template #footer>
        <div class="row-between">
          <span />
          <n-space>
            <n-button @click="showImport = false">取消</n-button>
            <n-button type="primary" @click="doImport">导入</n-button>
          </n-space>
        </div>
      </template>
    </n-modal>

    <n-modal v-model:show="showExport" preset="card" style="width: 420px" title="导出到其他验证器">
      <div v-if="currentBatch" class="export-body">
        <p class="export-tip">
          用 Google Authenticator、Microsoft Authenticator、Aegis、FreeOTP 等应用的
          「扫描二维码 / 导入账户」功能扫描下方二维码。
          每个二维码最多 10 条{{ currentBatch.total > 1 ? `，共 ${currentBatch.total} 个二维码，请逐个扫描` : '' }}。
        </p>
        <div class="export-batch-info">
          第 {{ currentBatch.index }} / {{ currentBatch.total }} 个 · 含 {{ currentBatch.count }} 条
        </div>
        <div class="export-qr-wrap">
          <img v-if="exportQR" :src="exportQR" alt="迁移二维码" class="export-qr" />
        </div>
        <n-space justify="center" size="small">
          <n-button size="small" :disabled="exportBatchIdx === 0" @click="prevBatch">上一个</n-button>
          <n-button size="small" :disabled="exportBatchIdx >= exportBatches.length - 1" @click="nextBatch">下一个</n-button>
        </n-space>
      </div>
      <template #footer>
        <div class="row-between">
          <n-space size="small">
            <n-button size="small" @click="copyBatchURI">复制 URI</n-button>
            <n-button size="small" @click="saveBatchPNG">保存 PNG</n-button>
          </n-space>
          <n-button @click="showExport = false">关闭</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.export-tip {
  font-size: 12px;
  opacity: 0.65;
  line-height: 1.6;
  margin: 0 0 10px;
}
.export-batch-info {
  font-size: 12px;
  text-align: center;
  opacity: 0.8;
  margin-bottom: 10px;
}
.export-qr-wrap {
  display: flex;
  justify-content: center;
  padding: 12px;
  background: #fff;
  border-radius: 8px;
  margin-bottom: 12px;
}
.export-qr {
  width: 260px;
  height: 260px;
  image-rendering: pixelated;
}
.otp-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}
.otp-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
  transition: background 0.15s ease, border-color 0.15s ease;
}
.otp-card:hover {
  background: rgba(255, 255, 255, 0.07);
  border-color: rgba(99, 226, 183, 0.35);
}
.otp-card-head {
  min-width: 0;
}
.otp-card-title {
  display: flex;
  align-items: center;
  gap: 8px;
}
.otp-card-name {
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.otp-card-account {
  font-size: 12px;
  opacity: 0.55;
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.otp-card-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 24px;
  letter-spacing: 2px;
  cursor: pointer;
  user-select: none;
  text-align: center;
  padding: 6px 0;
  border-radius: 8px;
  background: rgba(99, 226, 183, 0.06);
}
.otp-card-code:hover {
  background: rgba(99, 226, 183, 0.12);
}
.otp-card-foot {
  display: flex;
  align-items: center;
  gap: 10px;
}
.otp-remaining {
  font-size: 12px;
  opacity: 0.6;
  min-width: 28px;
  text-align: right;
}
.otp-remaining.urgent {
  color: #f2c97d;
  opacity: 1;
}
</style>
