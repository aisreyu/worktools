<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NDivider,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSpace,
  NSwitch,
  NTable,
  NTag,
  useMessage,
} from 'naive-ui'
import { Dialogs } from '@wailsio/runtime'
import { requestSavePath, publishSaved, savedLocationSuffix } from '../lib/saveFile'
import {
  AppService,
  BackupService,
  ExportService,
  VaultService,
  WorkbenchService,
  type AppInfo,
  type RecoveryQuestion,
  type PasswordAuditReport,
  type PasswordAuditFinding,
  type AutoBackupStatus,
  type BackupFile,
  type ImportedCredential,
  type ImportedCredentials,
} from '../lib/api'
import { refreshStatus } from '../lib/vaultState'
import { locale, setLocale, t, errText, detailText } from '../lib/i18n'
import { themeMode, accentColor, accentPresets, setThemeMode, setAccentColor, type ThemeMode } from '../lib/theme'
import { autoLockMinutes, setAutoLockMinutes } from '../lib/autoLock'
import { clipClearSeconds, setClipClearSeconds } from '../lib/clipboardClear'

const message = useMessage()

// ---- Auto-lock ----
const autoLockOptions = computed(() => [
  { label: t('settings.autoLockOff'), value: 0 },
  { label: t('settings.autoLock5'), value: 5 },
  { label: t('settings.autoLock15'), value: 15 },
  { label: t('settings.autoLock30'), value: 30 },
  { label: t('settings.autoLock60'), value: 60 },
])

function onAutoLockChange(v: number) {
  setAutoLockMinutes(v)
}

// ---- Clipboard auto-clear ----
const clipClearOptions = computed(() => [
  { label: t('settings.clipClearOff'), value: 0 },
  { label: t('settings.clipClear10'), value: 10 },
  { label: t('settings.clipClear30'), value: 30 },
  { label: t('settings.clipClear60'), value: 60 },
  { label: t('settings.clipClear120'), value: 120 },
])

function onClipClearChange(v: number) {
  setClipClearSeconds(v)
}

// ---- Language ----
const langOptions = [
  { label: '简体中文', value: 'zh' },
  { label: 'English', value: 'en' },
]

function onLangChange(v: string) {
  setLocale(v as 'zh' | 'en')
}

// ---- Theme ----
const themeOptions = computed(() => [
  { label: t('settings.themeSystem'), value: 'system' },
  { label: t('settings.themeLight'), value: 'light' },
  { label: t('settings.themeDark'), value: 'dark' },
])

function onThemeChange(v: string) {
  setThemeMode(v as ThemeMode)
}

