<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header
        class="page-header mb-0 rounded-3xl bg-surface p-5 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700 sm:p-6"
      >
        <h1 class="page-title flex items-center gap-2 text-xl font-black text-ink">
          {{ t('admin.resellerDomains.title') }}
        </h1>
        <p class="page-description mt-1.5 text-xs text-ink-secondary">
          {{ t('admin.resellerDomains.description') }}
        </p>
      </header>

      <div class="rounded-3xl bg-surface p-5 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700 sm:p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <span class="text-sm text-ink-secondary">{{ t('admin.resellerDomains.hint') }}</span>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="load">
              {{ t('common.refresh') }}
            </button>
            <button type="button" class="btn btn-primary" @click="openCreate">
              {{ t('admin.resellerDomains.create') }}
            </button>
          </div>
        </div>

        <DataTable :columns="columns" :data="domains" :loading="loading" row-key="id">
          <template #cell-domain="{ row }">
            <span class="font-mono text-sm text-ink">{{ row.domain }}</span>
          </template>

          <template #cell-user_id="{ row }">
            <span class="font-mono tabular-nums">#{{ row.user_id }}</span>
          </template>

          <template #cell-status="{ row }">
            <span :class="['badge', row.status === 'active' ? 'badge-success' : 'badge-danger']">
              {{ row.status === 'active' ? t('admin.resellerDomains.statusActive') : t('admin.resellerDomains.statusDisabled') }}
            </span>
          </template>

          <template #cell-notes="{ row }">
            <span class="text-xs text-ink-secondary">{{ row.notes || '—' }}</span>
          </template>

          <!--
            Which resellers are white-labelled is the question an operator asks
            first, so it is a column rather than something you learn by opening
            each row's dialog in turn.
          -->
          <template #cell-branding="{ row }">
            <span :class="['badge', hasBranding(row) ? 'badge-primary' : 'badge-gray']">
              {{
                hasBranding(row)
                  ? t('admin.resellerDomains.branding.overrideBadge')
                  : t('admin.resellerDomains.branding.inheritBadge')
              }}
            </span>
          </template>

          <template #cell-created_at="{ row }">
            <span class="whitespace-nowrap text-xs text-ink-tertiary">{{ formatDateTime(row.created_at) }}</span>
          </template>

          <template #actions="{ row }">
            <button type="button" class="btn btn-secondary btn-sm" @click="openBranding(row)">
              {{ t('admin.resellerDomains.branding.editAction') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="togglingId === row.id"
              @click="toggleStatus(row)"
            >
              {{
                row.status === 'active'
                  ? t('admin.resellerDomains.disableAction')
                  : t('admin.resellerDomains.enableAction')
              }}
            </button>
            <button type="button" class="btn btn-danger btn-sm" @click="deletingDomain = row">
              {{ t('common.delete') }}
            </button>
          </template>
        </DataTable>
      </div>
    </div>

    <BaseDialog
      :show="showEditor"
      :title="t('admin.resellerDomains.createTitle')"
      @close="showEditor = false"
    >
      <form id="reseller-domain-form" class="space-y-4" @submit.prevent="save">
        <div>
          <label class="input-label">{{ t('admin.resellerDomains.fields.domain') }}</label>
          <input
            v-model="form.domain"
            type="text"
            autocomplete="off"
            spellcheck="false"
            class="input font-mono"
            :placeholder="t('admin.resellerDomains.fields.domainPlaceholder')"
          />
          <p class="mt-1 text-xs text-ink-tertiary">
            {{ t('admin.resellerDomains.fields.domainHint') }}
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.resellerDomains.fields.userId') }}</label>
          <input v-model.number="form.user_id" type="number" min="1" class="input" />
          <p class="mt-1 text-xs text-ink-tertiary">
            {{ t('admin.resellerDomains.fields.userIdHint') }}
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.resellerDomains.fields.notes') }}</label>
          <input v-model="form.notes" type="text" class="input" />
        </div>
        <p v-if="editorError" class="text-sm text-danger">{{ editorError }}</p>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showEditor = false">
          {{ t('common.cancel') }}
        </button>
        <button type="submit" form="reseller-domain-form" class="btn btn-primary" :disabled="saving">
          {{ t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="!!brandingDomain"
      :title="t('admin.resellerDomains.branding.title', { domain: brandingDomain?.domain ?? '' })"
      @close="brandingDomain = null"
    >
      <form id="reseller-domain-branding-form" class="space-y-4" @submit.prevent="saveBranding">
        <p class="text-xs text-ink-secondary">
          {{ t('admin.resellerDomains.branding.intro') }}
        </p>

        <div>
          <div class="flex items-center justify-between gap-2">
            <label class="input-label">{{ t('admin.resellerDomains.branding.siteName') }}</label>
            <span v-if="!brandingForm.site_name.trim()" class="badge badge-gray">
              {{ t('admin.resellerDomains.branding.inheritBadge') }}
            </span>
          </div>
          <input
            v-model="brandingForm.site_name"
            type="text"
            class="input"
            :placeholder="t('admin.resellerDomains.branding.inheritPlaceholder')"
          />
          <p class="mt-1 text-xs text-ink-tertiary">
            {{ t('admin.resellerDomains.branding.siteNameHint') }}
          </p>
        </div>

        <div>
          <div class="flex items-center justify-between gap-2">
            <label class="input-label">{{ t('admin.resellerDomains.branding.siteSubtitle') }}</label>
            <span v-if="!brandingForm.site_subtitle.trim()" class="badge badge-gray">
              {{ t('admin.resellerDomains.branding.inheritBadge') }}
            </span>
          </div>
          <input
            v-model="brandingForm.site_subtitle"
            type="text"
            class="input"
            :placeholder="t('admin.resellerDomains.branding.inheritPlaceholder')"
          />
          <p class="mt-1 text-xs text-ink-tertiary">
            {{ t('admin.resellerDomains.branding.siteSubtitleHint') }}
          </p>
        </div>

        <div>
          <div class="flex items-center justify-between gap-2">
            <label class="input-label">{{ t('admin.resellerDomains.branding.siteLogo') }}</label>
            <span v-if="!brandingForm.site_logo.trim()" class="badge badge-gray">
              {{ t('admin.resellerDomains.branding.inheritBadge') }}
            </span>
          </div>
          <input
            v-model="brandingForm.site_logo"
            type="text"
            autocomplete="off"
            spellcheck="false"
            class="input font-mono"
            :placeholder="t('admin.resellerDomains.branding.siteLogoPlaceholder')"
          />
          <p class="mt-1 text-xs text-ink-tertiary">
            {{ t('admin.resellerDomains.branding.siteLogoHint') }}
          </p>

          <!--
            A logo that 404s is invisible to whoever typed it and obvious to the
            reseller's customers, so the URL gets checked here: rejected outright
            if it is not a usable image URL, and flagged if it fails to load.
          -->
          <div v-if="brandingForm.site_logo.trim()" class="mt-2">
            <p v-if="!logoPreviewUrl" class="text-xs text-danger">
              {{ t('admin.resellerDomains.branding.logoInvalid') }}
            </p>
            <div v-else class="flex items-center gap-3">
              <div
                class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-surface-sunken ring-1 ring-gray-900/5 dark:ring-dark-700"
              >
                <img
                  :src="logoPreviewUrl"
                  alt=""
                  class="max-h-full max-w-full object-contain"
                  @load="logoBroken = false"
                  @error="logoBroken = true"
                />
              </div>
              <div class="min-w-0">
                <p class="text-xs text-ink-tertiary">
                  {{ t('admin.resellerDomains.branding.logoPreview') }}
                </p>
                <p v-if="logoBroken" class="mt-0.5 text-xs text-warn">
                  {{ t('admin.resellerDomains.branding.logoBroken') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <p v-if="brandingError" class="text-sm text-danger">{{ brandingError }}</p>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="brandingDomain = null">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="reseller-domain-branding-form"
          class="btn btn-primary"
          :disabled="savingBranding"
        >
          {{ t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="!!deletingDomain"
      :title="t('admin.resellerDomains.deleteTitle')"
      :message="t('admin.resellerDomains.deleteMessage', { domain: deletingDomain?.domain ?? '' })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="deletingDomain = null"
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
import { formatDateTime } from '@/utils/format'
import { sanitizeUrl } from '@/utils/url'
import type { Column } from '@/components/common/types'
import type { CreateResellerDomainRequest, ResellerDomain } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const domains = ref<ResellerDomain[]>([])
const loading = ref(false)
const saving = ref(false)
const togglingId = ref<number | null>(null)
const showEditor = ref(false)
const deletingDomain = ref<ResellerDomain | null>(null)
const editorError = ref('')

const form = reactive<Required<CreateResellerDomainRequest>>({
  domain: '',
  user_id: 0,
  notes: ''
})

const brandingDomain = ref<ResellerDomain | null>(null)
const brandingError = ref('')
const savingBranding = ref(false)
const logoBroken = ref(false)

const brandingForm = reactive({
  site_name: '',
  site_logo: '',
  site_subtitle: ''
})

const columns = computed<Column[]>(() => [
  { key: 'domain', label: t('admin.resellerDomains.fields.domain') },
  { key: 'user_id', label: t('admin.resellerDomains.fields.userId'), sortable: true },
  { key: 'status', label: t('admin.resellerDomains.fields.status') },
  { key: 'branding', label: t('admin.resellerDomains.fields.branding') },
  { key: 'notes', label: t('admin.resellerDomains.fields.notes') },
  { key: 'created_at', label: t('admin.resellerDomains.fields.createdAt'), sortable: true }
])

/**
 * The API sends `null` and `''` interchangeably for "no override", so an
 * override only counts once something non-blank is actually stored.
 */
function hasBranding(domain: ResellerDomain): boolean {
  return [domain.site_name, domain.site_logo, domain.site_subtitle].some(
    (value) => (value ?? '').trim() !== ''
  )
}

/**
 * Empty when the field is blank OR the URL is one no browser would render.
 * Passing it through sanitizeUrl keeps a `javascript:` URL out of the preview's
 * `src` — the operator pastes this, and it later ships to a reseller's visitors.
 */
const logoPreviewUrl = computed(() =>
  sanitizeUrl(brandingForm.site_logo, { allowRelative: true, allowDataUrl: true })
)

async function load() {
  loading.value = true
  try {
    domains.value = await adminAPI.resellerDomains.list()
  } catch (error) {
    console.error('Error loading reseller domains:', error)
    appStore.showError(t('admin.resellerDomains.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.domain = ''
  form.user_id = 0
  form.notes = ''
  editorError.value = ''
  showEditor.value = true
}

async function save() {
  if (!form.domain.trim()) {
    editorError.value = t('admin.resellerDomains.domainRequired')
    return
  }
  if (!form.user_id || form.user_id < 1) {
    editorError.value = t('admin.resellerDomains.userRequired')
    return
  }

  saving.value = true
  editorError.value = ''
  try {
    await adminAPI.resellerDomains.create({
      domain: form.domain.trim(),
      user_id: form.user_id,
      notes: form.notes.trim()
    })
    showEditor.value = false
    appStore.showSuccess(t('admin.resellerDomains.saved'))
    await load()
  } catch (error: any) {
    // Duplicate domains and unknown user ids are both rejected server-side with
    // a message that names the offending value, which beats a generic failure.
    editorError.value = error?.response?.data?.message || t('admin.resellerDomains.saveFailed')
  } finally {
    saving.value = false
  }
}

function openBranding(domain: ResellerDomain) {
  brandingDomain.value = domain
  brandingForm.site_name = domain.site_name ?? ''
  brandingForm.site_logo = domain.site_logo ?? ''
  brandingForm.site_subtitle = domain.site_subtitle ?? ''
  brandingError.value = ''
  logoBroken.value = false
}

/**
 * Every field goes over the wire, blank ones included: a blank field is the
 * operator clearing the override, and the API reads `''` as exactly that.
 */
async function saveBranding() {
  const domain = brandingDomain.value
  if (!domain) return

  savingBranding.value = true
  brandingError.value = ''
  try {
    await adminAPI.resellerDomains.update(domain.id, {
      site_name: brandingForm.site_name.trim(),
      site_logo: brandingForm.site_logo.trim(),
      site_subtitle: brandingForm.site_subtitle.trim()
    })
    brandingDomain.value = null
    appStore.showSuccess(t('admin.resellerDomains.branding.saved'))
    await load()
  } catch (error: any) {
    brandingError.value =
      error?.response?.data?.message || t('admin.resellerDomains.branding.saveFailed')
  } finally {
    savingBranding.value = false
  }
}

/**
 * Disabling is the reversible half of this page: the row keeps its notes and
 * its history, so it needs no confirmation. Deleting does.
 */
async function toggleStatus(domain: ResellerDomain) {
  const next = domain.status === 'active' ? 'disabled' : 'active'
  togglingId.value = domain.id
  try {
    await adminAPI.resellerDomains.setStatus(domain.id, next)
    appStore.showSuccess(
      next === 'active'
        ? t('admin.resellerDomains.enabled')
        : t('admin.resellerDomains.disabled')
    )
    await load()
  } catch (error) {
    console.error('Error updating reseller domain status:', error)
    appStore.showError(t('admin.resellerDomains.statusFailed'))
  } finally {
    togglingId.value = null
  }
}

async function confirmDelete() {
  const domain = deletingDomain.value
  deletingDomain.value = null
  if (!domain) return
  try {
    await adminAPI.resellerDomains.delete(domain.id)
    appStore.showSuccess(t('admin.resellerDomains.deleted'))
    await load()
  } catch (error) {
    console.error('Error deleting reseller domain:', error)
    appStore.showError(t('admin.resellerDomains.deleteFailed'))
  }
}

onMounted(load)
</script>
