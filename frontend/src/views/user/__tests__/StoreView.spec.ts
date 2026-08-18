import { describe, expect, it } from 'vitest'

import { canAfford, quoteAddon } from '../storePricing'

/**
 * The add-on store spends real balance, so these tests pin the arithmetic that
 * decides whether a user is charged — not the markup around it.
 *
 * The dangerous asymmetry throughout: RPM is *sold* in `rpm_step` blocks but
 * *capped* in single requests per minute. Every unscaled comparison between the
 * two silently sells capacity the operator never meant to sell.
 */

/** Concurrency: sold and capped in the same unit. */
const CONCURRENCY = {
  unitPrice: 1,
  cap: 10,
  unitsPerPurchase: 1,
  heldAmount: 0,
  amount: 1,
  months: 1,
  balance: 1000
}

/** RPM: sold in blocks of 30, capped at 600 single req/min. */
const RPM = {
  unitPrice: 2,
  cap: 600,
  unitsPerPurchase: 30,
  heldAmount: 0,
  amount: 1,
  months: 1,
  balance: 1000
}

describe('quoteAddon — RPM cap scaling', () => {
  it('refuses a purchase that only fits when blocks are compared to a req/min cap', () => {
    // Holding 540 of 600 req/min, buying 3 blocks = 90 req/min -> 630, over cap.
    // The naive unscaled check (540 + 3 = 543) would wave this through and sell
    // 30 req/min more than the operator's ceiling allows.
    const quote = quoteAddon({ ...RPM, heldAmount: 540, amount: 3 })

    expect(quote.capAfter).toBe(630)
    expect(quote.blockedBy).toBe('cap')
    expect(quote.canBuy).toBe(false)
  })

  it('allows a purchase that only fails when the existing holding is scaled twice', () => {
    // `heldAmount` is already native req/min. Scaling it by the step again
    // (540 * 30) would refuse a purchase that lands comfortably under the cap.
    const quote = quoteAddon({ ...RPM, heldAmount: 540, amount: 1 })

    expect(quote.capAfter).toBe(570)
    expect(quote.blockedBy).toBeNull()
    expect(quote.canBuy).toBe(true)
  })

  it('allows landing exactly on the cap', () => {
    const quote = quoteAddon({ ...RPM, heldAmount: 0, amount: 20 })

    expect(quote.capAfter).toBe(600)
    expect(quote.canBuy).toBe(true)
  })

  it('refuses one block past the cap', () => {
    const quote = quoteAddon({ ...RPM, heldAmount: 0, amount: 21 })

    expect(quote.capAfter).toBe(630)
    expect(quote.blockedBy).toBe('cap')
  })

  it('does not scale concurrency, which sells and caps in the same unit', () => {
    expect(quoteAddon({ ...CONCURRENCY, heldAmount: 8, amount: 2 }).capAfter).toBe(10)
    expect(quoteAddon({ ...CONCURRENCY, heldAmount: 8, amount: 2 }).canBuy).toBe(true)
    expect(quoteAddon({ ...CONCURRENCY, heldAmount: 8, amount: 3 }).blockedBy).toBe('cap')
  })
})

describe('quoteAddon — the cap is cumulative, not per purchase', () => {
  it('refuses a tiny buy when the existing holding is already near the cap', () => {
    // 2 is trivially small against a cap of 100; what matters is 99 + 2.
    const quote = quoteAddon({ ...CONCURRENCY, cap: 100, heldAmount: 99, amount: 2 })

    expect(quote.blockedBy).toBe('cap')
    expect(quote.canBuy).toBe(false)
  })

  it('allows the same tiny buy from a clean slate', () => {
    const quote = quoteAddon({ ...CONCURRENCY, cap: 100, heldAmount: 0, amount: 2 })

    expect(quote.canBuy).toBe(true)
  })

  it('refuses a single block when one block is one too many', () => {
    const quote = quoteAddon({ ...RPM, heldAmount: 590, amount: 1 })

    expect(quote.capAfter).toBe(620)
    expect(quote.blockedBy).toBe('cap')
  })
})

describe('quoteAddon — affordability', () => {
  it('blocks when the total exceeds the balance', () => {
    const quote = quoteAddon({ ...CONCURRENCY, unitPrice: 10, amount: 2, months: 1, balance: 19 })

    expect(quote.total).toBe(20)
    expect(quote.blockedBy).toBe('balance')
    expect(quote.canBuy).toBe(false)
  })

  it('allows a total that exactly equals the balance', () => {
    const quote = quoteAddon({ ...CONCURRENCY, unitPrice: 2.5, amount: 4, months: 2, balance: 20 })

    expect(quote.total).toBe(20)
    expect(quote.blockedBy).toBeNull()
    expect(quote.canBuy).toBe(true)
  })

  it('allows an exact match that floating point would otherwise refuse', () => {
    // 0.05 * 3 is 0.15000000000000002 in binary floating point.
    const quote = quoteAddon({ ...CONCURRENCY, unitPrice: 0.05, amount: 3, months: 1, balance: 0.15 })

    expect(quote.total).toBe(0.15)
    expect(quote.canBuy).toBe(true)
  })

  it('treats an unloaded balance as unaffordable rather than as zero', () => {
    const quote = quoteAddon({ ...CONCURRENCY, balance: null })

    expect(quote.blockedBy).toBe('balance')
    expect(quote.canBuy).toBe(false)
  })

  it('reports the cap before the balance when both would block', () => {
    const quote = quoteAddon({ ...RPM, heldAmount: 590, amount: 1, balance: 0 })

    expect(quote.blockedBy).toBe('cap')
  })
})

describe('quoteAddon — total arithmetic', () => {
  it('multiplies quantity by months for concurrency', () => {
    expect(quoteAddon({ ...CONCURRENCY, unitPrice: 1.5, amount: 4, months: 3 }).total).toBe(18)
  })

  it('prices RPM per block, not per request per minute', () => {
    // 3 blocks x 2 USD x 2 months = 12. Pricing the 90 granted req/min instead
    // would bill 360 for the same purchase.
    const quote = quoteAddon({ ...RPM, unitPrice: 2, amount: 3, months: 2 })

    expect(quote.total).toBe(12)
    expect(quote.capAfter).toBe(90)
  })

  it('scales linearly with months alone', () => {
    expect(quoteAddon({ ...RPM, unitPrice: 2, amount: 1, months: 12 }).total).toBe(24)
  })
})

describe('quoteAddon — input validation', () => {
  it('blocks on amount before months so the first empty field is the one named', () => {
    expect(quoteAddon({ ...CONCURRENCY, amount: 0, months: 0 }).blockedBy).toBe('amount')
  })

  it('blocks on months once the amount is valid', () => {
    expect(quoteAddon({ ...CONCURRENCY, amount: 1, months: 0 }).blockedBy).toBe('months')
  })

  it('treats a cleared number input as an invalid amount rather than a free purchase', () => {
    const quote = quoteAddon({ ...CONCURRENCY, amount: Number.NaN, months: Number.NaN })

    expect(quote.total).toBe(0)
    expect(quote.blockedBy).toBe('amount')
    expect(quote.canBuy).toBe(false)
  })
})

describe('canAfford', () => {
  it('is inclusive at the boundary', () => {
    expect(canAfford(10, 10)).toBe(true)
    expect(canAfford(10, 10.01)).toBe(false)
  })

  it('refuses everything when the balance has not loaded', () => {
    expect(canAfford(null, 0)).toBe(false)
  })
})
