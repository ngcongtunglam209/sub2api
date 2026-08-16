import { describe, expect, it } from 'vitest'

import {
  BASE_DISPLAY_CURRENCY,
  LOCALE_DISPLAY_CURRENCY,
  convertFromUSD,
  formatDisplayAmount,
  intlLocaleFor,
  preferredDisplayCurrency,
  resolveDisplayCurrency,
} from '@/utils/displayCurrency'

const RATES = { USD: 1, CNY: 7.1, VND: 26320 }

describe('locale to currency mapping', () => {
  it('gives every locale a currency', () => {
    expect(LOCALE_DISPLAY_CURRENCY).toEqual({ en: 'USD', zh: 'CNY', vi: 'VND' })
  })

  it.each([
    ['en', 'USD'],
    ['zh', 'CNY'],
    ['vi', 'VND'],
  ] as const)('%s prefers %s', (locale, currency) => {
    expect(preferredDisplayCurrency(locale)).toBe(currency)
  })

  it('formats through the matching BCP-47 tag so grouping matches the language', () => {
    expect(intlLocaleFor('vi')).toBe('vi-VN')
    expect(intlLocaleFor('zh')).toBe('zh-CN')
    expect(intlLocaleFor('en')).toBe('en-US')
  })
})

describe('resolveDisplayCurrency', () => {
  it('leaves USD unconverted — stored amounts are already dollars', () => {
    expect(resolveDisplayCurrency('en', RATES)).toEqual({
      currency: 'USD',
      rate: 1,
      converted: false,
    })
  })

  it('converts when a rate exists', () => {
    expect(resolveDisplayCurrency('vi', RATES)).toEqual({
      currency: 'VND',
      rate: 26320,
      converted: true,
    })
    expect(resolveDisplayCurrency('zh', RATES)).toEqual({
      currency: 'CNY',
      rate: 7.1,
      converted: true,
    })
  })

  // The operator may never have enabled the FX sync. Showing honest dollars then
  // beats showing a dong figure derived from a rate nobody supplied.
  it.each([
    ['missing table', undefined],
    ['null table', null],
    ['USD only', { USD: 1 }],
    ['zero rate', { USD: 1, VND: 0 }],
    ['negative rate', { USD: 1, VND: -26320 }],
    ['non-finite rate', { USD: 1, VND: Number.POSITIVE_INFINITY }],
    ['non-numeric rate', { USD: 1, VND: '26320' as unknown as number }],
  ])('falls back to USD when the VND rate is unusable (%s)', (_label, rates) => {
    expect(resolveDisplayCurrency('vi', rates as Record<string, number> | null | undefined)).toEqual({
      currency: 'USD',
      rate: 1,
      converted: false,
    })
  })

  it('keeps USD as the base currency constant', () => {
    expect(BASE_DISPLAY_CURRENCY).toBe('USD')
  })
})

describe('convertFromUSD', () => {
  // The dong has no minor unit, so a converted amount must be a whole number —
  // otherwise a rounded total disagrees with the sum of the rows above it.
  it('rounds to the currency minor unit', () => {
    expect(convertFromUSD(1.5, 26320, 'VND')).toBe(39480)
    expect(convertFromUSD(0.0001, 26320, 'VND')).toBe(3)
    expect(convertFromUSD(10, 7.12345, 'CNY')).toBe(71.23)
  })

  it('returns the dollar amount untouched when the rate is unusable', () => {
    expect(convertFromUSD(12.34, 0, 'VND')).toBe(12.34)
    expect(convertFromUSD(12.34, -1, 'VND')).toBe(12.34)
    expect(convertFromUSD(12.34, Number.NaN, 'VND')).toBe(12.34)
  })

  it('treats a non-finite amount as zero rather than propagating NaN into the UI', () => {
    expect(convertFromUSD(Number.NaN, 26320, 'VND')).toBe(0)
    expect(convertFromUSD(Number.POSITIVE_INFINITY, 26320, 'VND')).toBe(0)
  })

  it('is exact at 1 USD', () => {
    expect(convertFromUSD(1, 26320, 'VND')).toBe(26320)
    expect(convertFromUSD(1, 7.1, 'CNY')).toBe(7.1)
  })
})

describe('formatDisplayAmount', () => {
  it('renders dong with no decimal places', () => {
    const formatted = formatDisplayAmount(1.5, 'vi', RATES)
    expect(formatted).toContain('₫')
    expect(formatted).not.toMatch(/[.,]\d{2}\s*₫/)
    // 1.5 USD * 26320 = 39,480 dong, however the locale groups it.
    expect(formatted.replace(/\D/g, '')).toBe('39480')
  })

  it('renders yuan with two decimal places', () => {
    const formatted = formatDisplayAmount(10, 'zh', RATES)
    expect(formatted.replace(/\D/g, '')).toBe('7100')
  })

  it('renders plain dollars for en', () => {
    expect(formatDisplayAmount(12.34, 'en', RATES)).toContain('12.34')
  })

  // Falling back to USD changes the currency, not the language: a Vietnamese
  // reader still gets Vietnamese digit grouping, so "12,34 $" is the correct
  // rendering of $12.34 here and the assertion must not demand a dot.
  it('renders dollars for vi when no rate is configured', () => {
    const formatted = formatDisplayAmount(12.34, 'vi', { USD: 1 })
    expect(formatted).toContain('$')
    expect(formatted).not.toContain('₫')
    expect(formatted.replace(/\D/g, '')).toBe('1234')
  })
})
