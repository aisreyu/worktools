<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  NSpin,
  NTable,
  NTag,
  useMessage,
} from 'naive-ui'
import { Browser, Clipboard } from '@wailsio/runtime'
import { OpenURLService, VaultService, WorkbenchService, type URLEntry, type URLHealthResult } from '../lib/api'
import { copyWithAutoClear } from '../lib/clipboardClear'
import { t, errText } from '../lib/i18n'
import { useRouteHighlight } from '../lib/routeHighlight'

useRouteHighlight()

const message = useMessage()

type Row = URLEntry

const rows = ref<Row[]>([])
const loading = ref(false)
const showForm = ref(false)
const saving = ref(false)
const form = ref<Row>(blank())
const search = ref('')
const activeEnv = ref<string>('all')

function blank(): Row {
  return {
    id: 0, title: '', url: '', env: '', tags: '', username: '', password: '', note: '', pinned: false,
    createdAt: 0, updatedAt: 0,
  }
}

async function load() {
  loading.value = true
  try {
    rows.value = ((await VaultService.ListURLs()) ?? []) as Row[]
  } catch (e) {
    message.error(errText(e))
  } finally {
    loading.value = false
  }
}

/** All envs present in the data, for the filter bar. */
const envs = computed(() => {
  const set = new Set<string>()
  for (const r of rows.value) {
    const e = r.env.trim()
    if (e) set.add(e)
  }
  return Array.from(set).sort()
})

/** Group filtered rows by env; empty env goes to "未分组". */
const groups = computed(() => {
  const q = search.value.trim().toLowerCase()
  const map = new Map<string, Row[]>()
  for (const r of rows.value) {
    if (activeEnv.value !== 'all' && r.env.trim() !== activeEnv.value) continue
    if (q) {
      const hay = `${r.title} ${r.url} ${r.tags} ${r.username} ${r.env}`.toLowerCase()
      if (!hay.includes(q)) continue
    }
    const key = r.env.trim() || '未分组'
    if (!map.has(key)) map.set(key, [])
    map.get(key)!.push(r)
  }
  return Array.from(map.entries()).map(([env, items]) => ({
    env,
    items: items.sort((a, b) => Number(b.pinned) - Number(a.pinned) || b.id - a.id),
  }))
})

/** First letter for the card icon fallback. */
function iconText(r: Row): string {
  const src = r.title.trim() || hostOf(r.url) || '?'
  return src.charAt(0).toUpperCase()
}

function hostOf(u: string): string {
  try {
    return new URL(u.includes('://') ? u : `https://${u}`).hostname.replace(/^www\./, '')
  } catch {
    return u
  }
}

/** Favicon URL via Google's public service; falls back to the letter tile. */
function faviconOf(r: Row): string | null {
  const host = hostOf(r.url)
  if (!host) return null
  return `https://www.google.com/s2/favicons?sz=64&domain=${encodeURIComponent(host)}`
}

function tagList(r: Row): string[] {
  return r.tags.split(/[,，]/).map((t) => t.trim()).filter(Boolean)
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
  if (!form.value.title.trim() && !form.value.url.trim()) {
    message.warning('标题或 URL 至少填写一项')
    return
  }
  saving.value = true
  try {
    await VaultService.SaveURL(form.value)
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
    await VaultService.DeleteURL(row.id)
    await load()
    message.success(t('trash.movedHint'))
  } catch (e) {
    message.error(errText(e))
  }
}

async function togglePin(row: Row) {
  try {
    await VaultService.SetURLPinned(row.id, !row.pinned)
    await load()
  } catch (e) {
    message.error(errText(e))
  }
}

async function copyPassword(row: Row) {
  if (!row.password) {
    message.warning('该条目没有保存密码')
    return
  }
  await copyWithAutoClear(row.password)
  message.success('密码已复制（N 秒后自动清理剪贴板）')
}

async function copyUsername(row: Row) {
  if (!row.username) {
    message.warning('该条目没有保存用户名')
    return
  }
  await Clipboard.SetText(row.username)
  message.success('用户名已复制')
}

