/**
 * Shared VIP status for chrome that follows the signed-in user around.
 *
 * The state lives at module scope on purpose: the header renders the tier badge
 * twice (inline next to the role, and inside the user dropdown for narrow
 * screens), and both copies should cost one request, not two.
 */

import { computed, ref, watch } from 'vue'

import vipAPI from '@/api/vip'
import { useAuthStore } from '@/stores'
import type { VIPStatus } from '@/types'

const status = ref<VIPStatus | null>(null)
let loadedForUserID: number | null = null
let inFlight: Promise<void> | null = null

function load(userID: number): Promise<void> {
  if (inFlight) return inFlight
  inFlight = (async () => {
    try {
      status.value = await vipAPI.getStatus()
      loadedForUserID = userID
    } catch (error) {
      // A deployment with no tiers configured is the normal case, not an error
      // worth shouting about: leave the badge hidden.
      console.debug('VIP status unavailable:', error)
      status.value = null
      loadedForUserID = null
    } finally {
      inFlight = null
    }
  })()
  return inFlight
}

export function resetVipStatus(): void {
  status.value = null
  loadedForUserID = null
}

export function useVipStatus() {
  const authStore = useAuthStore()

  // Keyed on the user id rather than fetched once: logging out and back in as
  // somebody else inside the same SPA session would otherwise keep showing the
  // previous account's tier.
  watch(
    () => authStore.user?.id ?? null,
    (userID) => {
      if (userID == null) {
        resetVipStatus()
        return
      }
      if (userID !== loadedForUserID) void load(userID)
    },
    { immediate: true }
  )

  // With tiers configured, an unranked user still has a next tier to climb to.
  // Both being empty means the site has no ladder at all, so there is no rank
  // worth announcing — not even BASE.
  const hasTierLadder = computed(() => {
    const current = status.value
    return current != null && (current.tier != null || current.next_tier != null)
  })

  return {
    status: computed(() => status.value),
    tier: computed(() => status.value?.tier ?? null),
    hasTierLadder
  }
}
