import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import UserVIPStatusCard from '../UserVIPStatusCard.vue'
import type { VIPStatus, VIPTier } from '@/types'

const getStatus = vi.fn()

vi.mock('@/api/vip', () => ({
  default: {
    getStatus: (...args: unknown[]) => getStatus(...args)
  }
}))

vi.mock('@/composables/useDisplayCurrency', () => ({
  useDisplayCurrency: () => ({ format: (value: number) => `$${value}` })
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
  id: 4,
  level: 4,
  name: 'VIP4',
  min_spend_usd: 1500,
  rate_multiplier: 0.7,
  concurrency: 1,
  rpm_limit: 0,
  unlimited_concurrency: false,
  unlimited_rpm: false,
  grace_days: 60,
  badge_color: '#f59e0b',
  enabled: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  ...overrides
})

const status = (overrides: Partial<VIPStatus> = {}): VIPStatus => ({
  tier: tier(),
  next_tier: null,
  qualifying_spend: 2000,
  total_paid_usd: 2000,
  spend_to_next_tier: 0,
  expires_at: null,
  locked: false,
  ...overrides
})

async function renderCard(value: VIPStatus) {
  getStatus.mockResolvedValueOnce(value)
  const wrapper = mount(UserVIPStatusCard)
  await flushPromises()
  return wrapper
}

describe('UserVIPStatusCard perk rendering', () => {
  it('signs a bounded bonus so it does not read as the total', async () => {
    const wrapper = await renderCard(status({ tier: tier({ concurrency: 5, rpm_limit: 60 }) }))

    expect(wrapper.text()).toContain('+5')
    expect(wrapper.text()).toContain('+60')
  })

  // The regression this file exists for. Both numbers are addends, and an
  // exempt tier leaves its addend at whatever it was — so printing the number
  // advertises the top tier as granting "+1", less than every tier below it.
  it('shows an exemption instead of the unused addend', async () => {
    const wrapper = await renderCard(
      status({
        tier: tier({
          concurrency: 1,
          rpm_limit: 0,
          unlimited_concurrency: true,
          unlimited_rpm: true
        })
      })
    )

    const text = wrapper.text()
    expect(text).toContain('vip.unlimited')
    expect(text).not.toContain('+1')
    expect(text).not.toContain('+0')
  })

  // The two exemptions are independent, and this is the shape worth supporting:
  // requests per minute are cheap to hand out, slots are the scarce resource.
  it('keeps a bounded concurrency while exempting RPM', async () => {
    const wrapper = await renderCard(
      status({ tier: tier({ concurrency: 4, unlimited_rpm: true }) })
    )

    const text = wrapper.text()
    expect(text).toContain('+4')
    expect(text).toContain('vip.unlimited')
  })

  // A tier that grants no RPM hides the row rather than advertising "+0" as if
  // it were a perk.
  it('omits the RPM row when the tier grants none', async () => {
    const wrapper = await renderCard(status({ tier: tier({ concurrency: 3, rpm_limit: 0 }) }))

    expect(wrapper.text()).not.toContain('vip.rpm')
  })
})
