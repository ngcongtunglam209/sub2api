//go:build unit

package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type recordingVIPSpendRepo struct {
	calls []float64
	err   error
}

func (r *recordingVIPSpendRepo) AddVIPSpend(_ context.Context, _ int64, amountUSD float64) error {
	if r.err != nil {
		return r.err
	}
	r.calls = append(r.calls, amountUSD)
	return nil
}

func newVIPSpendFulfillmentClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:payment_vip_spend?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newRechargingOrder(t *testing.T, client *dbent.Client, orderType string, amount float64) *dbent.PaymentOrder {
	t.Helper()
	ctx := context.Background()
	user, err := client.User.Create().
		SetEmail("vip-fulfillment@example.com").
		SetPasswordHash("hash").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName("vip-fulfillment").
		SetAmount(amount).
		SetPayAmount(amount).
		SetFeeRate(0).
		SetRechargeCode("code-vip-spend").
		SetOutTradeNo("sub2_vip_spend").
		SetPaymentType("sepay").
		SetPaymentTradeNo("trade-vip-spend").
		SetOrderType(orderType).
		SetStatus(OrderStatusRecharging).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("pay.example.com").
		Save(ctx)
	require.NoError(t, err)
	return order
}

// A completed order is the only thing that raises a VIP tier, and fulfillment
// is retried on every provider re-notification, so the credit has to happen
// exactly once per order.
func TestMarkCompleted_CreditsVIPSpendOncePerOrder(t *testing.T) {
	ctx := context.Background()
	client := newVIPSpendFulfillmentClient(t)
	order := newRechargingOrder(t, client, payment.OrderTypeBalance, 42.5)

	spendRepo := &recordingVIPSpendRepo{}
	svc := &PaymentService{entClient: client, vipSpendRepo: spendRepo}
	lease := &paymentFulfillmentLease{version: order.UpdatedAt}

	require.NoError(t, svc.markCompleted(ctx, order, lease, "RECHARGE_SUCCESS"))
	require.Equal(t, []float64{42.5}, spendRepo.calls)

	// Same lease replayed: the status CAS no longer matches, and the second
	// attempt must return without crediting the order again.
	require.NoError(t, svc.markCompleted(ctx, order, lease, "RECHARGE_SUCCESS"))
	require.Equal(t, []float64{42.5}, spendRepo.calls, "replayed fulfillment must not double count")

	stored, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, stored.Status)
}

// Subscription purchases never touch the balance, so they are invisible to
// total_recharged. They are real money and must count towards the tier.
func TestMarkCompleted_CreditsVIPSpendForSubscriptionOrders(t *testing.T) {
	ctx := context.Background()
	client := newVIPSpendFulfillmentClient(t)
	order := newRechargingOrder(t, client, payment.OrderTypeSubscription, 100)

	spendRepo := &recordingVIPSpendRepo{}
	svc := &PaymentService{entClient: client, vipSpendRepo: spendRepo}

	require.NoError(t, svc.markCompleted(ctx, order, &paymentFulfillmentLease{version: order.UpdatedAt}, "SUBSCRIPTION_SUCCESS"))
	require.Equal(t, []float64{100}, spendRepo.calls)
}

// The spend write shares the completion transaction. If it fails, the order
// must stay in recharging so the next retry redoes both — a completed order
// with no spend recorded can never be repaired, since the CAS will not fire
// again.
func TestMarkCompleted_SpendFailureRollsBackCompletion(t *testing.T) {
	ctx := context.Background()
	client := newVIPSpendFulfillmentClient(t)
	order := newRechargingOrder(t, client, payment.OrderTypeBalance, 30)

	spendRepo := &recordingVIPSpendRepo{err: context.DeadlineExceeded}
	svc := &PaymentService{entClient: client, vipSpendRepo: spendRepo}

	err := svc.markCompleted(ctx, order, &paymentFulfillmentLease{version: order.UpdatedAt}, "RECHARGE_SUCCESS")
	require.Error(t, err)

	stored, getErr := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, getErr)
	require.Equal(t, OrderStatusRecharging, stored.Status)
}

// Deployments that wire a user repository without the VIP capability keep
// fulfilling orders; the tier feature is simply inert.
func TestMarkCompleted_WithoutVIPSpendRepo(t *testing.T) {
	ctx := context.Background()
	client := newVIPSpendFulfillmentClient(t)
	order := newRechargingOrder(t, client, payment.OrderTypeBalance, 15)

	svc := &PaymentService{entClient: client}
	require.NoError(t, svc.markCompleted(ctx, order, &paymentFulfillmentLease{version: order.UpdatedAt}, "RECHARGE_SUCCESS"))

	stored, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, stored.Status)
}
