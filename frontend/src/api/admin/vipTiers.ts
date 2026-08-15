/**
 * Admin VIP tier API endpoints
 */

import { apiClient } from '../client'
import type { VIPStatus, VIPTier, VIPTierRequest } from '@/types'

export async function list(): Promise<VIPTier[]> {
  const { data } = await apiClient.get<VIPTier[]>('/admin/vip-tiers')
  return data
}

export async function create(request: VIPTierRequest): Promise<VIPTier> {
  const { data } = await apiClient.post<VIPTier>('/admin/vip-tiers', request)
  return data
}

export async function update(id: number, request: VIPTierRequest): Promise<VIPTier> {
  const { data } = await apiClient.put<VIPTier>(`/admin/vip-tiers/${id}`, request)
  return data
}

export async function remove(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/vip-tiers/${id}`)
  return data
}

export async function getUserStatus(userId: number): Promise<VIPStatus> {
  const { data } = await apiClient.get<VIPStatus>(`/admin/users/${userId}/vip-tier`)
  return data
}

/**
 * Pin a user to a tier, or release them back to automatic grading by passing
 * null. A pinned tier never expires and is ignored by the grader.
 */
export async function setUserTier(userId: number, tierId: number | null): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>(`/admin/users/${userId}/vip-tier`, {
    tier_id: tierId
  })
  return data
}

export default {
  list,
  create,
  update,
  remove,
  getUserStatus,
  setUserTier
}
