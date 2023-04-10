package calculator

import (
	"fmt"
	"time"

	"github.com/georgesolomos/enket/api/cdsenergy"
	"github.com/georgesolomos/enket/internal/nem12"
	"golang.org/x/exp/slog"
)

type Calculator struct {
	logger *slog.Logger
}

type Cost struct {
	// This will always be populated and will be an average monthly cost given all the usage data available
	AverageMonthly int
	// Breaks down the cost into an average per month. Indexed with January being 0. How much of
	// this is populated depends on the usage data provided. If there is at least a year of usage,
	// all indices will have a value.
	AveragePerMonth []int
}

func NewCalculator(logger *slog.Logger) *Calculator {
	return &Calculator{
		logger: logger,
	}
}

func (c *Calculator) CalculateMonthly(usage *nem12.UsageData, plan *cdsenergy.EnergyPlanDetail) (Cost, error) {
	switch plan.ElectricityContract.PricingModel {
	case cdsenergy.EnergyPlanContractFullPricingModelSINGLERATE, cdsenergy.EnergyPlanContractFullPricingModelSINGLERATECONTLOAD:
		return c.calculateSingleRate(usage, plan)
	case cdsenergy.EnergyPlanContractFullPricingModelTIMEOFUSE, cdsenergy.EnergyPlanContractFullPricingModelTIMEOFUSECONTLOAD:
		return c.calculateTimeOfUse(usage, plan)
	default:
		return Cost{}, fmt.Errorf("unsupported pricing model %v", plan.ElectricityContract.PricingModel)
	}
}

func (c *Calculator) calculateSingleRate(usage *nem12.UsageData, plan *cdsenergy.EnergyPlanDetail) (Cost, error) {
	cost := Cost{}
	for _, tariff := range plan.ElectricityContract.TariffPeriod {
		start, err := time.Parse("01-02", tariff.StartDate)
		if err != nil {
			return cost, err
		}
		end, err := time.Parse("01-02", tariff.EndDate)
		if err != nil {
			return cost, err
		}
		c.logger.Info(fmt.Sprintf("%v - %v", start, end))
	}
	return cost, nil
}

func (c *Calculator) calculateTimeOfUse(usage *nem12.UsageData, plan *cdsenergy.EnergyPlanDetail) (Cost, error) {
	panic("unimplemented")
}
