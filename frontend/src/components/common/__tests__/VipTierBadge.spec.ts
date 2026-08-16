import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import VipTierBadge from '../VipTierBadge.vue'
import { resetVipStatus } from '@/composables/useVipStatus'
import { useAuthStore } from '@/stores'
import type { VIPStatus, VIPTier } from '@/types'

const getStatus = vi.fn()

vi.mock('@/api/vip', () => ({
  default: {
    getStatus: (...args: unknown[]) => getStatus(...args)
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key
    })
  }
})

const tier = (overrides: Partial<VIPTier> = {}): VIPTier => ({
  id: 2,
  level: 2,
  name: 'VIP2',
  min_spend_usd: 100,
  rate_multiplier: 0.9,
  concurrency: 10,
  grace_days: 30,
  badge_color: '#7c3aed',
  enabled: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  ...overrides
})

const status = (overrides: Partial<VIPStatus> = {}): VIPStatus => ({
  tier: tier(),
  next_tier: null,
  qualifying_spend: 150,
  total_paid_usd: 150,
  spend_to_next_tier: 0,
  expires_at: null,
  locked: false,
  ...overrides
})

function signIn(id = 7) {
  const authStore = useAuthStore()
  authStore.user = { id, username: 'admin', role: 'admin' } as never
  return authStore
}

describe('VipTierBadge', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetVipStatus()
    getStatus.mockReset()
  })

  it('renders the tier name in its configured colour', async () => {
    getStatus.mockResolvedValue(status())
    signIn()

    const wrapper = mount(VipTierBadge)
    await flushPromises()

    expect(wrapper.text()).toBe('VIP2')
    expect(wrapper.attributes('style')).toContain('rgb(124, 58, 237)')
  })

  it('names the unranked state BASE while a ladder exists', async () => {
    getStatus.mockResolvedValue(status({ tier: null, next_tier: tier({ id: 1, level: 1, name: 'VIP1' }) }))
    signIn()

    const wrapper = mount(VipTierBadge)
    await flushPromises()

    expect(wrapper.text()).toBe('vip.baseTier')
  })

  it('renders nothing when no tiers are configured', async () => {
    getStatus.mockResolvedValue(status({ tier: null, next_tier: null }))
    signIn()

    const wrapper = mount(VipTierBadge)
    await flushPromises()

    expect(wrapper.find('span').exists()).toBe(false)
  })

  it('stays hidden when the status request fails', async () => {
    getStatus.mockRejectedValue(new Error('vip disabled'))
    signIn()

    const wrapper = mount(VipTierBadge)
    await flushPromises()

    expect(wrapper.find('span').exists()).toBe(false)
  })

  it('fetches once for two mounted badges', async () => {
    getStatus.mockResolvedValue(status())
    signIn()

    mount(VipTierBadge)
    mount(VipTierBadge)
    await flushPromises()

    expect(getStatus).toHaveBeenCalledTimes(1)
  })

  it('refetches when a different user signs in', async () => {
    getStatus.mockResolvedValue(status())
    const authStore = signIn(7)

    const wrapper = mount(VipTierBadge)
    await flushPromises()
    expect(getStatus).toHaveBeenCalledTimes(1)

    getStatus.mockResolvedValue(status({ tier: tier({ id: 3, name: 'VIP3', badge_color: '#059669' }) }))
    authStore.user = { id: 9, username: 'other', role: 'user' } as never
    await flushPromises()

    expect(getStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toBe('VIP3')
  })

  it('surfaces the pinned state as a tooltip', async () => {
    getStatus.mockResolvedValue(status({ locked: true }))
    signIn()

    const wrapper = mount(VipTierBadge)
    await flushPromises()

    expect(wrapper.attributes('title')).toBe('vip.pinned')
  })
})
