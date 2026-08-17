/**
 * Admin reseller domain API endpoints
 */

import { apiClient } from '../client'
import type {
  CreateResellerDomainRequest,
  ResellerDomain,
  ResellerDomainStatus,
  UpdateResellerDomainRequest
} from '@/types'

export async function list(options?: { signal?: AbortSignal }): Promise<ResellerDomain[]> {
  const { data } = await apiClient.get<ResellerDomain[]>('/admin/reseller-domains', {
    signal: options?.signal
  })
  return data
}

export async function create(request: CreateResellerDomainRequest): Promise<ResellerDomain> {
  const { data } = await apiClient.post<ResellerDomain>('/admin/reseller-domains', request)
  return data
}

/**
 * Partial update. Every field is optional and an omitted one is left alone, so
 * the branding editor and the status toggle can share one endpoint without
 * either clobbering the other's fields. Sending `''` for a branding field
 * clears the override — the host falls back to the platform's own branding.
 */
export async function update(id: number, request: UpdateResellerDomainRequest): Promise<ResellerDomain> {
  const { data } = await apiClient.patch<ResellerDomain>(`/admin/reseller-domains/${id}`, request)
  return data
}

/**
 * Flip a domain between active and disabled. Disabling is the reversible
 * alternative to deleting: the row, its notes and its branding survive.
 */
export async function setStatus(id: number, status: ResellerDomainStatus): Promise<ResellerDomain> {
  return update(id, { status })
}

export async function deleteDomain(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/reseller-domains/${id}`)
  return data
}

const resellerDomainsAPI = {
  list,
  create,
  update,
  setStatus,
  delete: deleteDomain
}

export default resellerDomainsAPI
