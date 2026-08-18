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
  PurchaseAddonRequest
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

export const addonsAPI = {
  getAddons,
  purchaseAddon
}

export default addonsAPI