/** Copy every field of the entry as a readable block. */
async function copyAll(row: Row) {
  const lines: string[] = []
  if (row.title) lines.push(`标题: ${row.title}`)
  if (row.url) lines.push(`URL: ${row.url}`)
  if (row.env) lines.push(`环境: ${row.env}`)
  if (row.username) lines.push(`用户名: ${row.username}`)
  if (row.password) lines.push(`密码: ${row.password}`)
  if (row.tags) lines.push(`标签: ${row.tags}`)
  if (row.note) lines.push(`备注: ${row.note}`)
  if (lines.length === 0) {
    message.warning('该条目没有可复制的内容')
    return
  }
  const text = lines.join('\n')
  await copyWithAutoClear(text)
  message.success('全部信息已复制（N 秒后自动清理剪贴板）')
}

/** Open in an embedded window with credential auto-fill. */
async function openWithFill(row: Row) {
  try {
    await OpenURLService.OpenWithLogin(row.id)
  } catch (e) {
    message.error(errText(e))
  }
}

/** Open in the system browser. */
function openExternal(row: Row) {
  if (row.url) Browser.OpenURL(row.url)
}

/** Open every URL entry in the active env (or all when "all") at once. */
const openingEnv = ref(false)
async function openActiveEnv() {
  if (rows.value.length === 0) return
  const label = activeEnv.value === 'all' ? '全部' : activeEnv.value
  openingEnv.value = true
  try {
    const n = await OpenURLService.OpenEnvBatch(activeEnv.value === 'all' ? '' : activeEnv.value)
    if (activeEnv.value === 'all') {
      // OpenEnvBatch('') only matches empty-env entries; for "all" iterate every env.
      let total = n
      for (const e of envs.value) total += await OpenURLService.OpenEnvBatch(e)
      message.success(`已打开 ${total} 个窗口（${label}）`)
    } else {
      message.success(`已打开 ${n} 个窗口（${label}）`)
    }
  } catch (e) {
    message.error(errText(e))
  } finally {
    openingEnv.value = false
  }
}

const envOptions = computed(() => [
  { label: '全部', value: 'all' },
  ...envs.value.map((e) => ({ label: e, value: e })),
])

// ---- URL batch health monitor ----
const showHealth = ref(false)
const healthRunning = ref(false)
const healthResults = ref<URLHealthResult[]>([])
const healthById = computed(() => {
  const m = new Map<number, URLHealthResult>()
  for (const r of healthResults.value) m.set(r.id, r)
  return m
})

async function runHealthCheck() {
  if (rows.value.length === 0) {
    message.warning('暂无 URL 可检测')
    return
  }
  showHealth.value = true
  healthRunning.value = true
  healthResults.value = []
  try {
    healthResults.value = ((await WorkbenchService.URLHealthCheck(8000)) ?? []) as URLHealthResult[]
  } catch (e) {
    message.error(errText(e))
  } finally {
    healthRunning.value = false
  }
}

function healthSummary() {
  const ok = healthResults.value.filter((r) => r.ok).length
  return t('urlhealth.summary', { ok, fail: healthResults.value.length - ok })
}

