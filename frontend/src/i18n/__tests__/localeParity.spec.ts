import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import vi from '../locales/vi'
import zh from '../locales/zh'

/**
 * Key parity of every locale against `en`.
 *
 * The repo already guards two i18n properties — top-level collisions
 * (`localesNoKeyCollision`) and message compilability (`localesMessageCompile`)
 * — but nothing checked that the locales describe the same key set. `en` is the
 * reference here: every other locale is diffed against it, so adding a fourth
 * locale means one entry in `BUNDLES` below rather than a new spec. A key
 * present in `en` and absent in `zh` does not throw: vue-i18n renders the key
 * path itself, so a Chinese user sees `admin.accounts.form.priorityHint` where
 * a sentence should be.
 *
 * This matters specifically because of the design-system migration. Every
 * rewritten view is an opportunity to add copy, and "zh to follow" is how a
 * translation backlog of several thousand keys gets created one PR at a time.
 * With this spec in place, adding an `en` key without its `zh` counterpart
 * fails immediately, in the PR that did it.
 *
 * Any existing asymmetry is grandfathered rather than fixed here: it predates
 * this work, some of it may be intentional (locale-specific legal copy), and
 * bundling a translation pass into a styling change would make both harder to
 * review. What the allowlist guarantees is that the number only goes down.
 */

type Json = Record<string, unknown>

/** Flatten to dotted leaf paths. Arrays are leaves — order/length is content. */
function flatten(value: unknown, prefix = '', out: Set<string> = new Set()): Set<string> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    out.add(prefix)
    return out
  }
  for (const [key, child] of Object.entries(value as Json)) {
    flatten(child, prefix ? `${prefix}.${key}` : key, out)
  }
  return out
}

/** Every shipped locale. `en` is the reference the others are diffed against. */
const BUNDLES = { en, zh, vi } as const

type Locale = keyof typeof BUNDLES

const REFERENCE: Locale = 'en'

const KEYS = Object.fromEntries(
  Object.entries(BUNDLES).map(([locale, messages]) => [locale, flatten(messages)])
) as Record<Locale, Set<string>>

const enKeys = KEYS[REFERENCE]

const TRANSLATIONS = (Object.keys(BUNDLES) as Locale[]).filter((locale) => locale !== REFERENCE)

/**
 * Pre-existing asymmetry, measured at the end of Tier 0. Shrink this; never
 * grow it. Each entry is a key that exists in exactly one locale.
 *
 * `GRANDFATHERED_ONLY_EN` holds keys the named locale is allowed to be missing;
 * `GRANDFATHERED_ONLY_TRANSLATION` holds keys it is allowed to have on its own.
 */
const GRANDFATHERED_ONLY_EN: Record<Exclude<Locale, 'en'>, string[]> = {
  zh: [],
  vi: []
}
const GRANDFATHERED_ONLY_TRANSLATION: Record<Exclude<Locale, 'en'>, string[]> = {
  zh: [],
  vi: []
}

describe('i18n: locale parity against en', () => {
  it('loads every locale', () => {
    for (const locale of Object.keys(BUNDLES) as Locale[]) {
      expect(KEYS[locale].size, `${locale} bundle looks empty`).toBeGreaterThan(1000)
    }
  })
})

describe.each(TRANSLATIONS)('i18n: en/%s parity', (locale) => {
  const localeKeys = KEYS[locale]
  const onlyEn = [...enKeys].filter((k) => !localeKeys.has(k)).sort()
  const onlyLocale = [...localeKeys].filter((k) => !enKeys.has(k)).sort()

  it(`has no en key missing from ${locale}`, () => {
    const allowed = new Set(GRANDFATHERED_ONLY_EN[locale])
    expect(
      onlyEn.filter((k) => !allowed.has(k)),
      `add the ${locale} translation in the same commit — a missing key renders as its own path`
    ).toEqual([])
  })

  it(`has no ${locale} key missing from en`, () => {
    const allowed = new Set(GRANDFATHERED_ONLY_TRANSLATION[locale])
    expect(onlyLocale.filter((k) => !allowed.has(k)), 'add the en translation').toEqual([])
  })

  it('keeps the grandfathered lists honest', () => {
    const enFixed = GRANDFATHERED_ONLY_EN[locale]
      .filter((k) => localeKeys.has(k) || !enKeys.has(k))
      .sort()
    const localeFixed = GRANDFATHERED_ONLY_TRANSLATION[locale]
      .filter((k) => enKeys.has(k) || !localeKeys.has(k))
      .sort()
    expect(enFixed, `no longer asymmetric — remove from GRANDFATHERED_ONLY_EN.${locale}`).toEqual([])
    expect(
      localeFixed,
      `no longer asymmetric — remove from GRANDFATHERED_ONLY_TRANSLATION.${locale}`
    ).toEqual([])
  })
})