function onAccentInput(e: Event) {
  const v = (e.target as HTMLInputElement).value
  if (/^#[0-9a-fA-F]{6}$/.test(v)) setAccentColor(v)
}

// ---- About ----
const info = ref<AppInfo | null>(null)

// ---- Change master password ----
const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')
const changingPwd = ref(false)

async function changePassword() {
  if (!oldPwd.value || !newPwd.value) {
    message.warning(t('settings.pwdEmpty'))
    return
  }
  if (newPwd.value.length < 8) {
    message.warning(locale.value === 'en' ? 'New password must be at least 8 characters' : '新密码至少 8 个字符')
    return
  }
  if (newPwd.value !== confirmPwd.value) {
    message.warning(t('settings.pwdMismatch'))
    return
  }
  changingPwd.value = true
  try {
    await VaultService.ChangePassword(oldPwd.value, newPwd.value)
    oldPwd.value = ''
    newPwd.value = ''
    confirmPwd.value = ''
    message.success(t('settings.pwdChanged'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    changingPwd.value = false
  }
}

// ---- Recovery questions ----
const hasRecovery = ref(false)
const questions = ref<RecoveryQuestion[]>([{ question: '', answer: '' }, { question: '', answer: '' }])
const savingRecovery = ref(false)

const recoveryValid = computed(() =>
  questions.value.some((q) => q.question.trim() && (q.answer ?? '').trim()),
)

function addQuestion() {
  questions.value.push({ question: '', answer: '' })
}

function removeQuestion(i: number) {
  questions.value.splice(i, 1)
}

async function loadRecovery() {
  try {
    hasRecovery.value = await VaultService.HasRecovery()
    if (hasRecovery.value) {
      const qs = (await VaultService.RecoveryQuestions()) as string[]
      questions.value = qs.map((q) => ({ question: q, answer: '' }))
    }
  } catch {
    /* not initialised yet */
  }
}

async function saveRecovery() {
  const valid = questions.value.filter((q) => q.question.trim() && (q.answer ?? '').trim())
  if (valid.length === 0) {
    message.warning(locale.value === 'en' ? 'Set at least one complete question and answer' : '至少设置一个完整的问题和答案')
    return
  }
  savingRecovery.value = true
  try {
    await VaultService.SetRecoveryQuestions(valid)
    hasRecovery.value = true
    for (const q of questions.value) q.answer = ''
    message.success(t('settings.recoverySaved'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    savingRecovery.value = false
  }
}

// ---- Backup ----
const backupPwd = ref('')
const exporting = ref(false)
const importing = ref(false)

async function exportBackup() {
  if (!backupPwd.value) {
    message.warning(locale.value === 'en' ? 'Set the backup file password first' : '请先设置备份文件密码')
    return
  }
  const filename = `worktools-backup-${new Date().toISOString().slice(0, 10)}.json`
  const path = await requestSavePath(t('settings.backupNow'), filename, [
    { DisplayName: 'JSON', Pattern: '*.json' },
  ])
  if (!path) return
  exporting.value = true
  try {
    await BackupService.Export(path, backupPwd.value)
    const where = await publishSaved(path, filename)
    message.success(`${t('settings.backupDone')}: ${where || path}`)
  } catch (e) {
    message.error(errText(e))
  } finally {
    exporting.value = false
  }
}

async function importBackup() {
  if (!backupPwd.value) {
    message.warning(locale.value === 'en' ? 'Enter the backup file password first' : '请先输入备份文件的密码')
    return
  }
  const path = await Dialogs.OpenFile({
    Title: t('settings.restore'),
    Filters: [{ DisplayName: 'JSON', Pattern: '*.json' }],
    CanChooseFiles: true,
    AllowsMultipleSelection: false,
  })
  if (!path || Array.isArray(path)) return
  importing.value = true
  try {
    const n = await BackupService.Import(path, backupPwd.value)
    message.success(
      locale.value === 'en'
        ? `Restored ${n} records (merge mode, existing data kept)`
        : `已还原 ${n} 条记录（合并模式，不覆盖现有数据）`,
    )
  } catch (e) {
    message.error(errText(e))
  } finally {
    importing.value = false
  }
}

// ---- Auto backup ----
const autoBackup = ref<AutoBackupStatus | null>(null)
const autoDir = ref('')
const autoInterval = ref(24)
const autoKeep = ref(7)
const autoEnabled = ref(false)
const autoPwd = ref('')
const autoSaving = ref(false)
const autoRunning = ref(false)

const intervalOptions = computed(() => [
  { label: locale.value === 'en' ? 'Every 6 hours' : '每 6 小时', value: 6 },
  { label: locale.value === 'en' ? 'Every 12 hours' : '每 12 小时', value: 12 },
  { label: locale.value === 'en' ? 'Daily' : '每天', value: 24 },
  { label: locale.value === 'en' ? 'Weekly' : '每周', value: 168 },
])

function applyAutoStatus(s: AutoBackupStatus) {
  autoBackup.value = s
  autoDir.value = s.dir
  autoInterval.value = s.intervalHours
  autoKeep.value = s.keep
  autoEnabled.value = s.enabled
}

async function loadAutoBackup() {
  try {
    applyAutoStatus((await BackupService.AutoBackupStatus()) as AutoBackupStatus)
  } catch (e) {
    // Silent: vault may be locked during initial mount.
  }
}

async function pickBackupDir() {
  const dir = await Dialogs.OpenFile({
    Title: t('autoBackup.pickDir'),
    CanChooseDirectories: true,
    CanChooseFiles: false,
    CanCreateDirectories: true,
  })
  if (!dir || Array.isArray(dir)) return
  autoDir.value = dir
}

async function saveAutoBackupSettings() {
  autoSaving.value = true
  try {
    await BackupService.SaveAutoBackupSettings(autoEnabled.value, autoDir.value, autoInterval.value, autoKeep.value)
    if (autoPwd.value) {
      await BackupService.SetAutoBackupPassword(autoPwd.value)
      autoPwd.value = ''
    }
    await loadAutoBackup()
    message.success(t('autoBackup.saved'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    autoSaving.value = false
  }
}

async function runAutoBackupNow() {
  autoRunning.value = true
  try {
    const path = await BackupService.RunAutoBackupNow()
    message.success(`${t('autoBackup.ranNow')}: ${path}`)
    await loadAutoBackup()
    await loadBackupFiles()
  } catch (e) {
    message.error(errText(e))
  } finally {
    autoRunning.value = false
  }
}

// ---- Backup restore history ----
const backupFiles = ref<BackupFile[]>([])
const backupFilesLoading = ref(false)
const restorePwd = ref('')
const restoringPath = ref('')
const showRestoreModal = ref(false)
const restoreTarget = ref<BackupFile | null>(null)

async function loadBackupFiles() {
  backupFilesLoading.value = true
  try {
    backupFiles.value = ((await BackupService.ListBackups()) ?? []) as BackupFile[]
  } catch {
    // ignore: may be locked or no dir configured
  } finally {
    backupFilesLoading.value = false
  }
}

function fmtSize(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

function openRestore(f: BackupFile) {
  restoreTarget.value = f
  restorePwd.value = ''
  showRestoreModal.value = true
}

async function confirmRestore() {
  const f = restoreTarget.value
  if (!f) return
  if (!restorePwd.value) {
    message.warning(t('autoBackup.passwordNeeded'))
    return
  }
  restoringPath.value = f.path
  try {
    const n = await BackupService.RestoreBackupFile(f.path, restorePwd.value)
    message.success(t('autoBackup.restoreDone', { n }))
    showRestoreModal.value = false
    await refreshStatus()
  } catch (e) {
    message.error(errText(e))
  } finally {
    restoringPath.value = ''
  }
}

function fmtTs(ts: number): string {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

// ---- Password health audit ----
const auditReport = ref<PasswordAuditReport | null>(null)
const auditing = ref(false)
type AuditFilter = 'all' | 'weak' | 'reused' | 'old'
const auditFilter = ref<AuditFilter>('all')
const filteredFindings = computed<PasswordAuditFinding[]>(() => {
  const list = auditReport.value?.findings ?? []
  if (auditFilter.value === 'all') return list
  return list.filter((f) => f.issue === auditFilter.value)
})

function setAuditFilter(f: AuditFilter) {
  auditFilter.value = auditFilter.value === f ? 'all' : f
}

async function runAudit() {
  auditing.value = true
  auditFilter.value = 'all'
  try {
    auditReport.value = (await WorkbenchService.PasswordAudit()) as PasswordAuditReport
  } catch (e) {
    message.error(errText(e))
  } finally {
    auditing.value = false
  }
}

function sevType(s: string): 'error' | 'warning' | 'default' {
  if (s === 'high') return 'error'
  if (s === 'medium') return 'warning'
  return 'default'
}

function sevLabel(s: string): string {
  if (s === 'high') return t('audit.sevHigh')
  if (s === 'medium') return t('audit.sevMedium')
  return t('audit.sevLow')
}

function issueLabel(issue: string): string {
  if (issue === 'weak') return t('audit.issueWeak')
  if (issue === 'reused') return t('audit.issueReused')
  return t('audit.issueOld')
}

function findingSource(f: PasswordAuditFinding): string {
  return f.source === 'secret' ? t('menu.secrets') : t('menu.urls')
}

// ---- External import ----
const importSource = ref('auto')
const importTarget = ref('urls')
const importingCreds = ref(false)
const parsedCreds = ref<ImportedCredentials | null>(null)

const sourceOptions = computed(() => [
  { label: t('import.sourceAuto'), value: 'auto' },
  { label: 'KeePass', value: 'keepass' },
  { label: 'Chrome', value: 'chrome' },
  { label: '1Password', value: '1password' },
])

const targetOptions = computed(() => [
  { label: t('import.targetUrls'), value: 'urls' },
  { label: t('import.targetSecrets'), value: 'secrets' },
])

async function pickImportFile() {
  const path = await Dialogs.OpenFile({
    Title: t('import.pickFile'),
    Filters: [{ DisplayName: 'CSV', Pattern: '*.csv' }],
    CanChooseFiles: true,
    AllowsMultipleSelection: false,
  })
  if (!path || Array.isArray(path)) return
  try {
    const text = await ExportService.ReadText(path)
    const src = importSource.value === 'auto' ? 'auto' : importSource.value
    parsedCreds.value = (await WorkbenchService.ParseImport(text, src)) as ImportedCredentials
    if ((parsedCreds.value.items ?? []).length === 0) {
      message.warning(t('import.empty'))
    }
  } catch (e) {
    message.error(errText(e))
  }
}

async function confirmImport() {
  const creds = parsedCreds.value
  if (!creds || (creds.items ?? []).length === 0) return
  importingCreds.value = true
  try {
    const n = await WorkbenchService.ImportCredentials(
      (creds.items ?? []) as ImportedCredential[],
      importTarget.value,
    )
    message.success(t('import.done', { n }))
    parsedCreds.value = null
  } catch (e) {
    message.error(errText(e))
  } finally {
    importingCreds.value = false
  }
}

onMounted(async () => {
  await refreshStatus()
  await loadRecovery()
  await loadAutoBackup()
  await loadBackupFiles()
  try {
    info.value = await AppService.Info()
  } catch {
    /* ignore */
  }
})
</script>

<template>
  <div class="page">
    <div class="page-title">{{ t('settings.title') }}</div>

    <n-space vertical size="large">
      <!-- Language -->
      <n-card :title="t('settings.language')" size="small">
        <div class="lang-row">
          <n-radio-group :value="locale" @update:value="onLangChange">
            <n-radio-button v-for="o in langOptions" :key="o.value" :value="o.value" :label="o.label" />
          </n-radio-group>
          <span class="hint">{{ t('settings.languageDesc') }}</span>
        </div>
      </n-card>

      <!-- Theme -->
      <n-card :title="t('settings.theme')" size="small">
        <div class="lang-row" style="margin-bottom: 12px">
          <n-radio-group :value="themeMode" @update:value="onThemeChange">
            <n-radio-button v-for="o in themeOptions" :key="o.value" :value="o.value" :label="o.label" />
          </n-radio-group>
          <span class="hint">{{ t('settings.themeDesc') }}</span>
        </div>
        <div class="lang-row">
          <span class="hint">{{ t('settings.accent') }}</span>
          <div class="accent-swatches">
            <span
              v-for="c in accentPresets"
              :key="c"
              class="accent-swatch"
              :class="{ active: accentColor.toLowerCase() === c }"
              :style="{ background: c }"
              :title="c"
              @click="setAccentColor(c)"
            />
            <input class="accent-picker" type="color" :value="accentColor" @input="onAccentInput" />
          </div>
        </div>
      </n-card>

      <!-- Auto-lock -->
      <n-card :title="t('settings.autoLock')" size="small">
        <div class="lang-row">
          <n-select
            :value="autoLockMinutes"
            :options="autoLockOptions"
            style="width: 200px"
            @update:value="onAutoLockChange"
          />
          <span class="hint">{{ t('settings.autoLockDesc') }}</span>
        </div>
      </n-card>

      <!-- Clipboard auto-clear -->
      <n-card :title="t('settings.clipClear')" size="small">
        <div class="lang-row">
          <n-select
            :value="clipClearSeconds"
            :options="clipClearOptions"
            style="width: 200px"
            @update:value="onClipClearChange"
          />
          <span class="hint">{{ t('settings.clipClearDesc') }}</span>
        </div>
      </n-card>

      <!-- About -->
      <n-card :title="t('settings.about')" size="small">
        <div v-if="info" class="about-grid">
          <div class="about-item"><span class="k">Name</span><span class="v">{{ info.name }}</span></div>
          <div class="about-item"><span class="k">{{ t('settings.version') }}</span><span class="v">{{ info.version }}</span></div>
          <div class="about-item"><span class="k">{{ t('settings.platform') }}</span><span class="v">{{ info.platform }}</span></div>
          <div class="about-item"><span class="k">{{ t('settings.website') }}</span><span class="v">{{ info.website }}</span></div>
          <div class="about-item about-desc"><span class="k">Description</span><span class="v">{{ info.description }}</span></div>
        </div>
        <n-spin v-else size="small" />
      </n-card>

      <!-- Change master password -->
      <n-card :title="t('settings.changePwd')" size="small">
        <n-form label-placement="left" label-width="90" inline>
          <n-form-item :label="t('settings.currentPwd')">
            <n-input v-model:value="oldPwd" type="password" show-password-on="click" style="width: 200px" />
          </n-form-item>
          <n-form-item :label="t('settings.newPwd')">
            <n-input v-model:value="newPwd" type="password" show-password-on="click" style="width: 200px" />
          </n-form-item>
          <n-form-item :label="t('settings.confirmPwd')">
            <n-input
              v-model:value="confirmPwd"
              type="password"
              show-password-on="click"
              style="width: 200px"
              @keyup.enter="changePassword"
            />
          </n-form-item>
          <n-form-item label=" ">
            <n-button type="primary" :loading="changingPwd" @click="changePassword">{{ t('settings.pwdSubmit') }}</n-button>
          </n-form-item>
        </n-form>
      </n-card>

      <!-- Recovery questions -->
      <n-card size="small">
        <template #header>
          {{ t('settings.recovery') }}
          <n-tag v-if="hasRecovery" size="small" type="success" style="margin-left: 8px">
            {{ locale === 'en' ? 'Set' : '已设置' }}
          </n-tag>
          <n-tag v-else size="small" type="warning" style="margin-left: 8px">
            {{ locale === 'en' ? 'Not set' : '未设置' }}
          </n-tag>
        </template>
        <template #header-extra>
          <n-button size="small" quaternary @click="addQuestion">
            {{ locale === 'en' ? 'Add question' : '添加问题' }}
          </n-button>
        </template>
        <div v-for="(q, i) in questions" :key="i" class="recovery-row">
          <n-input
            v-model:value="q.question"
            :placeholder="locale === 'en' ? 'Question, e.g. name of your first pet?' : '问题，例如：你的第一只宠物叫什么名字？'"
            style="flex: 1"
          />
          <n-input
            v-model:value="q.answer"
            type="password"
            show-password-on="click"
            :placeholder="locale === 'en' ? 'Answer (encrypted)' : '答案（加密保存）'"
            style="width: 220px"
          />
          <n-button
            v-if="questions.length > 1"
            size="small"
            quaternary
            type="error"
            @click="removeQuestion(i)"
          >
            {{ locale === 'en' ? 'Remove' : '移除' }}
          </n-button>
        </div>
        <div style="margin-top: 12px; display: flex; align-items: center; gap: 12px">
          <n-button type="primary" :loading="savingRecovery" :disabled="!recoveryValid" @click="saveRecovery">
            {{ locale === 'en' ? 'Save recovery questions' : '保存密保问题' }}
          </n-button>
          <span style="font-size: 12px; opacity: 0.55">{{ t('settings.recoveryDesc') }}</span>
        </div>
      </n-card>

      <!-- Password health audit -->
      <n-card size="small">
        <template #header>
          {{ t('audit.title') }}
        </template>
        <template #header-extra>
          <n-button size="small" type="primary" :loading="auditing" @click="runAudit">
            {{ t('audit.run') }}
          </n-button>
        </template>
        <div v-if="!auditReport" class="hint">{{ t('audit.clean') }}</div>
        <template v-else>
          <div class="audit-stats">
            <n-tag size="small" round :bordered="false">{{ t('audit.total', { n: auditReport.total }) }}</n-tag>
            <n-tag
              size="small"
              round
              type="error"
              style="cursor: pointer"
              :class="{ 'audit-tag-active': auditFilter === 'weak' }"
              @click="setAuditFilter('weak')"
            >{{ t('audit.weak') }} {{ auditReport.weak }}</n-tag>
            <n-tag
              size="small"
              round
              type="warning"
              style="cursor: pointer"
              :class="{ 'audit-tag-active': auditFilter === 'reused' }"
              @click="setAuditFilter('reused')"
            >{{ t('audit.reused') }} {{ auditReport.reused }}</n-tag>
            <n-tag
              size="small"
              round
              type="default"
              style="cursor: pointer"
              :class="{ 'audit-tag-active': auditFilter === 'old' }"
              @click="setAuditFilter('old')"
            >{{ t('audit.old') }} {{ auditReport.old }}</n-tag>
          </div>
          <div v-if="auditFilter !== 'all'" class="hint" style="margin-top: 6px">
            {{ t('audit.filtering') }}
            <a style="cursor: pointer; text-decoration: underline" @click="auditFilter = 'all'">{{ t('audit.clearFilter') }}</a>
          </div>
          <n-empty v-if="filteredFindings.length === 0" :description="t('audit.clean')" style="padding: 20px 0" />
          <n-table v-else size="small" :bordered="false" :single-line="false" style="margin-top: 12px">
            <thead>
              <tr>
                <th>{{ t('import.colTitle') }}</th>
                <th>{{ t('audit.source') }}</th>
                <th>{{ t('audit.issue') }}</th>
                <th>{{ t('audit.severity') }}</th>
                <th>{{ t('audit.detail') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(f, i) in filteredFindings" :key="i">
                <td>{{ f.title || f.username || '—' }}</td>
                <td>{{ findingSource(f) }}</td>
                <td>{{ issueLabel(f.issue) }}</td>
                <td>
                  <n-tag size="tiny" :type="sevType(f.severity)" round>{{ sevLabel(f.severity) }}</n-tag>
                </td>
                <td class="audit-detail">{{ detailText(f.detail) }}</td>
              </tr>
            </tbody>
          </n-table>
        </template>
      </n-card>

      <!-- External data import -->
      <n-card :title="t('import.title')" size="small">
        <div class="import-bar">
          <span class="hint">{{ t('import.source') }}</span>
          <n-select v-model:value="importSource" :options="sourceOptions" size="small" style="width: 140px" />
          <n-button size="small" @click="pickImportFile">{{ t('import.pickFile') }}</n-button>
        </div>
        <div class="hint" style="margin-top: 8px">{{ t('import.hint') }}</div>

        <template v-if="parsedCreds">
          <div class="import-preview-head">
            <span>{{ t('import.preview') }} · {{ t('import.count', { n: (parsedCreds.items ?? []).length }) }}</span>
            <n-space align="center" size="small">
              <span class="hint">{{ t('import.target') }}</span>
              <n-select v-model:value="importTarget" :options="targetOptions" size="small" style="width: 160px" />
              <n-button type="primary" size="small" :loading="importingCreds" @click="confirmImport">
                {{ t('import.confirm') }}
              </n-button>
            </n-space>
          </div>
          <n-table size="small" :bordered="false" :single-line="false" style="margin-top: 10px">
            <thead>
              <tr>
                <th>{{ t('import.colTitle') }}</th>
                <th>{{ t('import.colUrl') }}</th>
                <th>{{ t('import.colUser') }}</th>
                <th>{{ t('import.colNote') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(c, i) in (parsedCreds.items ?? []).slice(0, 50)" :key="i">
                <td>{{ c.title }}</td>
                <td class="audit-detail">{{ c.url }}</td>
                <td>{{ c.username }}</td>
                <td class="audit-detail">{{ c.note }}</td>
              </tr>
            </tbody>
          </n-table>
          <div class="hint" style="margin-top: 8px">{{ t('import.overwriteHint') }}</div>
        </template>
      </n-card>

      <!-- Backup -->
      <n-card :title="t('settings.backup')" size="small">
        <n-form label-placement="left" label-width="90">
          <n-form-item :label="locale === 'en' ? 'Backup password' : '备份密码'">
            <n-input
              v-model:value="backupPwd"
              type="password"
              show-password-on="click"
              :placeholder="locale === 'en' ? 'Encrypts the backup file (can differ from master password)' : '用于加密备份文件（可与主密码不同）'"
              style="max-width: 360px"
            />
          </n-form-item>
        </n-form>
        <n-space>
          <n-button type="primary" :loading="exporting" @click="exportBackup">
            {{ locale === 'en' ? 'Export backup' : '导出备份' }}
          </n-button>
          <n-button :loading="importing" @click="importBackup">
            {{ locale === 'en' ? 'Restore from backup' : '从备份还原' }}
          </n-button>
        </n-space>
        <div style="margin-top: 10px; font-size: 12px; opacity: 0.55">
          {{ locale === 'en'
            ? 'Backups are independently encrypted with Argon2id + AES-256-GCM and contain URLs, secrets, OTP and notes. Restore is merge-mode: existing data is kept.'
            : '备份文件使用 Argon2id + AES-256-GCM 独立加密，包含 URL、密码、OTP 和记事本全部数据。还原为合并模式：现有数据保留，备份中的记录追加导入。' }}
        </div>
      </n-card>

      <!-- Auto backup -->
      <n-card :title="t('autoBackup.title')" size="small">
        <div class="hint" style="margin-bottom: 10px">{{ t('autoBackup.desc') }}</div>
        <n-form label-placement="left" label-width="96">
          <n-form-item :label="t('autoBackup.enable')">
            <n-switch v-model:value="autoEnabled" />
            <span v-if="autoBackup" style="margin-left: 12px; font-size: 12px; opacity: 0.55">
              {{ t('autoBackup.lastRun') }}: {{ fmtTs(autoBackup.lastAt) }}
              <template v-if="autoBackup.nextAt"> · {{ t('autoBackup.nextRun') }}: {{ fmtTs(autoBackup.nextAt) }}</template>
              · {{ t('autoBackup.fileCount', { n: autoBackup.count }) }}
            </span>
          </n-form-item>
          <n-form-item :label="t('autoBackup.dir')">
            <n-input v-model:value="autoDir" readonly :placeholder="t('autoBackup.dirPlaceholder')" style="flex: 1" />
            <n-button style="margin-left: 8px" @click="pickBackupDir">{{ t('autoBackup.pickDir') }}</n-button>
          </n-form-item>
          <n-form-item :label="t('autoBackup.interval')">
            <n-select v-model:value="autoInterval" :options="intervalOptions" style="width: 160px" />
            <span class="hint" style="margin-left: 16px">{{ t('autoBackup.keep') }}</span>
            <n-input-number v-model:value="autoKeep" :min="1" :max="100" size="small" style="width: 90px; margin-left: 8px" />
          </n-form-item>
          <n-form-item :label="t('autoBackup.password')">
            <n-input
              v-model:value="autoPwd"
              type="password"
              show-password-on="click"
              :placeholder="autoBackup?.hasPassword ? t('autoBackup.passwordKeep') : t('autoBackup.passwordSet')"
              style="max-width: 360px"
            />
          </n-form-item>
        </n-form>
        <n-space>
          <n-button type="primary" :loading="autoSaving" @click="saveAutoBackupSettings">{{ t('autoBackup.save') }}</n-button>
          <n-button :loading="autoRunning" @click="runAutoBackupNow">{{ t('autoBackup.runNow') }}</n-button>
        </n-space>
        <div style="margin-top: 10px; font-size: 12px; opacity: 0.55">{{ t('autoBackup.securityHint') }}</div>

        <!-- Restore history: pick any previous auto-backup and merge it back -->
        <n-divider style="margin: 16px 0 12px">{{ t('autoBackup.history') }}</n-divider>
        <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px">
          <span class="hint">{{ t('autoBackup.historyHint') }}</span>
          <n-button size="tiny" quaternary :loading="backupFilesLoading" @click="loadBackupFiles">🔄</n-button>
        </div>
        <n-empty v-if="backupFiles.length === 0" :description="t('autoBackup.historyEmpty')" size="small" />
        <n-table v-else :bordered="false" size="small">
          <thead>
            <tr>
              <th>{{ t('autoBackup.colFile') }}</th>
              <th>{{ t('autoBackup.colTime') }}</th>
              <th>{{ t('autoBackup.colSize') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="f in backupFiles" :key="f.path">
              <td style="max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap" :title="f.name">{{ f.name }}</td>
              <td>{{ fmtTs(f.modTime) }}</td>
              <td>{{ fmtSize(f.size) }}</td>
              <td style="text-align: right">
                <n-button size="tiny" @click="openRestore(f)">{{ t('autoBackup.restoreBtn') }}</n-button>
              </td>
            </tr>
          </tbody>
        </n-table>
      </n-card>
    </n-space>
  </div>

  <n-modal v-model:show="showRestoreModal" preset="card" :title="t('autoBackup.restoreTitle')" style="width: 460px">
    <div v-if="restoreTarget" class="hint" style="margin-bottom: 12px">
      {{ restoreTarget.name }} · {{ fmtTs(restoreTarget.modTime) }}
    </div>
    <n-input
      v-model:value="restorePwd"
      type="password"
      show-password-on="click"
      :placeholder="t('autoBackup.passwordNeeded')"
    />
    <div class="hint" style="margin-top: 10px">{{ t('autoBackup.restoreWarn') }}</div>
    <template #footer>
      <div style="display: flex; justify-content: flex-end; gap: 8px">
        <n-button @click="showRestoreModal = false">{{ t('autoBackup.cancel') }}</n-button>
        <n-button type="primary" :loading="!!restoringPath" @click="confirmRestore">{{ t('autoBackup.restoreBtn') }}</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.recovery-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
  align-items: center;
}
.lang-row {
  display: flex;
  align-items: center;
  gap: 16px;
}
.hint {
  font-size: 12px;
  opacity: 0.55;
}
.audit-stats {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.audit-tag-active {
  outline: 2px solid rgba(99, 226, 183, 0.7);
  outline-offset: 1px;
}
.audit-detail {
  max-width: 320px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.import-bar {
  display: flex;
  align-items: center;
  gap: 10px;
}
.import-preview-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
  gap: 12px;
  flex-wrap: wrap;
}
.about-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 24px;
}
.about-item {
  display: flex;
  gap: 8px;
  align-items: baseline;
}
.about-item.about-desc {
  grid-column: 1 / -1;
}
.about-item .k {
  font-size: 12px;
  opacity: 0.55;
  min-width: 70px;
}
.about-item .v {
  font-size: 13px;
}
.accent-swatches {
  display: flex;
  align-items: center;
  gap: 8px;
}
.accent-swatch {
  width: 22px;
  height: 22px;
  border-radius: 4px;
  cursor: pointer;
  border: 2px solid transparent;
  box-sizing: border-box;
}
.accent-swatch.active {
  border-color: var(--nb-fg);
}
.accent-picker {
  width: 30px;
  height: 24px;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
}
</style>
