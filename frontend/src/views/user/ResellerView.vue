<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <!-- Current plan, or the reason there is nothing else on this page. -->
      <Surface :title="t('reseller.planTitle')">
        <div v-if="loadingPlan" class="space-y-3">
          <div class="skeleton h-3 w-1/3"></div>
          <div class="skeleton h-3 w-2/3"></div>
        </div>

        <template v-else-if="plan">
          <div class="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
            <Metric :label="t('reseller.planName')" :value="plan.name" />
            <Metric
              :label="t('reseller.creditRate')"
              :value="plan.credit_rate"
              :precision="2"
              :caption="t('reseller.creditRateCaption')"
            />
            <Metric
              :label="t('reseller.rpmLimit')"
              :value="plan.rpm_limit"
              :unit="t('reseller.perMinute')"
            />
            <Metric :label="t('reseller.maxDomains')" :value="plan.max_domains" />
          </div>
          <p class="mt-4 text-xs text-ink-secondary">
            <template v-if="assignment?.expires_at">
              {{ t('reseller.expiresOn', { date: formatDateTime(assignment.expires_at) }) }}
            </template>
            <template v-else>{{ t('reseller.noExpiry') }}</template>
          </p>
        </template>

        <EmptyState
          v-else
          :title="t('reseller.noPlanTitle')"
          :description="t('reseller.noPlanDescription')"
        />
      </Surface>

      <template v-if="plan">
        <Surface :title="t('reseller.generateTitle')" :description="t('reseller.generateDescription')">
          <form class="space-y-4" @submit.prevent="handleGenerate">
            <div class="grid gap-4 sm:grid-cols-2">
              <FormField
                id="reseller-count"
                :label="t('reseller.fields.count')"
                :hint="t('reseller.fields.countHint')"
              >
                <input
                  id="reseller-count"
                  v-model.number="form.count"
                  type="number"
                  min="1"
                  max="500"
                  class="input"
                  :disabled="generating"
                  data-testid="reseller-count-input"
                />
              </FormField>

              <FormField
                id="reseller-value"
                :label="t('reseller.fields.value')"
                :hint="t('reseller.fields.valueHint')"
              >
                <input
                  id="reseller-value"
                  v-model.number="form.value"
                  type="number"
                  min="0"
                  step="0.01"
                  class="input"
                  :disabled="generating"
                  data-testid="reseller-value-input"
                />
              </FormField>

              <FormField
                id="reseller-group"
                :label="t('reseller.fields.group')"
                :hint="t('reseller.fields.groupHint')"
                optional
              >
                <select
                  id="reseller-group"
                  v-model="form.group_id"
                  class="input"
                  :disabled="generating"
                  data-testid="reseller-group-select"
                >
                  <option :value="null">{{ t('reseller.fields.noGroup') }}</option>
                  <option v-for="group in allowedGroups" :key="group.id" :value="group.id">
                    {{ group.name }}
                  </option>
                </select>
              </FormField>

              <FormField
                id="reseller-notes"
                :label="t('reseller.fields.notes')"
                :hint="t('reseller.fields.notesHint')"
                optional
              >
                <input
                  id="reseller-notes"
                  v-model="form.notes"
                  type="text"
                  class="input"
                  :disabled="generating"
                />
              </FormField>
            </div>

            <p v-if="generateError" class="text-xs text-danger">{{ generateError }}</p>

            <div class="flex justify-end">
              <Button
                type="submit"
                tone="accent"
                variant="solid"
                size="md"
                :loading="generating"
                data-testid="reseller-generate-submit"
              >
                {{ t('reseller.generateButton') }}
              </Button>
            </div>
          </form>
        </Surface>

        <!--
          The API returns the code strings exactly once and nothing re-fetches
          them, so this panel is the reseller's only chance to take them away.
          It stays on screen until the next generation replaces it, and the
          copy-all action yields the whole batch rather than one row at a time.
        -->
        <Surface
          v-if="mintedCodes.length"
          :title="t('reseller.mintedTitle')"
          :description="t('reseller.mintedWarning')"
        >
          <template #actions>
            <Button size="sm" data-testid="reseller-copy-all" @click="copyAll">
              {{ t('reseller.copyAll') }}
            </Button>
          </template>
          <pre
            class="max-h-64 overflow-auto rounded bg-surface-sunken p-3 font-mono text-xs text-ink"
            data-testid="reseller-minted-codes"
          >{{ mintedText }}</pre>
        </Surface>

        <Surface :title="t('reseller.codesTitle')" flush>
          <div v-if="loadingCodes" class="space-y-3 p-4">
            <div class="skeleton h-3 w-full"></div>
            <div class="skeleton h-3 w-4/5"></div>
            <div class="skeleton h-3 w-2/3"></div>
          </div>

          <div v-else-if="codes.length" class="overflow-x-auto">
            <table class="table min-w-[40rem]" data-testid="reseller-codes-table">
              <thead>
                <tr>
                  <th scope="col">{{ t('reseller.columns.code') }}</th>
                  <th scope="col" class="is-numeric">{{ t('reseller.columns.value') }}</th>
                  <th scope="col">{{ t('reseller.columns.status') }}</th>
                  <th scope="col" class="is-numeric">{{ t('reseller.columns.createdAt') }}</th>
                  <th scope="col" class="is-numeric">{{ t('reseller.columns.usedAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in codes" :key="item.code">
                  <td class="whitespace-nowrap font-mono text-xs text-ink">{{ item.code }}</td>
                  <td class="is-numeric">
                    <NumCell :value="item.value" :precision="2" unit="USD" />
                  </td>
                  <td>
                    <span class="text-xs text-ink-secondary">{{ statusLabel(item.status) }}</span>
                  </td>
                  <td class="is-numeric whitespace-nowrap">{{ formatDateTime(item.created_at) }}</td>
                  <td class="is-numeric whitespace-nowrap">
                    {{ item.used_at ? formatDateTime(item.used_at) : '—' }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <p v-else class="px-4 py-8 text-center text-xs text-ink-tertiary">
            {{ t('reseller.noCodes') }}
          </p>

          <template #footer>
            <Pagination
              v-model:page="page"
              v-model:page-size="pageSize"
              :total="total"
              @update:page="loadCodes"
              @update:page-size="onPageSizeChange"
            />
          </template>
        </Surface>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { userGroupsAPI } from '@/api'
import resellerAPI from '@/api/reseller'
import Button from '@/components/common/Button.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import FormField from '@/components/common/FormField.vue'
import Metric from '@/components/common/Metric.vue'
import NumCell from '@/components/common/NumCell.vue'
import Pagination from '@/components/common/Pagination.vue'
import Surface from '@/components/common/Surface.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import type { Group, ResellerAssignment, ResellerCode } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const assignment = ref<ResellerAssignment | null>(null)
const loadingPlan = ref(false)

const groups = ref<Group[]>([])

const codes = ref<ResellerCode[]>([])
const loadingCodes = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const generating = ref(false)
const generateError = ref('')
const mintedCodes = ref<Array<{ code: string; value: number }>>([])

const form = reactive<{
  count: number
  value: number
  group_id: number | null
  notes: string
}>({
  count: 10,
  value: 10,
  group_id: null,
  notes: ''
})

const plan = computed(() => assignment.value?.plan ?? null)

/**
 * A plan with no allowed groups is unrestricted, which is why an empty list
 * means "all", not "none". Filtering to an empty select would silently make the
 * group option unusable for exactly those plans.
 */
const allowedGroups = computed(() => {
  const allowed = plan.value?.allowed_group_ids ?? []
  if (allowed.length === 0) return groups.value
  return groups.value.filter((group) => allowed.includes(group.id))
})

/** One code per line — the shape a reseller pastes into a spreadsheet. */
const mintedText = computed(() => mintedCodes.value.map((item) => item.code).join('\n'))

function statusLabel(status: string): string {
  if (status === 'used') return t('reseller.status.used')
  if (status === 'expired') return t('reseller.status.expired')
  if (status === 'disabled') return t('reseller.status.disabled')
  return t('reseller.status.unused')
}

async function loadPlan() {
  loadingPlan.value = true
  try {
    assignment.value = await resellerAPI.getPlan()
  } catch (error) {
    console.error('Failed to load reseller plan:', error)
    appStore.showError(t('reseller.planLoadFailed'))
  } finally {
    loadingPlan.value = false
  }
}

async function loadCodes() {
  loadingCodes.value = true
  try {
    const result = await resellerAPI.getCodes(page.value, pageSize.value)
    codes.value = result.items
    total.value = result.total
  } catch (error) {
    console.error('Failed to load reseller codes:', error)
    appStore.showError(t('reseller.codesLoadFailed'))
  } finally {
    loadingCodes.value = false
  }
}

function onPageSizeChange() {
  page.value = 1
  loadCodes()
}

// The group select only narrows an optional field; failing to load it must not
// take the generation form down with it.
async function loadGroups() {
  try {
    groups.value = await userGroupsAPI.getAvailable()
  } catch (error) {
    console.error('Failed to load groups:', error)
  }
}

async function handleGenerate() {
  generateError.value = ''

  if (!form.count || form.count < 1) {
    generateError.value = t('reseller.countRequired')
    return
  }
  if (form.value === null || form.value === undefined || form.value <= 0) {
    generateError.value = t('reseller.valueRequired')
    return
  }

  generating.value = true
  try {
    const result = await resellerAPI.generateCodes({
      count: form.count,
      value: form.value,
      ...(form.group_id ? { group_id: form.group_id } : {}),
      ...(form.notes.trim() ? { notes: form.notes.trim() } : {})
    })
    mintedCodes.value = result.codes
    appStore.showSuccess(t('reseller.generateSuccess', { count: result.count }))
    page.value = 1
    await loadCodes()
  } catch (error: any) {
    generateError.value =
      error?.response?.data?.message ||
      error?.response?.data?.detail ||
      t('reseller.generateFailed')
  } finally {
    generating.value = false
  }
}

async function copyAll() {
  await copyToClipboard(mintedText.value, t('reseller.copiedAll', { count: mintedCodes.value.length }))
}

onMounted(async () => {
  await loadPlan()
  if (plan.value) {
    loadGroups()
    loadCodes()
  }
})
</script>
