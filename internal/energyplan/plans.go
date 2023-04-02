package energyplan

import (
	"context"
	"fmt"

	"github.com/georgesolomos/enket/api/cdsenergy"
	"golang.org/x/exp/slog"
)

type PlanFetcher struct {
	logger *slog.Logger
}

func NewPlanFetcher(logger *slog.Logger) *PlanFetcher {
	return &PlanFetcher{
		logger: logger,
	}
}

func (p *PlanFetcher) FetchAllPlans() {

}

func (p *PlanFetcher) FetchPlan(provider, planID string) (*cdsenergy.EnergyPlanResponse, error) {
	url := fmt.Sprintf("https://cdr.energymadeeasy.gov.au/%v/cds-au/v1", provider)
	c, err := cdsenergy.NewClientWithResponses(url)
	if err != nil {
		return nil, err
	}
	params := &cdsenergy.GetPlanParams{
		XV: "1",
	}
	resp, err := c.GetPlanWithResponse(context.Background(), planID, params)
	if err != nil {
		return nil, err
	}
	return resp.JSON200, nil
}
