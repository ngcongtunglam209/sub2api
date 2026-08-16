package service

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// Trimmed copy of a real Vietcombank pXML.aspx response, including the leading
// comment and the thousands separators that make the amounts unparseable by
// strconv on their own.
const vcbSampleXML = `<!--For reference only. Only one request every 5 minutes!-->
<ExrateList>
  <DateTime>8/14/2026 10:20:00 AM</DateTime>
  <Exrate CurrencyCode="AUD" CurrencyName="AUSTRALIAN DOLLAR   " Buy="17,936.08" Transfer="18,117.25" Sell="18,697.70" />
  <Exrate CurrencyCode="CNY" CurrencyName="YUAN RENMINBI       " Buy="3,769.67" Transfer="3,807.75" Sell="3,930.25" />
  <Exrate CurrencyCode="THB" CurrencyName="THAILAND BAHT       " Buy="692.39" Transfer="769.32" Sell="801.95" />
  <Exrate CurrencyCode="USD" CurrencyName="US DOLLAR           " Buy="25,860.00" Transfer="25,890.00" Sell="26,270.00" />
  <Source>Joint Stock Commercial Bank for Foreign Trade of Vietnam - Vietcombank</Source>
</ExrateList>`

func TestParseVCBRate_Columns(t *testing.T) {
	for _, tc := range []struct {
		column string
		want   float64
	}{
		{VNDRateColumnBuy, 25860},
		{VNDRateColumnTransfer, 25890},
		{VNDRateColumnSell, 26270},
	} {
		got, err := parseVCBRate([]byte(vcbSampleXML), "USD", tc.column)
		require.NoError(t, err, tc.column)
		require.InDelta(t, tc.want, got, 0.001, tc.column)
	}
}

// An unrecognised column must not silently read a different one; sell is the
// documented default and the conservative side of the board for a seller.
func TestParseVCBRate_UnknownColumnFallsBackToSell(t *testing.T) {
	got, err := parseVCBRate([]byte(vcbSampleXML), "USD", "midpoint")
	require.NoError(t, err)
	require.InDelta(t, 26270, got, 0.001)
}

func TestParseVCBRate_CurrencyLookupIsCaseInsensitive(t *testing.T) {
	got, err := parseVCBRate([]byte(vcbSampleXML), "usd", VNDRateColumnSell)
	require.NoError(t, err)
	require.InDelta(t, 26270, got, 0.001)
}

func TestParseVCBRate_MissingCurrency(t *testing.T) {
	_, err := parseVCBRate([]byte(vcbSampleXML), "KRW", VNDRateColumnSell)
	require.Error(t, err)
	require.Contains(t, err.Error(), "KRW")
}

func TestParseVCBRate_MalformedDocument(t *testing.T) {
	_, err := parseVCBRate([]byte("not xml at all"), "USD", VNDRateColumnSell)
	require.Error(t, err)
}

// A currency present but with a blank column must fail rather than resolve to
// 0, which downstream means "conversion disabled, charge the plan price as-is".
func TestParseVCBRate_BlankColumnIsAnError(t *testing.T) {
	doc := `<ExrateList><Exrate CurrencyCode="USD" Buy="" Transfer="" Sell="" /></ExrateList>`
	_, err := parseVCBRate([]byte(doc), "USD", VNDRateColumnSell)
	require.Error(t, err)
}

func TestParseVCBAmount(t *testing.T) {
	got, err := parseVCBAmount(" 26,270.00 ")
	require.NoError(t, err)
	require.InDelta(t, 26270, got, 0.001)

	_, err = parseVCBAmount("")
	require.Error(t, err)

	_, err = parseVCBAmount("abc")
	require.Error(t, err)
}

func TestApplyVNDRateMargin(t *testing.T) {
	require.InDelta(t, 26270, applyVNDRateMargin(26270, 0), 0.001)
	require.InDelta(t, 26795.4, applyVNDRateMargin(26270, 2), 0.001)
	// Negative margins are ignored so a typo cannot undercut the board rate.
	require.InDelta(t, 26270, applyVNDRateMargin(26270, -5), 0.001)
}

