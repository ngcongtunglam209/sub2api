package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stubAddonExpiryRepo records what the sweep asked for. Only the two sweep
// methods carry behaviour; the rest satisfy the interface.
type stubAddonExpiryRepo struct {
	*stubAddonRepo

	batches   [][]int64
	deleted   []int
	listErr   error
	deleteErr error
	listCalls int
}

func (s *stubAddonExpiryRepo) ListExpiredAddonUserIDs(_ context.Context, _ time.Time, _ int) ([]int64, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listCalls >= len(s.batches) {
		return nil, nil
	}
	batch := s.batches[s.listCalls]
	s.listCalls++
	return batch, nil
}

func (s *stubAddonExpiryRepo) DeleteExpiredAddons(_ context.Context, _ time.Time, userIDs []int64) (int, error) {
	if s.deleteErr != nil {
		return 0, s.deleteErr
	}
	s.deleted = append(s.deleted, len(userIDs))
	return len(userIDs), nil
}

type stubAuthCacheInvalidator struct {
	users []int64
}

func (s *stubAuthCacheInvalidator) InvalidateAuthCacheByKey(context.Context, string) {}

func (s *stubAuthCacheInvalidator) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	s.users = append(s.users, userID)
}

func (s *stubAuthCacheInvalidator) InvalidateAuthCacheByGroupID(context.Context, int64) {}

func newAddonExpiryTestService(repo UserAddonRepository, invalidator APIKeyAuthCacheInvalidator) *AddonExpiryService {
	// interval 0 keeps the ticker from starting; RunOnce is driven directly.
	svc := NewAddonExpiryService(repo, 0)
	svc.SetAuthCacheInvalidator(invalidator)
	return svc
}

// The sweep deletes what it listed and invalidates the affected users' cached
// snapshots afterwards — after, because a snapshot rebuilt before the delete
// would just cache the row on its way out.
func TestAddonExpirySweepDeletesAndInvalidates(t *testing.T) {
	repo := &stubAddonExpiryRepo{
		stubAddonRepo: newStubAddonRepo(0),
		batches:       [][]int64{{7, 8, 9}},
	}
	invalidator := &stubAuthCacheInvalidator{}

	newAddonExpiryTestService(repo, invalidator).RunOnce()

	require.Equal(t, []int{3}, repo.deleted)
	require.Equal(t, []int64{7, 8, 9}, invalidator.users)
}

// A short batch means the backlog is drained; looping again would only cost a
// round trip.
func TestAddonExpirySweepStopsOnAShortBatch(t *testing.T) {
	repo := &stubAddonExpiryRepo{
		stubAddonRepo: newStubAddonRepo(0),
		batches:       [][]int64{{1}, {2}},
	}

	newAddonExpiryTestService(repo, nil).RunOnce()

	require.Equal(t, 1, repo.listCalls, "a batch smaller than the page size ends the cycle")
}

// Nothing expired is not an error and must not delete anything.
func TestAddonExpirySweepDoesNothingWhenNothingLapsed(t *testing.T) {
	repo := &stubAddonExpiryRepo{stubAddonRepo: newStubAddonRepo(0)}
	invalidator := &stubAuthCacheInvalidator{}

	newAddonExpiryTestService(repo, invalidator).RunOnce()

	require.Empty(t, repo.deleted)
	require.Empty(t, invalidator.users)
}

// A failing sweep is logged and abandoned, never retried in a tight loop — and
// nothing is invalidated on the strength of a listing that failed.
func TestAddonExpirySweepGivesUpOnError(t *testing.T) {
	t.Run("list fails", func(t *testing.T) {
		repo := &stubAddonExpiryRepo{stubAddonRepo: newStubAddonRepo(0), listErr: errors.New("db down")}
		invalidator := &stubAuthCacheInvalidator{}

		newAddonExpiryTestService(repo, invalidator).RunOnce()

		require.Empty(t, repo.deleted)
		require.Empty(t, invalidator.users)
	})

	t.Run("delete fails", func(t *testing.T) {
		repo := &stubAddonExpiryRepo{
			stubAddonRepo: newStubAddonRepo(0),
			batches:       [][]int64{{1, 2}},
			deleteErr:     errors.New("db down"),
		}
		invalidator := &stubAuthCacheInvalidator{}

		newAddonExpiryTestService(repo, invalidator).RunOnce()

		require.Empty(t, invalidator.users, "nothing was cleared, so nothing should be invalidated")
	})
}
