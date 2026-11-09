<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NSpace,
  useMessage,
} from 'naive-ui'
import { VaultService } from '../lib/api'
import { refreshStatus, vault } from '../lib/vaultState'

const router = useRouter()
const message = useMessage()

const password = ref('')
const confirm = ref('')
const loading = ref(false)

// ---- Forgot password (recovery) flow ----
const showRecovery = ref(false)
const recoveryQuestions = ref<string[]>([])
const recoveryAnswers = ref<string[]>([])
const newMasterPwd = ref('')
const newMasterConfirm = ref('')
const resetting = ref(false)

const initialised = computed(() => vault.initialised)

onMounted(async () => {
  await refreshStatus()
  // Preload recovery questions if configured.
  try {
    const has = await VaultService.HasRecovery()
    if (has) {
      recoveryQuestions.value = ((await VaultService.RecoveryQuestions()) ?? []) as string[]
    }
  } catch {
    /* ignore */
  }
})

async function submit() {
  if (!password.value) {
    message.warning('请输入主密码')
    return
  }
  if (!initialised.value && password.value !== confirm.value) {
    message.warning('两次输入的密码不一致')
    return
  }
  loading.value = true
  try {
    if (!initialised.value) {
      await VaultService.Init(password.value)
      message.success('保险箱已创建')
    } else {
      await VaultService.Unlock(password.value)
    }
    await refreshStatus()
    password.value = ''
    confirm.value = ''
    router.push({ name: 'urls' })
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    message.error(initialised.value ? `解锁失败：${msg}` : `创建失败：${msg}`)
  } finally {
    loading.value = false
  }
}

function openRecovery() {
  if (recoveryQuestions.value.length === 0) {
    message.warning('未设置密保问题，无法通过密保重置密码')
    return
  }
  recoveryAnswers.value = recoveryQuestions.value.map(() => '')
  newMasterPwd.value = ''
  newMasterConfirm.value = ''
  showRecovery.value = true
}

async function doReset() {
  if (recoveryAnswers.value.some((a) => !a.trim())) {
    message.warning('请回答所有密保问题')
    return
  }
  if (newMasterPwd.value.length < 8) {
    message.warning('新主密码至少 8 个字符')
    return
  }
  if (newMasterPwd.value !== newMasterConfirm.value) {
    message.warning('两次输入的新密码不一致')
    return
  }
  resetting.value = true
  try {
    await VaultService.ResetPasswordWithRecovery(recoveryAnswers.value, newMasterPwd.value)
    message.success('主密码已重置，请使用新密码解锁')
    showRecovery.value = false
    password.value = ''
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    message.error(`重置失败：${msg}`)
  } finally {
    resetting.value = false
  }
}
</script>

<template>
  <div class="center">
    <n-card style="width: 380px" :title="initialised ? '解锁' : '创建保险箱'">
      <n-form @submit.prevent="submit">
        <n-form-item :label="initialised ? '主密码' : '设置主密码'">
          <n-input
            v-model:value="password"
            type="password"
            show-password-on="click"
            placeholder="主密码"
            @keyup.enter="submit"
          />
        </n-form-item>
        <n-form-item v-if="!initialised" label="确认密码">
          <n-input
            v-model:value="confirm"
            type="password"
            show-password-on="click"
            placeholder="再次输入"
            @keyup.enter="submit"
          />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="submit">
          {{ initialised ? '解锁' : '创建并解锁' }}
        </n-button>
      </n-form>
      <template #footer>
        <div class="footer-row">
          <span style="font-size: 12px; opacity: 0.6">
            {{ initialised ? '主密码用于派生加密密钥。' : '主密码用于派生加密密钥，丢失后无法恢复。' }}
          </span>
          <n-button
            v-if="initialised && recoveryQuestions.length > 0"
            text
            type="primary"
            size="tiny"
            @click="openRecovery"
          >
            忘记密码？
          </n-button>
        </div>
      </template>
    </n-card>

    <!-- Recovery reset modal -->
    <n-modal v-model:show="showRecovery" preset="card" style="width: 460px" title="通过密保问题重置主密码">
      <n-form label-placement="top">
        <n-form-item v-for="(q, i) in recoveryQuestions" :key="i" :label="q">
          <n-input v-model:value="recoveryAnswers[i]" placeholder="输入答案" @keyup.enter="doReset" />
        </n-form-item>
        <n-form-item label="新主密码">
          <n-input v-model:value="newMasterPwd" type="password" show-password-on="click" placeholder="至少 8 个字符" />
        </n-form-item>
        <n-form-item label="确认新主密码">
          <n-input v-model:value="newMasterConfirm" type="password" show-password-on="click" @keyup.enter="doReset" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="footer-row">
          <span style="font-size: 12px; opacity: 0.55">答案不区分大小写</span>
          <n-space>
            <n-button @click="showRecovery = false">取消</n-button>
            <n-button type="primary" :loading="resetting" @click="doReset">重置密码</n-button>
          </n-space>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.center {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
.footer-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
</style>
