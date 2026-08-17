/**
 * User-facing reseller API endpoints (own account only)
 */

import { apiClient } from './client'
import type {
  BasePaginationResponse,
  GenerateResellerCodesRequest,
  GenerateResellerCodesResponse,
  ResellerAssignment,
  ResellerCode
} from '@/types'

/** null when the current user holds no reseller plan. */
export async function getPlan(): Promise<ResellerAssignment | null> {
  const { data } = await apiClient.get<ResellerAssignment | null>('/reseller/plan')
  return data ?? null
}

export async function getCodes(
  page: number = 1,
  pageSize: number = 20,
  options?: {
    signal?: AbortSignal
  }
): Promise<BasePaginationResponse<ResellerCode>> {
  const { data } = await apiClient.get<BasePaginationResponse<ResellerCode>>('/reseller/codes', {
    params: { page, page_size: pageSize },
    signal: options?.signal
  })
  return data
}

/**
 * Mint codes against the reseller's own credit. The response carries the only
 * copy of the code strings the API will ever hand out — the caller must show
 * them, not just count them.
 */
export async function generateCodes(
  request: GenerateResellerCodesRequest
): Promise<GenerateResellerCodesResponse> {
  const { data } = await apiClient.post<GenerateResellerCodesResponse>('/reseller/codes', request)
  return data
}

export const resellerAPI = {
  getPlan,
  getCodes,
  generateCodes
}

export default resellerAPI
