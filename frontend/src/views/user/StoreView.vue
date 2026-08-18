<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <!--
        Balance first, because every price below is only meaningful against it.
        It is a stored USD figure that no gateway has touched, so the display
        conversion is safe to apply — see `useDisplayCurrency`.
      -->
      <Surface :title="t('store.title')" :description="t('store.description')">
        <Metric
          :label="t('store.balanceLabel')"
          :value="balanceText"
          data-testid="store-balance"
        />
      </Surface>

      <div v-if="loadingAddons" class="grid gap-6 sm:grid-cols-2">
        <Surface v-for="n in 2" :key="n">
          <div class="space-y-3">
            <div class="skeleton h-3 w-1/2"></div>
            <div class="skeleton h-3 w-2/3"></div>
            <div class="skeleton h-3 w-1/3"></div>
          </div>
        </Surface>
      </div>

      <div v-else-if="addonCards.length" class="grid gap-6 sm:grid-cols-2">
        <Surface
          v-for="card in addonCards"
          :key="card.kind"
          :title="card.title"
          :description="card.description"
        >
          <div class="space-y-4">
            <div class="grid gap-6 sm:grid-cols-2">
              <Metric
                :label="t('store.unitPriceLabel')"
                :value="card.unitPriceText"
                :caption="t('store.perUnitMonth')"
              />
              <Metric
                :label="t('store.heldLabel')"
                :value="card.held ? card.held.amount : null"
                :unit="card.unit"
                :caption="card.heldCaption"
                :data-testid="`store-${card.kind}-held`"
              />
            </div>

            <form class="space-y-4" @submit.prevent="handleAddonPurchase(card)">
              <div class="grid gap-4 sm:grid-cols-2">
                <FormField
                  :id="`store-${card.kind}-amount`"
                  :label="card.amountLabel"
                  :hint="card.amountHint"
                >
                  <input
                    :id="`store-${card.kind}-amount`"
                    v-model.number="forms[card.kind].amount"
                    type="number"
                    min="1"
                    step="1"
                    class="input"
                    :disabled="purchasingKind === card.kind"
                    :data-testid="`store-${card.kind}-amount-input`"
                  />
                </FormField>

                <FormField
                  :id="`store-${card.kind}-months`"
                  :label="t('store.monthsLabel')"
                  :hint="t('store.monthsHint')"
                >
                  <input
                    :id="`store-${card.kind}-months`"
                    v-model.number="forms[card.kind].months"
                    type="number"
                    min="1"
                    step="1"
                    class="input"
                    :disabled="purchasingKind === card.kind"
                    :data-testid="`store-${card.kind}-months-input`"
                  />
                </FormField>
              </div>

              <!--
                The cap bar counts what this purchase would take the user to,
                not what they hold now: a ceiling you only discover after the
                server rejects you is not a ceiling anyone can plan against.
              -->
              <Meter
                :label="t('store.capLabel')"
                :value="card.capAfter"
                :max="card.cap"
                format="ratio"
              />

              <div class="flex items-baseline justify-between gap-2 border-t border-line-subtle pt-3">
                <span class="text-xs text-ink-secondary">{{ t('store.totalLabel') }}</span>
                <span
                  class="font-mono text-sm tabular-nums text-ink"
                  :data-testid="`store-${card.kind}-total`"
                >
                  {{ card.totalText }}
                </span>
              </div>

              <p
                v-if="card.blockedReason"
                class="text-xs text-danger"
                :data-testid="`store-${card.kind}-blocked`"
              >
                {{ card.blockedReason }}
              </p>

              <div class="flex justify-end">
                <Button
                  type="submit"
                  size="md"
                  :loading="purchasingKind === card.kind"
                  :disabled="!card.canBuy"
                  :data-testid="`store-${card.kind}-buy`"
                >
                  {{ t('store.buy') }}
                </Button>
              </div>
            </form>
          </div>
        </Surface>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import addonsAPI from '@/api/addons'
import Button from '@/components/common/Button.vue'
import FormField from '@/components/common/FormField.vue'
import Meter from '@/components/common/Meter.vue'
import Metric from '@/components/common/Metric.vue'
import Surface from '@/components/common/Surface.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useDisplayCurrency } from '@/composables/useDisplayCurrency'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { formatDateTime } from '@/utils/format'
import { quoteAddon, type AddonBlockReason } from './storePricing'
import type {
  AddonHolding,
  AddonKind,
  AddonPricing,
  AddonsResponse
} from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { format: formatDisplay } = useDisplayCurrency()

const loadingAddons = ref(false)
const pricing = ref<AddonPricing | null>(null)
const held = ref<AddonsResponse['held']>({ concurrency: null, rpm: null })

const purchasingKind = ref<AddonKind | null>(null)

const forms = reactive<Record<AddonKind, { amount: number; months: number }>>({
  concurrency: { amount: 1, months: 1 },
  rpm: { amount: 1, months: 1 }
})

/**
 * `null` rather than `0` while the user has not loaded: `Metric` renders an en
 * dash for null and a real `0` for zero, so "not known yet" and "broke" stay
 * distinguishable instead of both reading as no money.
 */
