<template>
  <BaseDialog :show="show" :title="t('admin.users.vipTier.title')" width="wide" @close="$emit('close')">
    <div v-if="user" class="space-y-6">
      <div class="border-b border-line pb-4">
        <p class="truncate font-semibold text-ink">{{ user.email }}</p>
        <p class="mt-1 text-xs text-ink-tertiary">{{ t('admin.users.vipTier.hint') }}</p>
      </div>

      <div v-if="loading" class="flex justify-center py-12">
        <LoadingSpinner />
      </div>

      <div v-else class="space-y-6">
        <!--
          Current standing. Spend figures are read-only here: grading is driven
          by completed orders, and letting an admin edit the accumulator would
          desynchronise it from the payment ledger.
        -->
        <div class="rounded border border-line bg-surface-sunken p-4">
          <div class="flex items-center gap-2">
            <span
              v-if="status?.tier"
              class="inline-flex items-center rounded-sm px-2 py-0.5 text-2xs font-medium uppercase tracking-[0.04em] text-white"
              :style="{ backgroundColor: status.tier.badge_color }"
            >
              {{ status.tier.name }}
            </span>
            <span
              v-else
              class="inline-flex items-center rounded-sm border border-line bg-surface px-2 py-0.5 text-2xs font-medium uppercase tracking-[0.04em] text-ink-secondary"
            >
              {{ t('vip.baseTier') }}
            </span>

            <span
              v-if="status?.locked"
              class="inline-flex items-center rounded-sm border border-line bg-surface px-1.5 py-0.5 text-2xs font-medium uppercase tracking-[0.04em] text-ink-secondary"
            >
              {{ t('admin.users.vipTier.pinned') }}
            </span>
          </div>

          <dl class="mt-3 grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
            <div class="flex justify-between gap-3">
              <dt class="text-ink-tertiary">{{ t('admin.users.vipTier.qualifyingSpend') }}</dt>
              <dd class="font-medium text-ink-secondary">${{ formatUSD(status?.qualifying_spend) }}</dd>
            </div>
            <div class="flex justify-between gap-3">
              <dt class="text-ink-tertiary">{{ t('admin.users.vipTier.totalPaid') }}</dt>
              <dd class="font-medium text-ink-secondary">${{ formatUSD(status?.total_paid_usd) }}</dd>
            </div>
            <div class="flex justify-between gap-3">
              <dt class="text-ink-tertiary">{{ t('admin.users.vipTier.expiresAt') }}</dt>
              <dd class="font-medium text-ink-secondary">
                {{ status?.locked ? t('admin.users.vipTier.never') : (status?.expires_at ? formatDateTime(status.expires_at) : '-') }}
              </dd>
            </div>
            <div class="flex justify-between gap-3">
              <dt class="text-ink-tertiary">{{ t('admin.users.vipTier.nextTier') }}</dt>
              <dd class="font-medium text-ink-secondary">
                <span v-if="status?.next_tier">
                  {{ status.next_tier.name }} (${{ formatUSD(status.spend_to_next_tier) }})
                </span>
                <span v-else>-</span>
              </dd>
            </div>
          </dl>
        </div>

        <!-- Assignment -->
        <div>
          <h4 class="mb-3 text-sm font-semibold text-ink-secondary">{{ t('admin.users.vipTier.assign') }}</h4>
          <div class="grid gap-2">
            <label
              class="flex cursor-pointer items-center gap-3 rounded border p-3 transition-colors duration-fast"
              :class="selectedTierId === null ? 'border-accent bg-accent-tint' : 'border-line bg-surface hover:border-line-strong'"
            >
              <input v-model="selectedTierId" type="radio" :value="null" class="accent-current" />
              <div class="min-w-0">
                <p class="text-sm font-medium text-ink">{{ t('admin.users.vipTier.automatic') }}</p>
                <p class="text-xs text-ink-tertiary">{{ t('admin.users.vipTier.automaticHint') }}</p>
              </div>
            </label>

            <label
              v-for="tier in assignableTiers"
              :key="tier.id"
              class="flex cursor-pointer items-center gap-3 rounded border p-3 transition-colors duration-fast"
              :class="selectedTierId === tier.id ? 'border-accent bg-accent-tint' : 'border-line bg-surface hover:border-line-strong'"
            >
              <input v-model="selectedTierId" type="radio" :value="tier.id" class="accent-current" />
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span
                    class="inline-flex items-center rounded-sm px-2 py-0.5 text-2xs font-medium uppercase tracking-[0.04em] text-white"
                    :style="{ backgroundColor: tier.badge_color }"
                  >
                    {{ tier.name }}
                  </span>
                  <span class="text-xs text-ink-tertiary">L{{ tier.level }}</span>
                </div>
                <p class="mt-1 text-xs text-ink-tertiary">
                  {{ t('admin.users.vipTier.tierSummary', {
                    rate: tier.rate_multiplier,
                    concurrency: tier.concurrency
                  }) }}
                </p>
              </div>
            </label>
          </div>

          <p v-if="selectedTierId !== null" class="mt-3 text-xs text-warn">
            {{ t('admin.users.vipTier.pinWarning') }}
          </p>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" class="btn btn-secondary px-5">{{ t('common.cancel') }}</button>
        <button @click="handleSave" :disabled="submitting || loading" class="btn btn-primary px-6">
          {{ submitting ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser, VIPStatus, VIPTier } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{
  show: boolean
  user: AdminUser | null
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const status = ref<VIPStatus | null>(null)
const tiers = ref<VIPTier[]>([])
const selectedTierId = ref<number | null>(null)

// A disabled tier can still be pinned to a user by an earlier decision, so keep
// it in the list when it is the current selection — dropping it would silently
// reassign the user on save.
const assignableTiers = computed(() =>
  tiers.value.filter(tier => tier.enabled || tier.id === selectedTierId.value)
)

const formatUSD = (value: number | undefined) => (value ?? 0).toFixed(2)

const load = async () => {
  if (!props.user) return
  loading.value = true
  try {
    const [userStatus, tierList] = await Promise.all([
      adminAPI.vipTiers.getUserStatus(props.user.id),
      adminAPI.vipTiers.list()
    ])
    status.value = userStatus
    tiers.value = tierList
    // Only a pinned tier is an admin decision worth pre-selecting; an
    // automatically graded tier must show as "automatic" so that saving
    // without touching anything does not convert it into a pin.
    selectedTierId.value = userStatus.locked ? (userStatus.tier?.id ?? null) : null
  } catch {
    appStore.showError(t('admin.users.vipTier.loadFailed'))
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  if (!props.user) return
  submitting.value = true
  try {
    await adminAPI.vipTiers.setUserTier(props.user.id, selectedTierId.value)
    appStore.showSuccess(t('admin.users.vipTier.saved'))
    emit('success')
    emit('close')
  } catch {
    appStore.showError(t('admin.users.vipTier.saveFailed'))
  } finally {
    submitting.value = false
  }
}

watch(
  () => [props.show, props.user?.id],
  ([show]) => {
    if (show) load()
  }
)
</script>
