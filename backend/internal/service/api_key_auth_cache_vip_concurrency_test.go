package service

// VIP 并发叠加回归：等级赠送的并发必须累加到 users.concurrency 之上。
//
// users.concurrency 是可售卖的加购项。此前这里取二者较大值，导致等级数字
// 大于加购数时，用户买的并发被整个吞掉——付了钱却毫无变化。下面的用例把
// “相加”钉死，尤其是赠送值小于用户值的那一条：在旧的 max() 语义下它会
// 悄悄退化成不加。

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubVIPBenefitRepo struct {
	concurrency          int
	rpm                  int
	unlimitedConcurrency bool
	unlimitedRPM         bool
	err                  error
	calls                int
}

// Mirrors the real repository on failure: a zero value, not a partial one. A
// stub that returned its configured numbers alongside the error would hide a
// caller that forgot to check it.
func (s *stubVIPBenefitRepo) GetVIPBenefits(context.Context, int64) (VIPBenefits, error) {
	s.calls++
	if s.err != nil {
		return VIPBenefits{}, s.err
	}
	return VIPBenefits{
		Concurrency:          s.concurrency,
		RPM:                  s.rpm,
		UnlimitedConcurrency: s.unlimitedConcurrency,
		UnlimitedRPM:         s.unlimitedRPM,
	}, nil
}

func vipConcurrencyTestAPIKey(userConcurrency int) *APIKey {
	return &APIKey{
		ID:     91,
		UserID: 41,
		Name:   "vip-concurrency",
		Status: StatusActive,
		User: &User{
			ID:          41,
			Email:       "vip-concurrency@test.local",
			Status:      StatusActive,
			Concurrency: userConcurrency,
		},
	}
}

func TestAPIKeyAuthSnapshotAddsVIPConcurrencyOnTopOfUserValue(t *testing.T) {
	for _, tc := range []struct {
		name            string
		userConcurrency int
		tierBonus       int
		want            int
	}{
		{name: "bonus larger than user value", userConcurrency: 5, tierBonus: 20, want: 25},
		// The case the max() rule got wrong: a user who bought their way past
		// the tier used to see the bonus vanish entirely.
		{name: "bonus smaller than user value", userConcurrency: 25, tierBonus: 5, want: 30},
		{name: "bonus equal to user value", userConcurrency: 5, tierBonus: 5, want: 10},
		{name: "no tier", userConcurrency: 5, tierBonus: 0, want: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubVIPBenefitRepo{concurrency: tc.tierBonus}
			svc := &APIKeyService{vipBenefitRepo: repo}

			snapshot := svc.snapshotFromAPIKey(context.Background(), vipConcurrencyTestAPIKey(tc.userConcurrency))

			require.NotNil(t, snapshot)
			require.Equal(t, tc.want, snapshot.User.Concurrency)
			require.Equal(t, 1, repo.calls, "the tier is resolved once per snapshot, not per request")
		})
	}
}

// A tier lookup that fails must leave the plain user limit in place: losing the
// perk is recoverable, refusing to build the snapshot is not.
func TestAPIKeyAuthSnapshotKeepsUserConcurrencyWhenVIPLookupFails(t *testing.T) {
	repo := &stubVIPBenefitRepo{concurrency: 20, err: errors.New("vip lookup unavailable")}
	svc := &APIKeyService{vipBenefitRepo: repo}

	snapshot := svc.snapshotFromAPIKey(context.Background(), vipConcurrencyTestAPIKey(5))

	require.NotNil(t, snapshot)
	require.Equal(t, 5, snapshot.User.Concurrency)
}

// No repo wired (e.g. a construction path that never sets it) must not panic.
func TestAPIKeyAuthSnapshotWithoutVIPRepoKeepsUserConcurrency(t *testing.T) {
	svc := &APIKeyService{}

	snapshot := svc.snapshotFromAPIKey(context.Background(), vipConcurrencyTestAPIKey(7))

	require.NotNil(t, snapshot)
	require.Equal(t, 7, snapshot.User.Concurrency)
}
