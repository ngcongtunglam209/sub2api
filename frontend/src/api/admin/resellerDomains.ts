/**
 * Admin reseller domain API endpoints
 */

import { apiClient } from '../client'
import type { CreateResellerDomainRequest, ResellerDomain, ResellerDomainStatus } from '@/types'

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
 * Flip a domain between active and disabled. Disabling is the reversible
 * alternative to deleting: the row and its notes survive.
 */
export async function setStatus(id: number, status: ResellerDomainStatus): Promise<ResellerDomain> {
  const { data } = await apiClient.patch<ResellerDomain>(`/admin/reseller-domains/${id}`, { status })
  return data
}

export async function deleteDomain(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/reseller-domains/${id}`)
  return data
}

const resellerDomainsAPI = {
  list,
  create,
  setStatus,
  delete: deleteDomain
}

export default resellerDomainsAPI
