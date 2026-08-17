/**
 * Display currency — what a price is *shown* in, which is deliberately not the
 * same thing as the currency a payment gateway *collects*.
 *
 * Every amount this panel holds is USD: balances, plan prices, VIP spend
 * thresholds, order amounts. Converting is a rendering concern only — nothing
 * here ever feeds a stored value or a charge. The gateway conversion lives in
 * `views/user/PaymentView.vue` (mirroring the backend's
 * `calculateGatewayBaseAmount`) and is driven by the *provider's* currency, not
 * by this module.
 *
 * The currency follows the UI locale rather than the account or the IP, because
 * the locale is the one signal the reader controls directly: someone who
 * switches to English is asking for dollars, and honouring that is less
 * surprising than pinning currency to a profile field they cannot see.
 *
 * Rates arrive from `GET /settings/public` (`display_fx_rates`) as "units per 1
 * USD". A currency with no usable rate is simply absent from that table, and
 * every function here then falls back to USD — a price that is honestly in
 * dollars beats one that is silently wrong in dong.
 */
import { formatPaymentAmount, paymentCurrencyFractionDigits } from '@/components/payment/currency'
import { DEFAULT_LOCALE, type LocaleCode } from '@/i18n/locale'

export type DisplayCurrency = 'USD' | 'CNY' | 'VND'

/** The canonical unit: every stored amount is already in it, so its rate is 1. */
export const BASE_DISPLAY_CURRENCY: DisplayCurrency = 'USD'

export const LOCALE_DISPLAY_CURRENCY: Record<LocaleCode, DisplayCurrency> = {
  en: 'USD',
  zh: 'CNY',
  vi: 'VND',
}

/** Units of each display currency per 1 USD. Mirrors the backend payload. */
export type DisplayFXRates = Record<string, number>

/** BCP-47 tag for `Intl`, so grouping and the symbol match the rendered locale. */
const INTL_LOCALE: Record<LocaleCode, string> = {
  en: 'en-US',
  zh: 'zh-CN',
  vi: 'vi-VN',
}

/**
 * Takes a plain `string` rather than a `LocaleCode` so callers holding
 * `useI18n().locale` (typed `string`) can use it without narrowing at every
 * call site — the alternative is a ternary repeated next to every `Intl` call,
 * which is exactly how `'zh-CN' : 'en-US'` ended up hardcoded in two views.
 */
export function intlLocaleFor(locale: string): string {
  return INTL_LOCALE[locale as LocaleCode] ?? INTL_LOCALE[DEFAULT_LOCALE]
}

/** The currency a locale *wants*, before checking whether a rate exists for it. */
export function preferredDisplayCurrency(locale: LocaleCode): DisplayCurrency {
  return LOCALE_DISPLAY_CURRENCY[locale] ?? BASE_DISPLAY_CURRENCY
}

function usableRate(rates: DisplayFXRates | null | undefined, currency: string): number {
  const rate = rates?.[currency]
  return typeof rate === 'number' && Number.isFinite(rate) && rate > 0 ? rate : 0
}

/**
 * What this locale can actually be priced in right now.
 *
 * Resolution, not preference: an operator who never enabled the FX sync has no
 * VND rate stored, and a Vietnamese reader must then see dollars rather than a
 * number derived from a rate of zero.
 */
export function resolveDisplayCurrency(
  locale: LocaleCode,
  rates: DisplayFXRates | null | undefined,
): { currency: DisplayCurrency; rate: number; converted: boolean } {
  const preferred = preferredDisplayCurrency(locale)
  if (preferred === BASE_DISPLAY_CURRENCY) {
    return { currency: BASE_DISPLAY_CURRENCY, rate: 1, converted: false }
  }

  const rate = usableRate(rates, preferred)
  if (rate <= 0) {
    return { currency: BASE_DISPLAY_CURRENCY, rate: 1, converted: false }
  }

  return { currency: preferred, rate, converted: true }
}

/**
 * Convert a stored USD amount into the display currency, rounded to that
 * currency's minor unit.
 *
 * Rounding here rather than at render time is what keeps a total from
 * disagreeing with the sum of the rows it is made of: two unrounded dong
 * figures printed as whole numbers can add up to one more dong than their
 * printed total.
 */
export function convertFromUSD(amountUSD: number, rate: number, currency: string): number {
  if (!Number.isFinite(amountUSD)) {
    return 0
  }
  if (!Number.isFinite(rate) || rate <= 0) {
    return amountUSD
  }

  const digits = paymentCurrencyFractionDigits(currency)
  const factor = 10 ** digits
  return Math.round(amountUSD * rate * factor) / factor
}

/**
 * Render a stored USD amount in the locale's display currency.
 *
 * Falls back to a plain USD rendering whenever no rate is available, so a
 * missing feed degrades to "correct but in dollars" rather than to a wrong
 * number or an empty cell.
 */
export function formatDisplayAmount(
  amountUSD: number,
  locale: LocaleCode,
  rates: DisplayFXRates | null | undefined,
): string {
  const { currency, rate } = resolveDisplayCurrency(locale, rates)
  return formatPaymentAmount(convertFromUSD(amountUSD, rate, currency), currency, intlLocaleFor(locale))
}
