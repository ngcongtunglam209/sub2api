package repository

import (
	"context"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/resellerdomain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type resellerDomainRepository struct {
	client *dbent.Client
}

func NewResellerDomainRepository(client *dbent.Client) service.ResellerDomainRepository {
	return &resellerDomainRepository{client: client}
}

// ListActiveDomains returns the hostname, its row id and its branding override.
//
// Still an explicit column list rather than whole rows: notes and timestamps
// are never read on the request path, and this is the one query that sits in
// front of it. The branding columns are here because the caller renders them on
// the same request it uses the hostname to admit — fetching them separately
// would be a second round trip for a row already in memory.
func (r *resellerDomainRepository) ListActiveDomains(ctx context.Context) ([]service.ActiveResellerDomain, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.ResellerDomain.Query().
		Where(resellerdomain.StatusEQ(service.ResellerDomainStatusActive)).
		Select(
			resellerdomain.FieldID,
			resellerdomain.FieldDomain,
			resellerdomain.FieldSiteName,
			resellerdomain.FieldSiteLogo,
			resellerdomain.FieldSiteSubtitle,
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active reseller domains: %w", err)
	}

	out := make([]service.ActiveResellerDomain, 0, len(rows))
	for _, row := range rows {
		out = append(out, service.ActiveResellerDomain{
			ID:           row.ID,
			Domain:       row.Domain,
			SiteName:     row.SiteName,
			SiteLogo:     row.SiteLogo,
			SiteSubtitle: row.SiteSubtitle,
		})
	}
	return out, nil
}

// UpdateBranding writes only the fields the caller addressed.
//
// An empty value clears the column to NULL rather than storing "": NULL and ""
// mean the same thing to every reader — "use the global setting" — and keeping
// one spelling of "unset" in the table keeps that equivalence from having to be
// rediscovered by every future query.
func (r *resellerDomainRepository) UpdateBranding(ctx context.Context, id int64, update service.ResellerDomainBrandingUpdate) error {
	if update.IsEmpty() {
		return nil
	}

	client := clientFromContext(ctx, r.client)
	builder := client.ResellerDomain.UpdateOneID(id)

	if update.SiteName != nil {
		if *update.SiteName == "" {
			builder = builder.ClearSiteName()
		} else {
			builder = builder.SetSiteName(*update.SiteName)
		}
	}
	if update.SiteLogo != nil {
		if *update.SiteLogo == "" {
			builder = builder.ClearSiteLogo()
		} else {
			builder = builder.SetSiteLogo(*update.SiteLogo)
		}
	}
	if update.SiteSubtitle != nil {
		if *update.SiteSubtitle == "" {
			builder = builder.ClearSiteSubtitle()
		} else {
			builder = builder.SetSiteSubtitle(*update.SiteSubtitle)
		}
	}

	if err := builder.Exec(ctx); err != nil {
		return fmt.Errorf("update reseller domain %d branding: %w", id, err)
	}
	return nil
}

func (r *resellerDomainRepository) Create(ctx context.Context, domain *service.ResellerDomain) (*service.ResellerDomain, error) {
	client := clientFromContext(ctx, r.client)
	created, err := client.ResellerDomain.Create().
		SetDomain(domain.Domain).
		SetUserID(domain.UserID).
		SetStatus(domain.Status).
		SetNotes(domain.Notes).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create reseller domain: %w", err)
	}
	return resellerDomainEntityToService(created), nil
}

func (r *resellerDomainRepository) List(ctx context.Context) ([]*service.ResellerDomain, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.ResellerDomain.Query().
		Order(dbent.Desc(resellerdomain.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list reseller domains: %w", err)
	}
	return resellerDomainEntitiesToService(rows), nil
}

func (r *resellerDomainRepository) ListByUser(ctx context.Context, userID int64) ([]*service.ResellerDomain, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.ResellerDomain.Query().
		Where(resellerdomain.UserIDEQ(userID)).
		Order(dbent.Desc(resellerdomain.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list reseller domains for user %d: %w", userID, err)
	}
	return resellerDomainEntitiesToService(rows), nil
}

func (r *resellerDomainRepository) SetStatus(ctx context.Context, id int64, status string) error {
	client := clientFromContext(ctx, r.client)
	if err := client.ResellerDomain.UpdateOneID(id).SetStatus(status).Exec(ctx); err != nil {
		return fmt.Errorf("set reseller domain %d status: %w", id, err)
	}
	return nil
}

func (r *resellerDomainRepository) Delete(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	if err := client.ResellerDomain.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("delete reseller domain %d: %w", id, err)
	}
	return nil
}

func resellerDomainEntityToService(row *dbent.ResellerDomain) *service.ResellerDomain {
	if row == nil {
		return nil
	}
	return &service.ResellerDomain{
		ID:           row.ID,
		Domain:       row.Domain,
		UserID:       row.UserID,
		Status:       row.Status,
		Notes:        row.Notes,
		SiteName:     row.SiteName,
		SiteLogo:     row.SiteLogo,
		SiteSubtitle: row.SiteSubtitle,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func resellerDomainEntitiesToService(rows []*dbent.ResellerDomain) []*service.ResellerDomain {
	result := make([]*service.ResellerDomain, 0, len(rows))
	for _, row := range rows {
		result = append(result, resellerDomainEntityToService(row))
	}
	return result
}
