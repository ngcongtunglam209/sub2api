<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header
        class="page-header mb-0 rounded-3xl bg-surface p-5 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700 sm:p-6"
      >
        <h1 class="page-title flex items-center gap-2 text-xl font-black text-ink">
          {{ t('admin.vipTiers.title') }}
        </h1>
        <p class="page-description mt-1.5 text-xs text-ink-secondary">
          {{ t('admin.vipTiers.description') }}
        </p>
      </header>

      <div class="rounded-3xl bg-surface p-5 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700 sm:p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <span class="text-sm text-ink-secondary">{{ t('admin.vipTiers.ladderHint') }}</span>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="load">
              {{ t('common.refresh') }}
            </button>
            <button type="button" class="btn btn-primary" @click="openCreate">
              {{ t('admin.vipTiers.create') }}
            </button>
          </div>
        </div>

        <DataTable :columns="columns" :data="tiers" :loading="loading" row-key="id">
          <template #cell-name="{ row }">
            <div class="flex items-center gap-2">
              <span
                class="inline-block h-2.5 w-2.5 rounded-full"
                :style="{ backgroundColor: row.badge_color || '#9ca3af' }"
              ></span>
              <span class="font-medium text-ink">{{ row.name }}</span>
              <span class="text-xs text-ink-tertiary">L{{ row.level }}</span>
            </div>
          </template>

          <template #cell-min_spend_usd="{ row }">
            <span class="font-mono tabular-nums">${{ formatAmount(row.min_spend_usd) }}</span>
          </template>

          <template #cell-rate_multiplier="{ row }">
            <div class="flex items-center gap-1.5">
              <span class="font-mono tabular-nums">{{ row.rate_multiplier.toFixed(2) }}x</span>
              <span class="text-xs text-ink-tertiary">
                {{ t('admin.vipTiers.discountOf', { percent: discountPercent(row.rate_multiplier) }) }}
              </span>
            </div>
          </template>

          <!--
            The exemption has to win over the number here. Both columns hold an
            addend, so an unlimited tier normally leaves it at whatever it was —
            printing that would advertise the top tier as the stingiest one.
          -->
          <template #cell-concurrency="{ row }">
            <span v-if="row.unlimited_concurrency" class="badge badge-success">
              {{ t('admin.vipTiers.unlimited') }}
            </span>
            <span v-else class="font-mono tabular-nums">+{{ row.concurrency }}</span>
          </template>

          <template #cell-rpm_limit="{ row }">
            <span v-if="row.unlimited_rpm" class="badge badge-success">
              {{ t('admin.vipTiers.unlimited') }}
            </span>
            <span v-else-if="row.rpm_limit > 0" class="font-mono tabular-nums">
              +{{ row.rpm_limit }}
            </span>
            <span v-else class="text-ink-tertiary">—</span>
          </template>

          <template #cell-grace_days="{ row }">
            <span class="font-mono tabular-nums">{{ row.grace_days }}</span>
          </template>

          <template #cell-enabled="{ row }">
            <span :class="['badge', row.enabled ? 'badge-success' : 'badge-danger']">
              {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
            </span>
          </template>

          <template #actions="{ row }">
            <button type="button" class="btn btn-secondary btn-sm" @click="openEdit(row)">
              {{ t('common.edit') }}
            </button>
            <button type="button" class="btn btn-danger btn-sm" @click="deletingTier = row">
              {{ t('common.delete') }}
            </button>
          </template>
        </DataTable>
      </div>
    </div>

    <BaseDialog
      :show="showEditor"
      :title="editingTier ? t('admin.vipTiers.editTitle') : t('admin.vipTiers.createTitle')"
      @close="showEditor = false"
    >
      <form id="vip-tier-form" class="space-y-4" @submit.prevent="save">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.vipTiers.fields.level') }}</label>
            <input v-model.number="form.level" type="number" min="1" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.vipTiers.fields.name') }}</label>
            <input v-model="form.name" type="text" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.vipTiers.fields.minSpend') }}</label>
            <input v-model.number="form.min_spend_usd" type="number" min="0" step="1" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.vipTiers.fields.rateMultiplier') }}</label>
            <input
              v-model.number="form.rate_multiplier"
              type="number"
              min="0.01"
              max="1"
              step="0.01"
              class="input"
            />
            <p class="mt-1 text-xs text-ink-tertiary">
              {{ t('admin.vipTiers.fields.rateMultiplierHint') }}
            </p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.vipTiers.fields.concurrency') }}</label>
            <!--
              Disabled rather than hidden when exempt: the number stays visible so
              an admin can see what the tier falls back to if they clear the
              exemption, without being able to edit a value that has no effect.
            -->
            <input
              v-model.number="form.concurrency"
              type="number"
              min="1"
              class="input"
              :disabled="form.unlimited_concurrency"
            />
            <label class="mt-2 flex items-center gap-2 text-sm text-ink-secondary">
              <input v-model="form.unlimited_concurrency" type="checkbox" />
              {{ t('admin.vipTiers.fields.unlimitedConcurrency') }}
            </label>
            <p class="mt-1 text-xs text-ink-tertiary">
              {{
                form.unlimited_concurrency
                  ? t('admin.vipTiers.fields.unlimitedConcurrencyHint')
                  : t('admin.vipTiers.fields.concurrencyHint')
              }}
            </p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.vipTiers.fields.rpmLimit') }}</label>
            <input
              v-model.number="form.rpm_limit"
              type="number"
              min="0"
              class="input"
              :disabled="form.unlimited_rpm"
            />
            <label class="mt-2 flex items-center gap-2 text-sm text-ink-secondary">
              <input v-model="form.unlimited_rpm" type="checkbox" />
              {{ t('admin.vipTiers.fields.unlimitedRpm') }}
            </label>
            <p class="mt-1 text-xs text-ink-tertiary">
              {{
                form.unlimited_rpm
                  ? t('admin.vipTiers.fields.unlimitedRpmHint')
                  : t('admin.vipTiers.fields.rpmLimitHint')
              }}
            </p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.vipTiers.fields.graceDays') }}</label>
            <input v-model.number="form.grace_days" type="number" min="1" class="input" />
            <p class="mt-1 text-xs text-ink-tertiary">
              {{ t('admin.vipTiers.fields.graceDaysHint') }}
            </p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.vipTiers.fields.badgeColor') }}</label>
            <input v-model="form.badge_color" type="text" placeholder="#f59e0b" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.vipTiers.fields.enabled') }}</label>
            <label class="flex items-center gap-2 text-sm text-ink-secondary">
              <input v-model="form.enabled" type="checkbox" />
              {{ t('admin.vipTiers.fields.enabledHint') }}
            </label>
          </div>
        </div>
        <p v-if="editorError" class="text-sm text-danger">{{ editorError }}</p>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showEditor = false">
          {{ t('common.cancel') }}
        </button>
        <button type="submit" form="vip-tier-form" class="btn btn-primary" :disabled="saving">
          {{ t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="!!deletingTier"
      :title="t('admin.vipTiers.deleteTitle')"
      :message="t('admin.vipTiers.deleteMessage', { name: deletingTier?.name ?? '' })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="deletingTier = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { Column } from '@/components/common/types'
import type { VIPTier, VIPTierRequest } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const tiers = ref<VIPTier[]>([])
const loading = ref(false)
const saving = ref(false)
const showEditor = ref(false)
const editingTier = ref<VIPTier | null>(null)
const deletingTier = ref<VIPTier | null>(null)
const editorError = ref('')

const form = reactive<Required<VIPTierRequest>>({
  level: 1,
  name: '',
  min_spend_usd: 0,
  rate_multiplier: 1,
  concurrency: 5,
  rpm_limit: 0,
  unlimited_concurrency: false,
  unlimited_rpm: false,
  grace_days: 60,
  badge_color: '',
  enabled: true
})

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.vipTiers.fields.name') },
  { key: 'min_spend_usd', label: t('admin.vipTiers.fields.minSpend'), sortable: true },
  { key: 'rate_multiplier', label: t('admin.vipTiers.fields.rateMultiplier') },
  { key: 'concurrency', label: t('admin.vipTiers.fields.concurrency') },
  { key: 'rpm_limit', label: t('admin.vipTiers.fields.rpmLimit') },
  { key: 'grace_days', label: t('admin.vipTiers.fields.graceDays') },
  { key: 'enabled', label: t('admin.vipTiers.fields.enabled') }
])

function formatAmount(value: number): string {
  return value.toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: 2 })
}

