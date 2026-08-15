//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubVIPExpiryRepo struct {
	batches        [][]int64
	listErr        error
	expireErr      error
	expired        [][]int64
	listCalls      int
	updatesNothing bool
}

func (s *stubVIPExpiryRepo) ListExpiredVIPUserIDs(_ context.Context, _ time.Time, _ int) ([]int64, error) {
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

func (s *stubVIPExpiryRepo) ExpireVIPTiers(_ context.Context, ids []int64) (int, error) {
	if s.expireErr != nil {
		return 0, s.expireErr
	}
	s.expired = append(s.expired, ids)
	if s.updatesNothing {
		return 0, nil
	}
	return len(ids), nil
}

type recordingInvalidator struct {
	users []int64
}

func (r *recordingInvalidator) InvalidateAuthCacheByKey(_ context.Context, _ string)    {}
func (r *recordingInvalidator) InvalidateAuthCacheByGroupID(_ context.Context, _ int64) {}
func (r *recordingInvalidator) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	r.users = append(r.users, userID)
}

// A short batch means the backlog is drained; asking again would be a wasted
// round trip every cycle.
func TestVIPExpiry_StopsAfterAShortBatch(t *testing.T) {
	repo := &stubVIPExpiryRepo{batches: [][]int64{{1, 2, 3}}}
	invalidator := &recordingInvalidator{}
	svc := NewVIPExpiryService(repo, time.Minute)
	svc.SetAuthCacheInvalidator(invalidator)

	svc.RunOnce()

	require.Equal(t, [][]int64{{1, 2, 3}}, repo.expired)
	require.Equal(t, 1, repo.listCalls)
	// The tier carried a concurrency floor into the cached auth snapshot, so
	// every expired user's snapshot has to be dropped.
	require.Equal(t, []int64{1, 2, 3}, invalidator.users)
}

// The loop must not spin when the UPDATE's own guard filters every row it was
// handed — otherwise the same rows are read forever.
func TestVIPExpiry_StopsWhenNothingWasUpdated(t *testing.T) {
	full := make([]int64, vipExpiryBatchSize)
	for i := range full {
		full[i] = int64(i + 1)
	}
	repo := &stubVIPExpiryRepo{batches: [][]int64{full, full, full}, updatesNothing: true}

	svc := NewVIPExpiryService(repo, time.Minute)
	svc.RunOnce()

	require.Equal(t, 1, repo.listCalls, "a batch that changed nothing must end the cycle")
}

func TestVIPExpiry_StopsOnRepoErrors(t *testing.T) {
	listFails := &stubVIPExpiryRepo{listErr: errors.New("db down")}
	svc := NewVIPExpiryService(listFails, time.Minute)
	svc.RunOnce()
	require.Empty(t, listFails.expired)

	expireFails := &stubVIPExpiryRepo{batches: [][]int64{{7}}, expireErr: errors.New("db down")}
	invalidator := &recordingInvalidator{}
	svc = NewVIPExpiryService(expireFails, time.Minute)
	svc.SetAuthCacheInvalidator(invalidator)
	svc.RunOnce()
	// Nothing was cleared, so nothing may be invalidated: dropping the snapshot
	// here would rebuild it from the tier that is still in place.
	require.Empty(t, invalidator.users)
}

func TestVIPExpiry_NoRepoIsInert(t *testing.T) {
	require.NotPanics(t, func() {
		NewVIPExpiryService(nil, time.Minute).RunOnce()
		var nilSvc *VIPExpiryService
		nilSvc.RunOnce()
		nilSvc.Stop()
	})
}
