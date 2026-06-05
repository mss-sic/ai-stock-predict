package service

import (
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/repository"
)

type BoardService struct {
	repo *repository.BoardRepo
}

func NewBoardService() *BoardService { return &BoardService{repo: &repository.BoardRepo{}} }

func (s *BoardService) GetToday() ([]model.AlgorithmPickDetail, error) {
	return s.repo.GetTodayBoard()
}

func (s *BoardService) GetByDate(date string) ([]model.AlgorithmPickDetail, error) {
	return s.repo.GetBoardByDate(date)
}

func (s *BoardService) GetHeatmap(from, to string) ([]model.AlgorithmPickDetail, error) {
	if from == "" { from = "2026-01-01" }
	if to == "" { to = "2099-01-01" }
	return s.repo.GetHeatmapData(from, to)
}

func (s *BoardService) GetStockHeatmap(code string) ([]model.AlgorithmPickDetail, error) {
	return s.repo.GetStockHeatmap(code)
}

func (s *BoardService) GetEnrichedHeatmap(from, to string) ([]model.HeatmapEnriched, error) {
	if from == "" { from = "2026-01-01" }
	if to == "" { to = "2099-01-01" }
	return s.repo.GetEnrichedHeatmap(from, to)
}