// 0.82 reads as "18% off", which is how the ladder is actually discussed.
function discountPercent(multiplier: number): string {
  return Math.round((1 - multiplier) * 100).toString()
}

async function load() {
  loading.value = true
  try {
    tiers.value = await adminAPI.vipTiers.list()
  } catch (error) {
    console.error('Error loading VIP tiers:', error)
    appStore.showError(t('admin.vipTiers.loadFailed'))
  } finally {
    loading.value = false
  }
}

function resetForm(tier: VIPTier | null) {
  const nextLevel = tiers.value.reduce((max, item) => Math.max(max, item.level), 0) + 1
  form.level = tier?.level ?? nextLevel
  form.name = tier?.name ?? `VIP${nextLevel}`
  form.min_spend_usd = tier?.min_spend_usd ?? 0
  form.rate_multiplier = tier?.rate_multiplier ?? 1
  form.concurrency = tier?.concurrency ?? 5
  form.rpm_limit = tier?.rpm_limit ?? 0
  form.unlimited_concurrency = tier?.unlimited_concurrency ?? false
  form.unlimited_rpm = tier?.unlimited_rpm ?? false
  form.grace_days = tier?.grace_days ?? 60
  form.badge_color = tier?.badge_color ?? ''
  form.enabled = tier?.enabled ?? true
  editorError.value = ''
}

function openCreate() {
  editingTier.value = null
  resetForm(null)
  showEditor.value = true
}

function openEdit(tier: VIPTier) {
  editingTier.value = tier
  resetForm(tier)
  showEditor.value = true
}

async function save() {
  saving.value = true
  editorError.value = ''
  try {
    if (editingTier.value) {
      await adminAPI.vipTiers.update(editingTier.value.id, { ...form })
    } else {
      await adminAPI.vipTiers.create({ ...form })
    }
    showEditor.value = false
    appStore.showSuccess(t('admin.vipTiers.saved'))
    await load()
  } catch (error: any) {
    // The ladder check runs on the server and its message names which two
    // tiers are out of order, so show that rather than a generic failure.
    editorError.value = error?.response?.data?.message || t('admin.vipTiers.saveFailed')
  } finally {
    saving.value = false
  }
}

async function confirmDelete() {
  const tier = deletingTier.value
  deletingTier.value = null
  if (!tier) return
  try {
    await adminAPI.vipTiers.remove(tier.id)
    appStore.showSuccess(t('admin.vipTiers.deleted'))
    await load()
  } catch (error) {
    console.error('Error deleting VIP tier:', error)
    appStore.showError(t('admin.vipTiers.deleteFailed'))
  }
}

onMounted(load)
</script>
