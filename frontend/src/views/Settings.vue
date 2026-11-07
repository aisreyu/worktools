<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSpace,
  NTable,
  NTag,
  useMessage,
} from 'naive-ui'
import { Dialogs } from '@wailsio/runtime'
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
  type ImportedCredential,
  type ImportedCredentials,
} from '../lib/api'
import { refreshStatus } from '../lib/vaultState'
import { locale, setLocale, t, errText, detailText } from '../lib/i18n'

const message = useMessage()

// ---- Language ----
const langOptions = [
  { label: '简体中文', value: 'zh' },
  { label: 'English', value: 'en' },
]

function onLangChange(v: string) {
  setLocale(v as 'zh' | 'en')
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
  const path = await Dialogs.SaveFile({
    Title: t('settings.backupNow'),
    Filename: `worktools-backup-${new Date().toISOString().slice(0, 10)}.json`,
    Filters: [{ DisplayName: 'JSON', Pattern: '*.json' }],
  })
  if (!path) return
  exporting.value = true
  try {
    await BackupService.Export(path, backupPwd.value)
    message.success(`${t('settings.backupDone')}: ${path}`)
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

// ---- Password health audit ----
const auditReport = ref<PasswordAuditReport | null>(null)
const auditing = ref(false)

async function runAudit() {
  auditing.value = true
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
            <n-tag size="small" round type="error">{{ t('audit.weak') }} {{ auditReport.weak }}</n-tag>
            <n-tag size="small" round type="warning">{{ t('audit.reused') }} {{ auditReport.reused }}</n-tag>
            <n-tag size="small" round type="default">{{ t('audit.old') }} {{ auditReport.old }}</n-tag>
          </div>
          <n-empty v-if="(auditReport.findings ?? []).length === 0" :description="t('audit.clean')" style="padding: 20px 0" />
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
              <tr v-for="(f, i) in auditReport.findings ?? []" :key="i">
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
    </n-space>
  </div>
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
</style>
