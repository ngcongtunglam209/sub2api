<template>
  <!--
    A plan card is the single most slop-prone surface in the product. What this
    deliberately does NOT have, and must not regain: a gradient ground, a glow
    ring on the "popular" plan, a pill badge, a hover lift, a stacked shadow, or
    a giant gradient-filled price.

    What carries the design instead: one hairline, a `sm`/medium plan name, the
    price in mono tabular figures with its symbol and ISO code a full step down
    so they never compete with the digits, and a spec list rendered as hairline
    rows rather than a column of green ticks. A plan the user already holds is
    marked by a 1px accent border plus a typographic eyebrow — never by a tinted
    ground.
  -->
  <article
    :class="[
      'flex min-w-0 flex-col rounded border bg-surface',
      isRenewal ? 'border-accent' : 'border-line',
    ]"
  >
    <div class="flex items-start justify-between gap-3 p-4">
      <div class="min-w-0 flex-1">
        <!--
          A <span>, not a <p>: the description below is the first paragraph in
          this card and downstream specs address it as such.
        -->
        <span
          v-if="isRenewal"
          class="mb-1 block text-2xs font-medium uppercase tracking-[0.04em] text-accent"
        >
          {{ t('payment.activeSubscription') }}
        </span>
        <h3
          :title="plan.name"
          class="line-clamp-2 h-9 min-w-0 break-words text-sm font-medium text-ink [overflow-wrap:anywhere]"
        >{{ plan.name }}</h3>
        <p v-if="plan.description" class="line-clamp-2 mt-1 text-xs text-ink-tertiary">
          {{ plan.description }}
        </p>
      </div>

      <div class="shrink-0 text-right">
        <div class="flex items-baseline justify-end gap-0.5 font-mono tabular-nums">
          <span class="text-2xs text-ink-tertiary">{{ priceSymbol }}</span>
          <span class="text-xl font-semibold text-ink">{{ priceText }}</span>
          <span v-if="priceCode" class="text-2xs font-normal text-ink-tertiary">{{ priceCode }}</span>
        </div>
        <div class="mt-1 flex items-center justify-end gap-1.5">
          <Badge caps class="shrink-0">{{ pLabel }}</Badge>
          <span class="text-2xs text-ink-tertiary">/ {{ validitySuffix }}</span>
        </div>
        <div v-if="plan.original_price" class="mt-1 flex items-center justify-end gap-1.5">
          <span class="font-mono text-2xs tabular-nums text-ink-disabled line-through">{{ priceSymbol }}{{ originalPriceText }}<template v-if="priceCode"> {{ priceCode }}</template></span>
          <span
            v-if="discountText"
            class="rounded-sm border border-line px-1 text-2xs font-medium text-ink-secondary"
          >{{ discountText }}</span>
        </div>
      </div>
    </div>

    <!-- Spec rows. Hairlines, not a sunken tinted well inside a card. -->
    <dl class="divide-y divide-line-subtle border-t border-line-subtle text-xs">
      <div class="flex items-baseline justify-between gap-3 px-4 py-1.5">
        <dt class="shrink-0 text-ink-tertiary">{{ t('payment.planCard.rate') }}</dt>
        <dd class="font-mono tabular-nums text-ink">{{ rateDisplay }}</dd>
      </div>
      <div v-if="hasPeakRate" class="flex items-baseline justify-between gap-3 px-4 py-1.5">
        <dt class="shrink-0 text-ink-tertiary">{{ t('payment.planCard.peakRate') }}</dt>
        <dd class="text-right font-mono tabular-nums text-ink-secondary">{{ peakRateDisplay }}</dd>
      </div>
      <div
        v-if="plan.daily_limit_usd != null"
        class="flex items-baseline justify-between gap-3 px-4 py-1.5"
      >
        <dt class="shrink-0 text-ink-tertiary">{{ t('payment.planCard.dailyLimit') }}</dt>
        <dd><NumCell :value="plan.daily_limit_usd" :precision="2" unit="USD" /></dd>
      </div>
      <div
        v-if="plan.weekly_limit_usd != null"
        class="flex items-baseline justify-between gap-3 px-4 py-1.5"
      >
        <dt class="shrink-0 text-ink-tertiary">{{ t('payment.planCard.weeklyLimit') }}</dt>
        <dd><NumCell :value="plan.weekly_limit_usd" :precision="2" unit="USD" /></dd>
      </div>
      <div
        v-if="plan.monthly_limit_usd != null"
        class="flex items-baseline justify-between gap-3 px-4 py-1.5"
      >
        <dt class="shrink-0 text-ink-tertiary">{{ t('payment.planCard.monthlyLimit') }}</dt>
        <dd><NumCell :value="plan.monthly_limit_usd" :precision="2" unit="USD" /></dd>
      </div>
      <div v-if="hasNoLimit" class="flex items-baseline justify-between gap-3 px-4 py-1.5">
        <dt class="shrink-0 text-ink-tertiary">{{ t('payment.planCard.quota') }}</dt>
        <dd class="text-ink">{{ t('payment.planCard.unlimited') }}</dd>
      </div>
      <div
        v-if="modelScopeLabels.length > 0"
        class="flex items-baseline justify-between gap-3 px-4 py-1.5"
      >
        <dt class="shrink-0 text-ink-tertiary">{{ t('payment.planCard.models') }}</dt>
        <dd class="flex flex-wrap justify-end gap-1">
          <Badge v-for="scope in modelScopeLabels" :key="scope">{{ scope }}</Badge>
        </dd>
      </div>
    </dl>

    <!-- Features. Hairline rows — a tick column adds a colour and no information. -->
    <ul
      v-if="plan.features.length > 0"
      class="divide-y divide-line-subtle border-t border-line-subtle"
    >
      <li
        v-for="feature in plan.features"
        :key="feature"
        class="px-4 py-1.5 text-xs text-ink-secondary"
      >
        {{ feature }}
      </li>
    </ul>

    <div class="flex-1" />

    <div class="border-t border-line-subtle p-4">
      <Button tone="accent" variant="outline" size="md" block @click="emit('select', plan)">
        {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
      </Button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import Badge from '@/components/common/Badge.vue'
import Button from '@/components/common/Button.vue'
import NumCell from '@/components/common/NumCell.vue'
import { currencySymbol } from '@/components/payment/currency'
import { useDisplayCurrency } from '@/composables/useDisplayCurrency'
import { useAppStore } from '@/stores/app'
import type { UserSubscription } from '@/types'
import type { SubscriptionPlan } from '@/types/payment'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformLabel } from '@/utils/platformColors'

