/**
 * User-facing add-on store endpoints (own account only)
 *
 * Everything here spends the caller's balance. The server is the authority on
 * money: each purchase returns the state it committed, and callers are expected
 * to re-read the balance rather than subtract the price themselves.
 */

import { apiClient } from './client'
import type {
  AddonHolding,
  AddonsResponse,
  PurchaseAddonRequest,
  ResellerAssignment,
  ResellerPlan
} from '@/types'

/** Prices, caps, and whatever the caller is already renting. */
export async function getAddons(): Promise<AddonsResponse> {
  const { data } = await apiClient.get<AddonsResponse>('/addons')
  return data
}

/**
 * Spend balance on extra capacity. Returns the holding as it now stands —
 * buying while an add-on is live extends and tops up the same holding, so the
 * response is not necessarily what was just bought.
 */
export async function purchaseAddon(request: PurchaseAddonRequest): Promise<AddonHolding> {
  const { data } = await apiClient.post<AddonHolding>('/addons/purchase', request)
  return data
}

/**
 * The tiers a user may buy for themselves. Same shape the admin list returns,
 * filtered server-side to the purchasable ones.
 */
export async function listResellerPlans(): Promise<ResellerPlan[]> {
  const { data } = await apiClient.get<ResellerPlan[]>('/reseller-plans')
  return data ?? []
}

/**
 * Buy a reseller tier with balance. The server refuses a second purchase while
 * an assignment is live — paying the `credit_rate` rebate twice for one tier is
 * a money bug, not a convenience.
 */
export async function purchaseResellerPlan(planId: number): Promise<ResellerAssignment> {
  const { data } = await apiClient.post<ResellerAssignment>(`/reseller-plans/${planId}/purchase`)
  return data
}

export const addonsAPI = {
  getAddons,
  purchaseAddon,
  listResellerPlans,
  purchaseResellerPlan
}

export default addonsAPI
