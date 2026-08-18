/**
 * Pure pricing and eligibility rules for the add-on store.
 *
 * Kept out of `StoreView.vue` so the money arithmetic can be tested without
 * mounting a component: every rule here decides whether a user is about to be
 * charged, and a bug in any of them is a billing bug, not a rendering one.
 *
 * Nothing in this module translates or formats. It returns a *reason code* for
 * a blocked purchase and lets the view pick the message, so the rules stay
 * independent of the locale layer.
 */

/** Why the buy button is disabled, in the order the checks are applied. */
export type AddonBlockReason = 'amount' | 'months' | 'cap' | 'balance' | null

export interface AddonQuoteInput {
  /** USD per purchase unit, per month. */
  unitPrice: number
  /** Ceiling on total held capacity, in the dimension's NATIVE unit. */
  cap: number
  /**
   * Native units granted per purchased unit.
   *
   * This is the whole reason the cap check is not a one-liner. Concurrency
   * sells and caps in the same unit, so it is 1. RPM sells in `rpm_step`
   * blocks while `rpm_cap` counts single requests per minute, so a purchase of
   * 3 blocks at step 30 is 90 against the cap, not 3.
   */
  unitsPerPurchase: number
  /** Existing holding, already in the native unit. Never scaled again. */
  heldAmount: number
  /** Quantity in purchase units. `NaN` when the input is empty. */
  amount: number
  months: number
  /** `null` while the account has not loaded — never treated as zero. */
  balance: number | null
}

export interface AddonQuote {
  /** USD the purchase would cost. */
  total: number
  /** Native-unit capacity the user would hold once this purchase lands. */
  capAfter: number
  blockedBy: AddonBlockReason
  canBuy: boolean
}

/**
 * Snap off binary-representation dust: `0.05 * 3` is `0.15000000000000002`, and
 * a user holding exactly `0.15` would otherwise be refused a purchase the
 * server accepts. Six places is far below any real unit price, so this only
 * removes float error and never rounds away sub-cent pricing.
 */
export function normalizeMoney(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.round(value * 1e6) / 1e6
}

/** `null` balance means "not loaded yet", which can never afford anything. */
export function canAfford(balance: number | null, price: number): boolean {
  if (balance === null || !Number.isFinite(balance)) return false
  return normalizeMoney(price) <= normalizeMoney(balance)
}

export function quoteAddon(input: AddonQuoteInput): AddonQuote {
  const amount = Number.isFinite(input.amount) ? input.amount : 0
  const months = Number.isFinite(input.months) ? input.months : 0

  const total = normalizeMoney(input.unitPrice * amount * months)

  // The cap is cumulative: what matters is the total held afterwards, not the
  // size of this one purchase. Someone sitting just under the ceiling cannot
  // buy even a single extra block.
  const capAfter = input.heldAmount + amount * input.unitsPerPurchase

  let blockedBy: AddonBlockReason = null
  if (amount < 1) {
    blockedBy = 'amount'
  } else if (months < 1) {
    blockedBy = 'months'
  } else if (capAfter > input.cap) {
    blockedBy = 'cap'
  } else if (!canAfford(input.balance, total)) {
    blockedBy = 'balance'
  }

  return { total, capAfter, blockedBy, canBuy: blockedBy === null }
}
