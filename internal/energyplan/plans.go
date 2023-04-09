package energyplan

import (
	"context"
	"fmt"

	"github.com/georgesolomos/enket/api/cdsenergy"
	"golang.org/x/exp/slog"
)

type PlanFetcher struct {
	logger *slog.Logger
	client *cdsenergy.ClientWithResponses
}

func NewPlanFetcher(logger *slog.Logger, provider string) (*PlanFetcher, error) {
	url := fmt.Sprintf("https://cdr.energymadeeasy.gov.au/%v/cds-au/v1", provider)
	c, err := cdsenergy.NewClientWithResponses(url)
	if err != nil {
		return nil, err
	}
	return &PlanFetcher{
		logger: logger,
		client: c,
	}, nil
}

func (p *PlanFetcher) FetchAllPlans() ([]cdsenergy.EnergyPlan, error) {
	fuelType := cdsenergy.ListPlansParamsFuelTypeELECTRICITY
	effective := cdsenergy.ListPlansParamsEffectiveCURRENT
	pageSize := 1000
	page := 1
	params := &cdsenergy.ListPlansParams{
		FuelType:  &fuelType,
		Effective: &effective,
		Page:      &page,
		PageSize:  &pageSize,
		XV:        "1",
	}
	totalPages := 1
	plans := make([]cdsenergy.EnergyPlan, 0)
	for page <= totalPages {
		resp, err := p.client.ListPlansWithResponse(context.Background(), params)
		if err != nil {
			return nil, err
		}
		plans = append(plans, resp.JSON200.Data.Plans...)
		totalPages = resp.JSON200.Meta.TotalPages
		page = page + 1
	}
	return plans, nil
}

func (p *PlanFetcher) FetchPlan(planID string) (*cdsenergy.EnergyPlanResponse, error) {
	params := &cdsenergy.GetPlanParams{
		XV: "1",
	}
	resp, err := p.client.GetPlanWithResponse(context.Background(), planID, params)
	if err != nil {
		return nil, err
	}
	return resp.JSON200, nil
}
