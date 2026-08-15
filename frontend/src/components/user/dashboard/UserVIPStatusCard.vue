<template>
  <div
    v-if="status"
    class="rounded-3xl bg-surface p-5 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700"
  >
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <span
          class="inline-block h-3 w-3 rounded-full"
          :style="{ backgroundColor: status.tier?.badge_color || '#9ca3af' }"
        ></span>
        <span v-if="status.tier" class="text-base font-bold text-ink">
          {{ status.tier.name }}
        </span>
        <!--
          Unranked reads as a starting rank, not as missing data. "No tier yet"
          framed the common case as an absence; BASE names it, so the ladder
          starts somewhere the user already stands.
        -->
        <span
          v-else
          class="inline-flex items-center rounded-sm border border-line bg-surface-sunken px-2 py-0.5 text-2xs font-semibold uppercase tracking-[0.08em] text-ink-secondary"
        >
          {{ t('vip.baseTier') }}
        </span>
        <span v-if="discountLabel" class="badge badge-success">{{ discountLabel }}</span>
      </div>
      <div class="text-xs text-ink-tertiary">
        <span v-if="status.locked">{{ t('vip.pinned') }}</span>
        <span v-else-if="status.expires_at">
          {{ t('vip.expiresOn', { date: formatDateTime(status.expires_at) }) }}
        </span>
      </div>
    </div>

    <div class="mt-3 grid grid-cols-2 gap-3 text-xs text-ink-secondary sm:grid-cols-3">
      <div>
        <div class="text-ink-tertiary">{{ t('vip.qualifyingSpend') }}</div>
        <div class="font-mono tabular-nums text-ink">${{ formatUsd(status.qualifying_spend) }}</div>
      </div>
      <div v-if="status.tier">
        <div class="text-ink-tertiary">{{ t('vip.concurrency') }}</div>
        <div class="font-mono tabular-nums text-ink">{{ status.tier.concurrency }}</div>
      </div>
      <div>
        <div class="text-ink-tertiary">{{ t('vip.totalPaid') }}</div>
        <div class="font-mono tabular-nums text-ink">${{ formatUsd(status.total_paid_usd) }}</div>
      </div>
    </div>

    <div v-if="status.next_tier" class="mt-4">
      <div class="mb-1.5 flex items-center justify-between text-xs">
        <span class="text-ink-secondary">
          {{ t('vip.spendToNext', { amount: formatUsd(status.spend_to_next_tier), tier: status.next_tier.name }) }}
        </span>
        <span class="font-mono tabular-nums text-ink-tertiary">{{ progressPercent }}%</span>
      </div>
      <div class="h-2 w-full overflow-hidden rounded-full bg-surface-active">
        <div
          class="h-full rounded-full bg-success transition-all"
          :style="{ width: `${progressPercent}%` }"
        ></div>
      </div>
    </div>
    <p v-else-if="status.tier" class="mt-4 text-xs text-ink-tertiary">{{ t('vip.topTier') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import vipAPI from '@/api/vip'
import { formatDateTime } from '@/utils/format'
import type { VIPStatus } from '@/types'

const { t } = useI18n()

const status = ref<VIPStatus | null>(null)

const discountLabel = computed(() => {
  const multiplier = status.value?.tier?.rate_multiplier
  if (!multiplier || multiplier >= 1) return ''
  return t('vip.discountBadge', { percent: Math.round((1 - multiplier) * 100) })
})

// Progress is measured across the current rung, not from zero: sitting at VIP2
// with 300 of the 400 VIP3 needs should read as most of the way there, not as
// three quarters of everything ever spent.
const progressPercent = computed(() => {
  const s = status.value
  if (!s?.next_tier) return 0
  const floor = s.tier?.min_spend_usd ?? 0
  const span = s.next_tier.min_spend_usd - floor
  if (span <= 0) return 0
  const progressed = s.qualifying_spend - floor
  return Math.min(100, Math.max(0, Math.round((progressed / span) * 100)))
})

function formatUsd(value: number): string {
  return value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

onMounted(async () => {
  try {
    status.value = await vipAPI.getStatus()
  } catch (error) {
    // A site with no tiers configured is the normal case, not an error worth
    // shouting about: leave the card hidden.
    console.debug('VIP status unavailable:', error)
  }
})
</script>
