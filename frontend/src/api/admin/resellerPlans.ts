/**
 * Admin reseller plan API endpoints
 */

import { apiClient } from '../client'
import type { ResellerPlan, ResellerPlanRequest } from '@/types'

export async function list(): Promise<ResellerPlan[]> {
  const { data } = await apiClient.get<ResellerPlan[]>('/admin/reseller-plans')
  return data
}

export async function update(id: number, request: ResellerPlanRequest): Promise<ResellerPlan> {
  const { data } = await apiClient.put<ResellerPlan>(`/admin/reseller-plans/${id}`, request)
  return data
}

/** Put a user on a plan. Re-assigning replaces whatever they held before. */
export async function assignToUser(userId: number, planId: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/users/${userId}/reseller-plan`, {
    plan_id: planId
  })
  return data
}

export async function revokeFromUser(userId: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/users/${userId}/reseller-plan`)
  return data
}

const resellerPlansAPI = {
  list,
  update,
  assignToUser,
  revokeFromUser
}

export default resellerPlansAPI
