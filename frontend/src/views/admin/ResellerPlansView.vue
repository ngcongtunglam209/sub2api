<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header
        class="page-header mb-0 rounded-3xl bg-surface p-5 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700 sm:p-6"
      >
        <h1 class="page-title flex items-center gap-2 text-xl font-black text-ink">
          {{ t('admin.resellerPlans.title') }}
        </h1>
        <p class="page-description mt-1.5 text-xs text-ink-secondary">
          {{ t('admin.resellerPlans.description') }}
        </p>
      </header>

      <div class="rounded-3xl bg-surface p-5 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700 sm:p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <span class="text-sm text-ink-secondary">{{ t('admin.resellerPlans.ladderHint') }}</span>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="load">
              {{ t('common.refresh') }}
            </button>
          </div>
        </div>

        <DataTable :columns="columns" :data="plans" :loading="loading" row-key="id">
          <template #cell-name="{ row }">
            <div class="flex items-center gap-2">
              <span class="font-medium text-ink">{{ row.name }}</span>
              <span class="text-xs text-ink-tertiary">L{{ row.level }}</span>
            </div>
          </template>

          <template #cell-price="{ row }">
            <span class="font-mono tabular-nums">${{ formatAmount(row.price) }}</span>
          </template>

          <template #cell-credit_rate="{ row }">
            <div class="flex items-center gap-1.5">
              <span class="font-mono tabular-nums">{{ row.credit_rate.toFixed(2) }}x</span>
              <span class="text-xs text-ink-tertiary">
                {{ t('admin.resellerPlans.marginOf', { percent: marginPercent(row.credit_rate) }) }}
              </span>
            </div>
          </template>

          <template #cell-concurrency_bonus="{ row }">
            <span class="font-mono tabular-nums">+{{ row.concurrency_bonus }}</span>
          </template>

          <template #cell-rpm_limit="{ row }">
            <span class="font-mono tabular-nums">{{ row.rpm_limit }}</span>
          </template>

          <template #cell-max_domains="{ row }">
            <span class="font-mono tabular-nums">{{ row.max_domains }}</span>
          </template>

          <template #cell-validity_days="{ row }">
            <span class="font-mono tabular-nums">{{ row.validity_days }}</span>
          </template>

          <template #cell-allowed_group_ids="{ row }">
            <span class="text-xs text-ink-secondary">{{ groupSummary(row.allowed_group_ids) }}</span>
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
          </template>
        </DataTable>
      </div>
    </div>

    <BaseDialog
      :show="showEditor"
      :title="t('admin.resellerPlans.editTitle')"
      @close="showEditor = false"
    >
      <form id="reseller-plan-form" class="space-y-4" @submit.prevent="save">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.resellerPlans.fields.level') }}</label>
            <input v-model.number="form.level" type="number" min="1" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.resellerPlans.fields.name') }}</label>
            <input v-model="form.name" type="text" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.resellerPlans.fields.price') }}</label>
            <input v-model.number="form.price" type="number" min="0" step="1" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.resellerPlans.fields.creditRate') }}</label>
            <input v-model.number="form.credit_rate" type="number" min="0.01" step="0.01" class="input" />
            <p class="mt-1 text-xs text-ink-tertiary">
              {{ t('admin.resellerPlans.fields.creditRateHint') }}
            </p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.resellerPlans.fields.concurrencyBonus') }}</label>
            <input v-model.number="form.concurrency_bonus" type="number" min="0" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.resellerPlans.fields.rpmLimit') }}</label>
            <input v-model.number="form.rpm_limit" type="number" min="0" class="input" />
            <p class="mt-1 text-xs text-ink-tertiary">
              {{ t('admin.resellerPlans.fields.rpmLimitHint') }}
            </p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.resellerPlans.fields.maxDomains') }}</label>
            <input v-model.number="form.max_domains" type="number" min="0" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.resellerPlans.fields.validityDays') }}</label>
            <input v-model.number="form.validity_days" type="number" min="1" class="input" />
            <p class="mt-1 text-xs text-ink-tertiary">
              {{ t('admin.resellerPlans.fields.validityDaysHint') }}
            </p>
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.resellerPlans.fields.allowedGroups') }}</label>
          <p class="mb-2 text-xs text-ink-tertiary">
            {{ t('admin.resellerPlans.fields.allowedGroupsHint') }}
          </p>
          <div
            v-if="groups.length"
            class="max-h-48 space-y-1 overflow-y-auto rounded-lg p-2 ring-1 ring-gray-900/5 dark:ring-dark-700"
          >
            <label
              v-for="group in groups"
              :key="group.id"
              class="flex items-center gap-2 text-sm text-ink-secondary"
            >
              <input v-model="form.allowed_group_ids" type="checkbox" :value="group.id" />
              {{ group.name }}
            </label>
          </div>
          <p v-else class="text-xs text-ink-tertiary">
            {{ t('admin.resellerPlans.fields.noGroups') }}
          </p>
        </div>

        <div>
          <label class="input-label">{{ t('admin.resellerPlans.fields.enabled') }}</label>
          <label class="flex items-center gap-2 text-sm text-ink-secondary">
            <input v-model="form.enabled" type="checkbox" />
            {{ t('admin.resellerPlans.fields.enabledHint') }}
          </label>
        </div>

        <p v-if="editorError" class="text-sm text-danger">{{ editorError }}</p>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showEditor = false">
          {{ t('common.cancel') }}
        </button>
        <button type="submit" form="reseller-plan-form" class="btn btn-primary" :disabled="saving">
          {{ t('common.save') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { Column } from '@/components/common/types'
import type { AdminGroup, ResellerPlan, ResellerPlanRequest } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const plans = ref<ResellerPlan[]>([])
const groups = ref<AdminGroup[]>([])
const loading = ref(false)
const saving = ref(false)
const showEditor = ref(false)
const editingPlan = ref<ResellerPlan | null>(null)
const editorError = ref('')

const form = reactive<Required<ResellerPlanRequest>>({
  level: 1,
  name: '',
  price: 0,
  credit_rate: 1,
  concurrency_bonus: 0,
  rpm_limit: 60,
  max_domains: 1,
  validity_days: 30,
  allowed_group_ids: [],
  enabled: true
})

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.resellerPlans.fields.name') },
  { key: 'price', label: t('admin.resellerPlans.fields.price'), sortable: true },
  { key: 'credit_rate', label: t('admin.resellerPlans.fields.creditRate') },
  { key: 'concurrency_bonus', label: t('admin.resellerPlans.fields.concurrencyBonus') },
  { key: 'rpm_limit', label: t('admin.resellerPlans.fields.rpmLimit') },
  { key: 'max_domains', label: t('admin.resellerPlans.fields.maxDomains') },
  { key: 'validity_days', label: t('admin.resellerPlans.fields.validityDays') },
  { key: 'allowed_group_ids', label: t('admin.resellerPlans.fields.allowedGroups') },
  { key: 'enabled', label: t('admin.resellerPlans.fields.enabled') }
])

