package energyplan

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/georgesolomos/enket/api/cdsenergy"
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

func (p *PlanFetcher) FetchAllPlans() ([]*cdsenergy.EnergyPlan, error) {
	var checkStatusCode = func(resp *cdsenergy.ListPlansResponse) error {
		switch resp.StatusCode() {
		case 200:
			return nil
		case 400:
			p.logger.Error("FetchAllPlans bad request", slog.Any("errors", resp.JSON400.Errors))
			return errors.New("bad request")
		case 406:
			p.logger.Error("FetchAllPlans not acceptable", slog.Any("errors", resp.JSON406.Errors))
			return errors.New("not acceptable")
		case 422:
			p.logger.Error("FetchAllPlans unprocessable entity", slog.Any("errors", resp.JSON422.Errors))
			return errors.New("unprocessable entity")
		default:
			p.logger.Error(fmt.Sprintf("FetchAllPlans unrecognised error code %v", resp.HTTPResponse.StatusCode))
			return nil
		}
	}

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
	plans := make([]*cdsenergy.EnergyPlan, 0)
	for page <= totalPages {
		resp, err := p.client.ListPlansWithResponse(context.Background(), params)
		if err != nil {
			return nil, err
		}
		err = checkStatusCode(resp)
		if err != nil {
			return nil, err
		}
		for _, plan := range resp.JSON200.Data.Plans {
			planCpy := plan
			plans = append(plans, &planCpy)
		}
		totalPages = resp.JSON200.Meta.TotalPages
		page = page + 1
	}
	return plans, nil
}

func (p *PlanFetcher) FetchPlan(planID string) (*cdsenergy.EnergyPlanDetail, error) {
	params := &cdsenergy.GetPlanParams{
		XV: "1",
	}
	resp, err := p.client.GetPlanWithResponse(context.Background(), planID, params)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return &resp.JSON200.Data, nil
	case 400:
		p.logger.Error("FetchPlan bad request", slog.Any("errors", resp.JSON400.Errors))
		return nil, errors.New("bad request")
	case 404:
		p.logger.Error("FetchPlan not found", slog.Any("errors", resp.JSON404.Errors))
		return nil, errors.New("not found")
	case 406:
		p.logger.Error("FetchPlan not acceptable", slog.Any("errors", resp.JSON406.Errors))
		return nil, errors.New("not acceptable")
	default:
		p.logger.Error(fmt.Sprintf("FetchPlan unrecognised error code %v", resp.HTTPResponse.StatusCode))
		return nil, nil
	}
}