import { planValiditySuffix } from './validity'

const props = defineProps<{ plan: SubscriptionPlan; activeSubscriptions?: UserSubscription[] }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

/*
 * The per-platform colour ramps are gone from this card. They were the only
 * reason it carried six different accent hues, they came with hardcoded `dark:`
 * pairs, and a platform is a CATEGORY — the label already says which one it is.
 * Only the label survives.
 */
const platform = computed(() => props.plan.group_platform || '')
const pLabel = computed(() => platformLabel(platform.value))

const isRenewal = computed(() =>
  props.activeSubscriptions?.some(s => s.group_id === props.plan.group_id && s.status === 'active') ?? false
)

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
  const rate = props.plan.rate_multiplier ?? 1
  return `×${Number(rate.toPrecision(10))}`
})

const hasNoLimit = computed(
  () =>
    props.plan.daily_limit_usd == null &&
    props.plan.weekly_limit_usd == null &&
    props.plan.monthly_limit_usd == null
)

const appStore = useAppStore()
const planCurrencySymbol = computed(() => currencySymbol(props.plan.currency || 'USD'))

/*
 * `plan.price` and `plan.original_price` are stored USD; `plan.currency` is only
 * a LABEL an operator typed. So once the reader's locale resolves to a currency
 * we hold a rate for, that label is not merely redundant, it is wrong — the
 * digits on screen are no longer dollars. The display currency then owns all
 * three spans (symbol, number, ISO code) and the operator label is dropped.
 *
 * With no usable rate `converted` is false and every branch below collapses to
 * exactly what this card rendered before: the operator's symbol, the raw stored
 * number, and the operator's label shown only when non-empty.
 */
const display = useDisplayCurrency()

function formatDisplayNumber(amountUSD: number): string {
  const digits = display.fractionDigits.value
  try {
    return new Intl.NumberFormat(display.intlLocale.value, {
      minimumFractionDigits: digits,
      maximumFractionDigits: digits,
    }).format(display.toDisplay(amountUSD))
  } catch {
    return display.toDisplay(amountUSD).toFixed(digits)
  }
}

const priceSymbol = computed(() =>
  display.converted.value ? display.symbol.value : planCurrencySymbol.value
)

/** Empty renders no code span at all — the `v-if` that guarded `plan.currency`. */
const priceCode = computed(() =>
  display.converted.value ? display.currency.value : props.plan.currency || ''
)

const priceText = computed(() =>
  display.converted.value ? formatDisplayNumber(props.plan.price) : String(props.plan.price)
)

const originalPriceText = computed(() => {
  const original = props.plan.original_price
  if (original == null) return ''
  return display.converted.value ? formatDisplayNumber(original) : String(original)
})

const hasPeakRate = computed(() => groupHasPeakRate(props.plan))

const peakRateDisplay = computed(() => {
  return formatPeakRateWindow(props.plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

const modelScopeLabels = computed(() => {
  if (platform.value !== 'antigravity') return []
  const scopes = props.plan.supported_model_scopes
  if (!scopes || scopes.length === 0) return []
  return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const validitySuffix = computed(() => planValiditySuffix(props.plan, t))
</script>