function formatAmount(value: number): string {
  return value.toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: 2 })
}

// A credit rate of 1.25 is the reseller getting 25% more face value than they
// paid, which is how the margin is actually discussed.
function marginPercent(rate: number): string {
  return Math.round((rate - 1) * 100).toString()
}

const groupNames = computed(() => new Map(groups.value.map((group) => [group.id, group.name])))

/** Names, not ids — an id column tells an admin nothing at a glance. */
function groupSummary(ids: number[]): string {
  if (!ids || ids.length === 0) return t('admin.resellerPlans.allGroups')
  return ids.map((id) => groupNames.value.get(id) ?? `#${id}`).join(', ')
}

async function load() {
  loading.value = true
  try {
    plans.value = await adminAPI.resellerPlans.list()
  } catch (error) {
    console.error('Error loading reseller plans:', error)
    appStore.showError(t('admin.resellerPlans.loadFailed'))
  } finally {
    loading.value = false
  }
}

// The group list only labels the allowed-groups column and fills the editor's
// checkboxes; failing to load it must not blank the table.
async function loadGroups() {
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch (error) {
    console.error('Error loading groups:', error)
  }
}

function openEdit(plan: ResellerPlan) {
  editingPlan.value = plan
  form.level = plan.level
  form.name = plan.name
  form.price = plan.price
  form.credit_rate = plan.credit_rate
  form.concurrency_bonus = plan.concurrency_bonus
  form.rpm_limit = plan.rpm_limit
  form.max_domains = plan.max_domains
  form.validity_days = plan.validity_days
  form.allowed_group_ids = [...(plan.allowed_group_ids ?? [])]
  form.enabled = plan.enabled
  editorError.value = ''
  showEditor.value = true
}

async function save() {
  const plan = editingPlan.value
  if (!plan) return

  saving.value = true
  editorError.value = ''
  try {
    await adminAPI.resellerPlans.update(plan.id, { ...form, allowed_group_ids: [...form.allowed_group_ids] })
    showEditor.value = false
    appStore.showSuccess(t('admin.resellerPlans.saved'))
    await load()
  } catch (error: any) {
    // The ladder check runs on the server and its message names which two
    // plans are out of order, so show that rather than a generic failure.
    editorError.value = error?.response?.data?.message || t('admin.resellerPlans.saveFailed')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  load()
  loadGroups()
})
</script>
