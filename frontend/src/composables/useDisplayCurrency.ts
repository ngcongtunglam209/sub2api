/**
 * Reactive access to the reader's display currency.
 *
 * Wraps `utils/displayCurrency` with the two pieces of live state it needs: the
 * active UI locale and the rate table from public settings. Keeping the maths in
 * the util and only the wiring here means the conversion rules stay testable
 * without mounting a component or standing up a store.
 *
 * Must be called from `setup()` — it reads `useI18n()` and a Pinia store. Call
 * `formatDisplayAmount` from the util directly if you need this outside a
 * component.
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'

import { useAppStore } from '@/stores/app'
import { isLocaleCode, DEFAULT_LOCALE, type LocaleCode } from '@/i18n/locale'
import {
  convertFromUSD,
  formatDisplayAmount,
  intlLocaleFor,
  resolveDisplayCurrency,
  type DisplayFXRates,
} from '@/utils/displayCurrency'
import { currencySymbol, paymentCurrencyFractionDigits } from '@/components/payment/currency'

export function useDisplayCurrency() {
  const { locale } = useI18n()
  const appStore = useAppStore()
  const { cachedPublicSettings } = storeToRefs(appStore)

  const localeCode = computed<LocaleCode>(() =>
    isLocaleCode(locale.value) ? locale.value : DEFAULT_LOCALE,
  )

  const rates = computed<DisplayFXRates>(
    () => cachedPublicSettings.value?.display_fx_rates ?? { USD: 1 },
  )

  const resolved = computed(() => resolveDisplayCurrency(localeCode.value, rates.value))

  return {
    /** ISO code actually in use — USD whenever the locale's currency has no rate. */
    currency: computed(() => resolved.value.currency),
    /** Units of `currency` per 1 USD. Exactly 1 when nothing is being converted. */
    rate: computed(() => resolved.value.rate),
    /** False when the reader is seeing raw stored dollars. */
    converted: computed(() => resolved.value.converted),
    symbol: computed(() => currencySymbol(resolved.value.currency)),
    fractionDigits: computed(() => paymentCurrencyFractionDigits(resolved.value.currency)),
    intlLocale: computed(() => intlLocaleFor(localeCode.value)),

    /** Stored USD -> a number in the display currency, rounded to its minor unit. */
    toDisplay: (amountUSD: number) =>
      convertFromUSD(amountUSD, resolved.value.rate, resolved.value.currency),
    /** Stored USD -> a fully formatted string with symbol and grouping. */
    format: (amountUSD: number) => formatDisplayAmount(amountUSD, localeCode.value, rates.value),
  }
}
