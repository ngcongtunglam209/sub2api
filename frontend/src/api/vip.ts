/**
 * User-facing VIP tier API endpoints
 */

import { apiClient } from './client'
import type { VIPStatus, VIPTier } from '@/types'

export async function getStatus(): Promise<VIPStatus> {
  const { data } = await apiClient.get<VIPStatus>('/vip/status')
  return data
}

/** The enabled ladder only; disabled tiers are unreachable and not shown. */
export async function getTiers(): Promise<VIPTier[]> {
  const { data } = await apiClient.get<VIPTier[]>('/vip/tiers')
  return data
}

export default {
  getStatus,
  getTiers
}