function certText(r: URLHealthResult): string {
  if (r.certExpired) return t('urlhealth.certExpired')
  if (r.certExpiring) return t('urlhealth.certExpiring')
  if (typeof r.certDaysLeft === 'number') return t('urlhealth.certDays', { n: r.certDaysLeft })
  return t('urlhealth.certNone')
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="row-between page-title">
      <span>常用 URL</span>
      <n-space>
        <n-button size="small" :loading="healthRunning" @click="runHealthCheck">
          🩺 {{ t('urlhealth.run') }}
        </n-button>
        <n-button size="small" :loading="openingEnv" :disabled="rows.length === 0" @click="openActiveEnv">
          🚀 {{ t('urls.openEnv') }}
        </n-button>
        <n-button type="primary" size="small" @click="openCreate">新增</n-button>
      </n-space>
    </div>

    <!-- Search + env filter bar -->
    <div class="nav-toolbar">
      <n-input
        v-model:value="search"
        placeholder="搜索标题 / URL / 标签 / 用户名…"
        clearable
        round
        size="small"
        style="max-width: 320px"
      />
      <n-space size="small" align="center">
        <n-tag
          v-for="opt in envOptions"
          :key="opt.value"
          size="small"
          round
          :type="activeEnv === opt.value ? 'primary' : 'default'"
          :bordered="activeEnv !== opt.value"
          class="env-tag"
          @click="activeEnv = opt.value"
        >
          {{ opt.label }}
        </n-tag>
      </n-space>
    </div>

    <!-- Card grid grouped by env -->
    <div v-if="loading" style="opacity: 0.6; padding: 24px 4px">加载中…</div>
    <div v-else-if="groups.length === 0" style="opacity: 0.6; padding: 24px 4px">
      暂无条目，点击右上角「新增」添加常用 URL。
    </div>
    <section v-for="g in groups" :key="g.env" class="nav-group">
      <h3 class="nav-group-title">
        <span class="nav-group-dot" />{{ g.env }}
        <span class="nav-group-count">{{ g.items.length }}</span>
      </h3>
      <div class="nav-grid">
        <div v-for="r in g.items" :key="r.id" class="nav-card" :data-hid="r.id">
          <div class="nav-card-main" @click="openWithFill(r)">
            <div class="nav-card-icon">
              <img
                v-if="faviconOf(r)"
                :src="faviconOf(r)!"
                alt=""
                loading="lazy"
                @error="($event.target as HTMLImageElement).parentElement && (($event.target as HTMLImageElement).style.display = 'none')"
              />
              <span v-if="!faviconOf(r)" class="nav-card-letter">{{ iconText(r) }}</span>
            </div>
            <div class="nav-card-info">
              <div class="nav-card-title" :title="r.title || r.url">
                <span
                  v-if="healthById.get(r.id)"
                  class="nav-health-dot"
                  :class="healthById.get(r.id)!.ok ? 'ok' : 'bad'"
                  :title="healthById.get(r.id)!.ok ? t('urlhealth.ok') : t('urlhealth.fail')"
                />
                {{ r.title || r.url }}
              </div>
              <div class="nav-card-sub" :title="r.username">{{ r.username || hostOf(r.url) }}</div>
            </div>
          </div>
          <div class="nav-card-tags" v-if="tagList(r).length">
            <n-tag v-for="t in tagList(r).slice(0, 3)" :key="t" size="tiny" round :bordered="false">
              {{ t }}
            </n-tag>
          </div>
          <div class="nav-card-actions">
            <n-button size="tiny" quaternary @click.stop="openWithFill(r)">打开并填充</n-button>
            <n-button size="tiny" quaternary @click.stop="copyUsername(r)">复制账号</n-button>
            <n-button size="tiny" quaternary @click.stop="copyPassword(r)">复制密码</n-button>
            <n-button size="tiny" quaternary @click.stop="copyAll(r)">复制全部</n-button>
            <n-button size="tiny" quaternary @click.stop="openExternal(r)">浏览器</n-button>
            <n-button size="tiny" quaternary :type="r.pinned ? 'warning' : 'default'" @click.stop="togglePin(r)">
              {{ r.pinned ? '取消置顶' : '置顶' }}
            </n-button>
            <n-button size="tiny" quaternary @click.stop="openEdit(r)">编辑</n-button>
            <n-popconfirm @positive-click="remove(r)">
              <template #trigger>
                <n-button size="tiny" quaternary type="error" @click.stop>删除</n-button>
              </template>
              确认删除该条目？
            </n-popconfirm>
          </div>
        </div>
      </div>
    </section>

    <n-modal v-model:show="showForm" preset="card" style="width: 520px" :title="form.id ? '编辑条目' : '新增条目'">
      <n-form label-placement="left" label-width="72">
        <n-form-item label="标题"><n-input v-model:value="form.title" placeholder="例如 生产环境控制台" /></n-form-item>
        <n-form-item label="URL"><n-input v-model:value="form.url" placeholder="https://..." /></n-form-item>
        <n-form-item label="环境"><n-input v-model:value="form.env" placeholder="prod / uat / dev" /></n-form-item>
        <n-form-item label="标签"><n-input v-model:value="form.tags" placeholder="逗号分隔" /></n-form-item>
        <n-form-item label="用户名"><n-input v-model:value="form.username" /></n-form-item>
        <n-form-item label="密码">
          <n-input v-model:value="form.password" type="password" show-password-on="click" />
        </n-form-item>
        <n-form-item label="备注"><n-input v-model:value="form.note" type="textarea" :autosize="{ minRows: 2 }" /></n-form-item>
      </n-form>
      <template #footer>
        <div class="row-between">
          <span style="font-size: 12px; opacity: 0.55">点击卡片可在内嵌窗口打开并自动填充账号密码</span>
          <n-space>
            <n-button @click="showForm = false">取消</n-button>
            <n-button type="primary" :loading="saving" @click="save">保存</n-button>
          </n-space>
        </div>
      </template>
    </n-modal>
    <n-modal v-model:show="showHealth" preset="card" style="width: 720px" :title="t('urlhealth.title')">
      <div class="health-summary">
        <n-spin v-if="healthRunning" size="small" />
        <span v-else>{{ healthSummary() }}</span>
      </div>
      <n-empty v-if="!healthRunning && healthResults.length === 0" :description="t('urlhealth.certNone')" style="padding: 24px 0" />
      <n-table v-else size="small" :bordered="false" :single-line="false">
        <thead>
          <tr>
            <th>{{ t('import.colTitle') }}</th>
            <th>{{ t('urlhealth.status') }}</th>
            <th>{{ t('urlhealth.latency') }}</th>
            <th>{{ t('urlhealth.cert') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in healthResults" :key="r.id">
            <td class="health-name" :title="r.url">{{ r.name || r.url }}</td>
            <td>
              <n-tag size="tiny" :type="r.ok ? 'success' : 'error'" round>
                {{ r.ok ? t('urlhealth.ok') : t('urlhealth.fail') }}
                <template v-if="r.status"> {{ r.status }}</template>
              </n-tag>
            </td>
            <td>{{ r.error ? r.error : r.latency }}</td>
            <td>
              <n-tag
                v-if="r.certExpired || r.certExpiring"
                size="tiny"
                :type="r.certExpired ? 'error' : 'warning'"
                round
              >
                {{ certText(r) }}
              </n-tag>
              <span v-else>{{ certText(r) }}</span>
            </td>
          </tr>
        </tbody>
      </n-table>
    </n-modal>
  </div>
</template>

<style scoped>
.nav-toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.env-tag {
  cursor: pointer;
  user-select: none;
}
.nav-group {
  margin-bottom: 20px;
}
.nav-group-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  font-size: 14px;
  font-weight: 600;
  color: var(--nb-fg);
  opacity: 0.9;
}
.nav-group-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #63e2b7;
}
.nav-group-count {
  font-size: 12px;
  font-weight: 400;
  opacity: 0.5;
}
.nav-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}
.nav-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
  transition: background 0.15s ease, border-color 0.15s ease, transform 0.15s ease;
}
.nav-card:hover {
  background: rgba(255, 255, 255, 0.07);
  border-color: rgba(99, 226, 183, 0.35);
  transform: translateY(-1px);
}
.nav-card-main {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  min-width: 0;
}
.nav-card-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: rgba(99, 226, 183, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
}
.nav-card-icon img {
  width: 20px;
  height: 20px;
}
.nav-card-letter {
  font-size: 16px;
  font-weight: 700;
  color: #63e2b7;
}
.nav-card-info {
  min-width: 0;
  flex: 1;
}
.nav-card-title {
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: flex;
  align-items: center;
  gap: 6px;
}
.nav-health-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.nav-health-dot.ok {
  background: #63e2b7;
}
.nav-health-dot.bad {
  background: #e88080;
}
.health-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  margin-bottom: 10px;
}
.health-name {
  max-width: 260px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.nav-card-sub {
  font-size: 12px;
  opacity: 0.55;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.nav-card-tags {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.nav-card-actions {
  display: flex;
  gap: 2px;
  flex-wrap: wrap;
  opacity: 0;
  transition: opacity 0.15s ease;
}
.nav-card:hover .nav-card-actions {
  opacity: 1;
}
</style>
