<template>
  <span
    v-if="hasTierLadder"
    :class="[
      'inline-flex items-center rounded-sm px-1.5 py-0.5 text-2xs font-medium uppercase tracking-[0.04em]',
      tier ? 'text-white' : 'border border-line bg-surface-sunken text-ink-tertiary'
    ]"
    :style="tier ? { backgroundColor: tier.badge_color } : undefined"
    :title="tooltip || undefined"
  >
    <!--
      Unranked reads as a starting rank, not as missing data: BASE names the
      rung the user already stands on, matching the dashboard status card.
    -->
    {{ tier ? tier.name : t('vip.baseTier') }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useVipStatus } from '@/composables/useVipStatus'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()

const { status, tier, hasTierLadder } = useVipStatus()

const tooltip = computed(() => {
  const current = status.value
  if (!current) return ''
  if (current.locked) return t('vip.pinned')
  if (current.expires_at) return t('vip.expiresOn', { date: formatDateTime(current.expires_at) })
  return ''
})
</script>
