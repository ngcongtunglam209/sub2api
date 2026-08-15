import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, put },
}))

import { getStatus, getTiers } from '@/api/vip'
import { setUserTier } from '@/api/admin/vipTiers'

describe('VIP API', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
    get.mockResolvedValue({ data: null })
    put.mockResolvedValue({ data: { message: 'ok' } })
  })

  it('returns the user status payload unchanged', async () => {
    const status = {
      tier: null,
      next_tier: null,
      qualifying_spend: 12.5,
      total_paid_usd: 12.5,
      spend_to_next_tier: 7.5,
      expires_at: null,
      locked: false,
    }
    get.mockResolvedValue({ data: status })

    await expect(getStatus()).resolves.toEqual(status)
    expect(get).toHaveBeenCalledWith('/vip/status')
  })

  it('reads the ladder from the user endpoint', async () => {
    get.mockResolvedValue({ data: [] })
    await expect(getTiers()).resolves.toEqual([])
    expect(get).toHaveBeenCalledWith('/vip/tiers')
  })

  // Releasing a pinned user is a null tier_id, not an omitted field: the
  // backend distinguishes "clear the pin" from "nothing to change".
  it('sends an explicit null to release a pinned user', async () => {
    await setUserTier(42, null)
    expect(put).toHaveBeenCalledWith('/admin/users/42/vip-tier', { tier_id: null })

    await setUserTier(42, 3)
    expect(put).toHaveBeenLastCalledWith('/admin/users/42/vip-tier', { tier_id: 3 })
  })
})