// displayRateRepo is a minimal in-memory SettingRepository: the CNY display rate
// path only ever writes, so everything else is a stub.
type displayRateRepo struct {
	values  map[string]string
	setKeys []string
}

func newDisplayRateRepo() *displayRateRepo {
	return &displayRateRepo{values: map[string]string{}}
}

func (r *displayRateRepo) Get(context.Context, string) (*Setting, error) { return nil, nil }

func (r *displayRateRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func (r *displayRateRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	r.setKeys = append(r.setKeys, key)
	return nil
}

func (r *displayRateRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *displayRateRepo) SetMultiple(context.Context, map[string]string) error { return nil }

func (r *displayRateRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *displayRateRepo) Delete(context.Context, string) error { return nil }

func newDisplayRateService(repo SettingRepository, currency string, margin float64) *VNDRateService {
	cfg := &config.Config{}
	cfg.VNDRate.Enabled = true
	cfg.VNDRate.URL = "https://portal.vietcombank.com.vn/x"
	cfg.VNDRate.Currency = currency
	cfg.VNDRate.Column = VNDRateColumnSell
	cfg.VNDRate.MarginPercent = margin
	return NewVNDRateService(cfg, repo)
}

func TestSyncDisplayCNYRate_StoresCrossRateFromTheSameBoard(t *testing.T) {
	repo := newDisplayRateRepo()
	svc := newDisplayRateService(repo, DisplayCurrencyUSD, 0)

	// 26,270.00 VND per USD over 3,930.25 VND per CNY, both from the sell column.
	svc.syncDisplayCNYRate(context.Background(), []byte(vcbSampleXML), DisplayCurrencyUSD, 26270.00)

	stored, ok := repo.values[SettingDisplayUSDToCNYRate]
	require.True(t, ok, "expected the CNY display rate to be persisted")

	got, err := strconv.ParseFloat(stored, 64)
	require.NoError(t, err)
	require.InDelta(t, 26270.00/3930.25, got, 1e-4)
}

func TestSyncDisplayCNYRate_AppliesTheConfiguredMargin(t *testing.T) {
	repo := newDisplayRateRepo()
	svc := newDisplayRateService(repo, DisplayCurrencyUSD, 2)

	svc.syncDisplayCNYRate(context.Background(), []byte(vcbSampleXML), DisplayCurrencyUSD, 26270.00)

	got, err := strconv.ParseFloat(repo.values[SettingDisplayUSDToCNYRate], 64)
	require.NoError(t, err)
	require.InDelta(t, (26270.00/3930.25)*1.02, got, 1e-4)
}

// Dividing a non-dollar board quote by the yuan quote yields something that is
// not USD to CNY at all, so the stored rate is left alone instead.
func TestSyncDisplayCNYRate_SkippedWhenTheBaseCurrencyIsNotUSD(t *testing.T) {
	repo := newDisplayRateRepo()
	svc := newDisplayRateService(repo, "AUD", 0)

	svc.syncDisplayCNYRate(context.Background(), []byte(vcbSampleXML), "AUD", 18697.70)

	require.Empty(t, repo.setKeys, "no setting should be written for a non-USD base currency")
}

// A board without a yuan row must not fail the dong sync that shares the fetch.
func TestSyncDisplayCNYRate_MissingYuanRowKeepsStoredValue(t *testing.T) {
	repo := newDisplayRateRepo()
	repo.values[SettingDisplayUSDToCNYRate] = "7.0000"
	svc := newDisplayRateService(repo, DisplayCurrencyUSD, 0)

	const noYuan = `<ExrateList>
  <Exrate CurrencyCode="USD" CurrencyName="US DOLLAR" Buy="25,860.00" Transfer="25,890.00" Sell="26,270.00" />
</ExrateList>`

	svc.syncDisplayCNYRate(context.Background(), []byte(noYuan), DisplayCurrencyUSD, 26270.00)

	require.Empty(t, repo.setKeys, "a missing CNY row must not overwrite the stored rate")
	require.Equal(t, "7.0000", repo.values[SettingDisplayUSDToCNYRate])
}
