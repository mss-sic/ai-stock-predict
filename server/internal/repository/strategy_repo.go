package repository

import (
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// StrategyRepo provides MySQL-based data access for strategies and conditions.
type StrategyRepo struct{}

// NewStrategyRepo creates a new StrategyRepo.
func NewStrategyRepo() *StrategyRepo { return &StrategyRepo{} }

// ListByUser returns all strategies for a user, ordered by sort_order.
func (r *StrategyRepo) ListByUser(uid uint) ([]model.Strategy, error) {
	var strategies []model.Strategy
	err := db.MySQL.Where("user_id = ?", uid).Order("sort_order ASC, id ASC").Find(&strategies).Error
	return strategies, err
}

// GetActivePkStrategyIDs returns strategy IDs that are in active PK events.
func (r *StrategyRepo) GetActivePkStrategyIDs(uid uint) ([]uint, error) {
	var ids []uint
	err := db.MySQL.Model(&model.PkEntry{}).
		Joins("JOIN pk_events ON pk_events.id = pk_entries.event_id").
		Where("pk_entries.user_id = ? AND pk_events.status IN (?)", uid, []string{"enrolling", "running"}).
		Pluck("pk_entries.strategy_id", &ids).Error
	return ids, err
}

// GetByID returns a strategy by ID and user ID.
func (r *StrategyRepo) GetByID(id int, uid uint) (*model.Strategy, error) {
	var s model.Strategy
	err := db.MySQL.Where("id = ? AND user_id = ?", id, uid).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Create inserts a new strategy.
func (r *StrategyRepo) Create(s *model.Strategy) error {
	return db.MySQL.Create(s).Error
}

// CountByUser returns the number of strategies for a user.
func (r *StrategyRepo) CountByUser(uid uint) (int64, error) {
	var count int64
	err := db.MySQL.Model(&model.Strategy{}).Where("user_id = ?", uid).Count(&count).Error
	return count, err
}

// UpdateFields updates specific fields of a strategy.
func (r *StrategyRepo) UpdateFields(id int, uid uint, updates map[string]interface{}) error {
	return db.MySQL.Model(&model.Strategy{}).Where("id = ? AND user_id = ?", id, uid).Updates(updates).Error
}

// Delete removes a strategy and its conditions.
func (r *StrategyRepo) Delete(id int, uid uint) error {
	if err := db.MySQL.Where("id = ? AND user_id = ?", id, uid).Delete(&model.Strategy{}).Error; err != nil {
		return err
	}
	return db.MySQL.Where("strategy_id = ?", id).Delete(&model.StrategyCondition{}).Error
}

// ListConditions returns conditions for a strategy.
func (r *StrategyRepo) ListConditions(strategyID uint) ([]model.StrategyCondition, error) {
	var conds []model.StrategyCondition
	err := db.MySQL.Where("strategy_id = ?", strategyID).Find(&conds).Error
	return conds, err
}

// SaveConditions replaces all conditions for a strategy in a transaction.
func (r *StrategyRepo) SaveConditions(strategyID uint, conds []model.StrategyCondition) error {
	tx := db.MySQL.Begin()
	if err := tx.Where("strategy_id = ?", strategyID).Delete(&model.StrategyCondition{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for i := range conds {
		conds[i].StrategyID = strategyID
	}
	if len(conds) > 0 {
		if err := tx.Create(&conds).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}
