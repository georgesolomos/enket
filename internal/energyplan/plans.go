package energyplan

import "golang.org/x/exp/slog"

type PlanFetcher struct {
	logger *slog.Logger
}

func NewPlanFetcher(logger *slog.Logger) *PlanFetcher {
	return &PlanFetcher{
		logger: logger,
	}
}

func (p *PlanFetcher) Fetch() {

}
