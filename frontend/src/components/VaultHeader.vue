<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NTag } from 'naive-ui'
import { vault } from '../lib/vaultState'

defineEmits<{ (e: 'lock'): void }>()

const locked = computed(() => !vault.unlocked)
</script>

<template>
  <div class="row-between nb-drag" style="height: 44px; padding: 0 16px">
    <n-tag class="nb-no-drag" size="small" :type="locked ? 'warning' : 'success'">
      {{ locked ? '已锁定' : '已解锁' }}
    </n-tag>
    <n-button
      v-if="!locked"
      class="nb-no-drag"
      size="small"
      secondary
      @click="$emit('lock')"
    >
      锁定
    </n-button>
  </div>
</template>