const balance = computed<number | null>(() => authStore.user?.balance ?? null)

const balanceText = computed<string | null>(() =>
  balance.value === null ? null : formatDisplay(balance.value)
)

interface AddonCard {
  kind: AddonKind
  title: string
  description: string
  unit: string
  amountLabel: string
  amountHint: string
  held: AddonHolding | null
  heldCaption: string
  unitPriceText: string
  cap: number
  /** Capacity this user would hold once the pending purchase lands. */
  capAfter: number
  totalText: string
  blockedReason: string
  canBuy: boolean
}

interface AddonSpec {
  title: string
  description: string
  unit: string
  amountLabel: string
  amountHint: string
  unitPrice: number
  cap: number
  /** See `AddonQuoteInput.unitsPerPurchase` — RPM sells in `rpm_step` blocks. */
  unitsPerPurchase: number
}

/** The rules live in `storePricing`; only the wording is decided here. */
function blockedReasonText(reason: AddonBlockReason, cap: number): string {
  switch (reason) {
    case 'amount':
      return t('store.amountRequired')
    case 'months':
      return t('store.monthsRequired')
    case 'cap':
      return t('store.capExceeded', { cap })
    case 'balance':
      return t('store.insufficientBalance')
    default:
      return ''
  }
}

function buildAddonCard(kind: AddonKind, spec: AddonSpec): AddonCard {
  const form = forms[kind]
  const holding = held.value[kind]

  const quote = quoteAddon({
    unitPrice: spec.unitPrice,
    cap: spec.cap,
    unitsPerPurchase: spec.unitsPerPurchase,
    heldAmount: holding?.amount ?? 0,
    amount: form.amount,
    months: form.months,
    balance: balance.value
  })

  return {
    kind,
    title: spec.title,
    description: spec.description,
    unit: spec.unit,
    amountLabel: spec.amountLabel,
    amountHint: spec.amountHint,
    held: holding,
    heldCaption: holding
      ? t('store.heldUntil', { date: formatDateTime(holding.expires_at) })
      : t('store.heldNone'),
    unitPriceText: formatDisplay(spec.unitPrice),
    cap: spec.cap,
    capAfter: quote.capAfter,
    totalText: formatDisplay(quote.total),
    blockedReason: blockedReasonText(quote.blockedBy, spec.cap),
    canBuy: quote.canBuy
  }
}

const addonCards = computed<AddonCard[]>(() => {
  const prices = pricing.value
  if (!prices) return []

  return [
    buildAddonCard('concurrency', {
      title: t('store.concurrency.title'),
      description: t('store.concurrency.description'),
      unit: t('store.concurrency.unit'),
      amountLabel: t('store.concurrency.amountLabel'),
      amountHint: t('store.concurrency.amountHint'),
      unitPrice: prices.concurrency_unit_price,
      cap: prices.concurrency_cap,
      unitsPerPurchase: 1
    }),
    buildAddonCard('rpm', {
      title: t('store.rpm.title'),
      description: t('store.rpm.description'),
      unit: t('store.rpm.unit'),
      amountLabel: t('store.rpm.amountLabel', { step: prices.rpm_step }),
      amountHint: t('store.rpm.amountHint', { step: prices.rpm_step }),
      unitPrice: prices.rpm_unit_price,
      cap: prices.rpm_cap,
      unitsPerPurchase: prices.rpm_step
    })
  ]
})

async function loadAddons(): Promise<void> {
  loadingAddons.value = true
  try {
    const data = await addonsAPI.getAddons()
    pricing.value = data.pricing
    held.value = {
      concurrency: data.held?.concurrency ?? null,
      rpm: data.held?.rpm ?? null
    }
  } catch (error) {
    console.error('Failed to load add-ons:', error)
    appStore.showError(t('store.loadFailed'))
  } finally {
    loadingAddons.value = false
  }
}

/**
 * Re-read the balance from the server after money moved.
 *
 * Patching it locally would be guessing: the server applies the price, any
 * credit rebate, and whatever else the tier triggers, and only it knows the
 * result. A failure here leaves a stale number on screen, which is why it is
 * swallowed — reporting it as a purchase error would invite a second payment
 * for something that already succeeded.
 */
async function refreshBalance(): Promise<void> {
  try {
    await authStore.refreshUser()
  } catch (error) {
    console.error('Failed to refresh balance after purchase:', error)
  }
}

async function handleAddonPurchase(card: AddonCard): Promise<void> {
  if (!card.canBuy || purchasingKind.value !== null) return

  purchasingKind.value = card.kind
  try {
    await addonsAPI.purchaseAddon({
      kind: card.kind,
      amount: forms[card.kind].amount,
      months: forms[card.kind].months
    })
    appStore.showSuccess(t('store.purchaseSuccess'))
    await loadAddons()
    await refreshBalance()
  } catch (error: any) {
    appStore.showError(
      error?.response?.data?.message ||
        error?.response?.data?.detail ||
        t('store.purchaseFailed')
    )
  } finally {
    purchasingKind.value = null
  }
}

onMounted(() => {
  loadAddons()
})
</script>
