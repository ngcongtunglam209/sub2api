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

// ListActiveDomains returns just the hostname column.
//
// Only the strings are selected: the caller builds a set for membership tests
// and never reads another field, so pulling whole rows on every cache refresh
// would be waste on the one query that sits in front of the request path.
func (r *resellerDomainRepository) ListActiveDomains(ctx context.Context) ([]string, error) {
	client := clientFromContext(ctx, r.client)
	domains, err := client.ResellerDomain.Query().
		Where(resellerdomain.StatusEQ(service.ResellerDomainStatusActive)).
		Select(resellerdomain.FieldDomain).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active reseller domains: %w", err)
	}
	return domains, nil
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
		ID:        row.ID,
		Domain:    row.Domain,
		UserID:    row.UserID,
		Status:    row.Status,
		Notes:     row.Notes,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func resellerDomainEntitiesToService(rows []*dbent.ResellerDomain) []*service.ResellerDomain {
	result := make([]*service.ResellerDomain, 0, len(rows))
	for _, row := range rows {
		result = append(result, resellerDomainEntityToService(row))
	}
	return result
}
