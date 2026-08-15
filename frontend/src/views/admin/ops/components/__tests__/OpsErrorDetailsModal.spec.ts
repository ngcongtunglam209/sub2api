import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import OpsErrorDetailsModal from '../OpsErrorDetailsModal.vue'

const listUpstreamErrors = vi.fn()
const listRequestErrors = vi.fn()

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    listUpstreamErrors: (...args: unknown[]) => listUpstreamErrors(...args),
    listRequestErrors: (...args: unknown[]) => listRequestErrors(...args)
  }
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const DialogStub = { template: '<div><slot /></div>' }
const SelectStub = { template: '<div />', props: ['modelValue', 'options'] }
const TableStub = { template: '<div />', props: ['rows', 'total', 'loading', 'page', 'pageSize'] }

function mountModal(props: Record<string, unknown> = {}) {
  return mount(OpsErrorDetailsModal, {
    props: {
      show: true,
      timeRange: '1h',
      errorType: 'upstream' as const,
      ...props
    },
    global: {
      stubs: {
        BaseDialog: DialogStub,
        Select: SelectStub,
        OpsErrorLogTable: TableStub
      }
    }
  })
}

describe('OpsErrorDetailsModal', () => {
  beforeEach(() => {
    listUpstreamErrors.mockReset()
    listRequestErrors.mockReset()
    listUpstreamErrors.mockResolvedValue({ items: [{ id: 1 }], total: 97 })
    listRequestErrors.mockResolvedValue({ items: [], total: 0 })
  })

  // The dashboard applies `?open_error_details=1&error_type=upstream` during its
  // own setup, so the modal is created with `show` already true and never sees an
  // edge. Without an immediate watcher it opened empty on every deep link.
  it('loads when it is mounted already open', async () => {
    mountModal()
    await flushPromises()

    expect(listUpstreamErrors).toHaveBeenCalledTimes(1)
    expect(listUpstreamErrors.mock.calls[0][0]).toMatchObject({ page: 1, view: 'errors' })
  })

  it('queries the request-error endpoint for the request type', async () => {
    mountModal({ errorType: 'request' })
    await flushPromises()

    expect(listRequestErrors).toHaveBeenCalledTimes(1)
    expect(listUpstreamErrors).not.toHaveBeenCalled()
  })

  it('stays quiet while closed, and loads when it is opened later', async () => {
    const wrapper = mountModal({ show: false })
    await flushPromises()
    expect(listUpstreamErrors).not.toHaveBeenCalled()

    await wrapper.setProps({ show: true })
    await flushPromises()
    // Opening from the button fetches twice: `resetFilters` pins the phase filter
    // to `upstream`, and the filter watcher — already registered by then — fires a
    // second request. Long-standing behaviour, unrelated to the deep-link fix, so
    // this asserts that it loads rather than pinning the count.
    expect(listUpstreamErrors).toHaveBeenCalled()
    expect(listUpstreamErrors.mock.calls.at(-1)?.[0]).toMatchObject({ phase: 'upstream' })
  })
})
