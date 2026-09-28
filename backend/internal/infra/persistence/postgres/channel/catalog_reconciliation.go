package channel

import (
	"context"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm/clause"
)

func (r *Repo) ApplyUpstreamModelCatalogChanges(
	ctx context.Context,
	upstreamID uint,
	input repository.ApplyUpstreamModelCatalogChangesInput,
) (int64, error) {
	if upstreamID == 0 {
		return 0, repository.ErrInvalidInput
	}

	createdRows := make([]model.LLMUpstreamModel, 0, len(input.Create))
	for i := range input.Create {
		item := input.Create[i]
		if item.ID != 0 || item.UpstreamID != upstreamID || strings.TrimSpace(item.UpstreamModelName) == "" || strings.TrimSpace(item.BindingCode) == "" {
			return 0, repository.ErrInvalidInput
		}
		createdRows = append(createdRows, toUpstreamModelModel(&item))
	}

	now := time.Now()
	updatedRows := make([]model.LLMUpstreamModel, 0, len(input.Update))
	for i := range input.Update {
		item := input.Update[i]
		if item.ID == 0 || item.UpstreamID != upstreamID || strings.TrimSpace(item.UpstreamModelName) == "" || strings.TrimSpace(item.BindingCode) == "" {
			return 0, repository.ErrInvalidInput
		}
		entity := toUpstreamModelModel(&item)
		entity.ID = item.ID
		entity.CreatedAt = item.CreatedAt
		entity.UpdatedAt = now
		updatedRows = append(updatedRows, entity)
	}

	db := r.db.WithContext(ctx)
	if len(createdRows) > 0 {
		if err := db.CreateInBatches(&createdRows, 200).Error; err != nil {
			return 0, dberror.Translate(err)
		}
	}
	if len(updatedRows) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"binding_code",
				"upstream_model_name",
				"vendor",
				"icon",
				"suggested_protocol",
				"kinds_json",
				"status",
				"source",
				"last_synced_at",
				"raw_json",
				"updated_at",
			}),
		}).CreateInBatches(&updatedRows, 200).Error; err != nil {
			return 0, dberror.Translate(err)
		}
	}

	uniqueInactiveIDs := make([]uint, 0, len(input.InactivateIDs))
	seenInactiveIDs := make(map[uint]struct{}, len(input.InactivateIDs))
	for _, id := range input.InactivateIDs {
		if id == 0 {
			return 0, repository.ErrInvalidInput
		}
		if _, exists := seenInactiveIDs[id]; exists {
			continue
		}
		seenInactiveIDs[id] = struct{}{}
		uniqueInactiveIDs = append(uniqueInactiveIDs, id)
	}

	var inactivated int64
	for start := 0; start < len(uniqueInactiveIDs); start += 200 {
		end := min(start+200, len(uniqueInactiveIDs))
		result := db.Model(&model.LLMUpstreamModel{}).
			Where("upstream_id = ? AND id IN ? AND source IN ? AND status = ?", upstreamID, uniqueInactiveIDs[start:end], []string{"sync", "import"}, "active").
			Update("status", "inactive")
		if result.Error != nil {
			return 0, dberror.Translate(result.Error)
		}
		inactivated += result.RowsAffected
	}
	return inactivated, nil
}

func (r *Repo) GetActiveRoutableModelKindsJSON(ctx context.Context, platformModelName string) (string, bool, error) {
	var result struct {
		KindsJSON string
	}
	dbResult := r.db.WithContext(ctx).
		Table("llm_platform_models AS pm").
		Select("pm.kinds_json").
		Joins("JOIN llm_model_routes AS r ON r.platform_model_id = pm.id AND r.status = ?", "active").
		Joins("JOIN llm_upstream_models AS um ON um.id = r.upstream_model_id AND um.status = ?", "active").
		Joins("JOIN llm_upstreams AS u ON u.id = um.upstream_id AND u.status = ?", "active").
		Where("pm.name = ? AND pm.status = ?", strings.TrimSpace(platformModelName), "active").
		Limit(1).
		Scan(&result)
	if dbResult.Error != nil {
		return "", false, dberror.Translate(dbResult.Error)
	}
	if dbResult.RowsAffected == 0 {
		return "", false, nil
	}
	return result.KindsJSON, true, nil
}

func (r *Repo) ListManagedUpstreamModels(ctx context.Context, upstreamID uint) ([]domainchannel.UpstreamModel, error) {
	items := make([]model.LLMUpstreamModel, 0)
	if err := r.db.WithContext(ctx).
		Where("upstream_id = ? AND source IN ?", upstreamID, []string{"sync", "import"}).
		Order("upstream_model_name ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	result := make([]domainchannel.UpstreamModel, 0, len(items))
	for _, item := range items {
		result = append(result, toUpstreamModelDomain(item))
	}
	return result, nil
}
