package calculator

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/georgesolomos/enket/api/cdsenergy"
	"github.com/georgesolomos/enket/internal/nem12"
	"github.com/georgesolomos/enket/internal/util"
	"golang.org/x/exp/maps"
)

type Calculator struct {
	logger *slog.Logger
}

type Cost struct {
	// This will always be populated and will be an average monthly cost given all the usage data available
	AverageMonthly float64
	// Breaks down the cost into an average per month. Indexed with January being 0. How much of
	// this is populated depends on the usage data provided. If there is at least a year of usage,
	// all indices will have a value.
	AveragePerMonth []float64
}

func NewCalculator(logger *slog.Logger) *Calculator {
	return &Calculator{
		logger: logger,
	}
}

func (c *Calculator) CalculateMonthly(usage nem12.UsageData, plan *cdsenergy.EnergyPlanDetail) (Cost, error) {
	switch plan.ElectricityContract.PricingModel {
	case cdsenergy.EnergyPlanContractFullPricingModelSINGLERATE, cdsenergy.EnergyPlanContractFullPricingModelSINGLERATECONTLOAD:
		return c.calculateSingleRate(usage, plan)
	case cdsenergy.EnergyPlanContractFullPricingModelTIMEOFUSE, cdsenergy.EnergyPlanContractFullPricingModelTIMEOFUSECONTLOAD:
		return c.calculateTimeOfUse(usage, plan)
	default:
		return Cost{}, fmt.Errorf("unsupported pricing model %v", plan.ElectricityContract.PricingModel)
	}
}

func (c *Calculator) calculateSingleRate(usage nem12.UsageData, plan *cdsenergy.EnergyPlanDetail) (Cost, error) {
	cost := Cost{
		AverageMonthly:  0,
		AveragePerMonth: make([]float64, 12),
	}
	monthlyTotals := make([]float64, 12)
	monthlyReadings := make([]int, 12)
	for _, tariff := range plan.ElectricityContract.TariffPeriod {
		start, err := time.Parse("01-02", tariff.StartDate)
		if err != nil {
			return cost, err
		}
		end, err := time.Parse("01-02", tariff.EndDate)
		if err != nil {
			return cost, err
		}
		nmis := maps.Keys(usage)
		selectedNmi := nmis[0]
		if len(nmis) > 1 {
			c.logger.Warn("More than 1 NMI detected - the one with the most readings will be used")
			maxReadings := 0
			for _, nmi := range nmis {
				count := len(usage[nmi])
				if count > maxReadings {
					maxReadings = count
					selectedNmi = nmi
				}
			}
			c.logger.Info(fmt.Sprintf("Using NMI %v", selectedNmi))
		}
		dailyKWh := 0.0
		dailyCharge := 0.0
		monthlyCharge := 0.0
		readingsThisMonth := 0
		for _, data := range usage[selectedNmi][nem12.GeneralUsage] {
			// The plan's tarrif period ranges only have a day and month, so we convert our reading to
			// the same format so we can see if it's in the tariff period
			date := time.Date(0, data.StartTime.Month(), data.StartTime.Day(), 0, 0, 0, 0, time.UTC)
			if !util.InDateRange(start, end, date) {
				continue
			}
			if util.IsMidnight(data.StartTime) {
				supply, err := strconv.ParseFloat(*tariff.DailySupplyCharges, 64)
				if err != nil {
					return cost, fmt.Errorf("couldn't parse daily supply charge: %w", err)
				}
				// It's a new day so re-initialise the daily total with the supply charge and zero the kWh
				dailyCharge = util.WithGST(supply)
				dailyKWh = 0.0
			}
			dailyKWh = dailyKWh + data.EnergyKWh
			rate, err := getRate(dailyKWh, tariff.SingleRate.Rates)
			if err != nil {
				return cost, fmt.Errorf("couldn't get rate: %w", err)
			}
			dailyCharge = dailyCharge + (data.EnergyKWh * rate)
			if util.IsMidnight(data.EndTime) {
				if data.StartTime.Day() == 1 {
					// We're at the start of a new month
					readingsThisMonth = 0
					monthlyCharge = 0.0
				}
				// It's the end of the day, so update the monthly total with today's total
				monthlyCharge = monthlyCharge + dailyCharge
				readingsThisMonth = readingsThisMonth + 1
				if data.EndTime.Day() == 1 {
					// We're at the end of the month
					if readingsThisMonth < util.DaysInMonth(data.StartTime) {
						// If we have less than 2 weeks of readings, we discount the month completely.
						// There's not enough data to go on. If we have more, we extrapolate the rest.
						if readingsThisMonth < 14 {
							continue
						} else {
							dailyAvg := monthlyCharge / float64(readingsThisMonth)
							missingDays := util.DaysInMonth(data.StartTime) - readingsThisMonth
							monthlyCharge = monthlyCharge + (dailyAvg * float64(missingDays))
						}
					}
					monthlyTotals[int(data.StartTime.Month())-1] = monthlyTotals[int(data.StartTime.Month())-1] + monthlyCharge
					monthlyReadings[int(data.StartTime.Month())-1] = monthlyReadings[int(data.StartTime.Month())-1] + 1
				}
			}
		}
	}
	validMonthlyReadings := 0
	for i, total := range monthlyTotals {
		if monthlyReadings[i] != 0 {
			cost.AveragePerMonth[i] = total / float64(monthlyReadings[i])
			cost.AverageMonthly = cost.AverageMonthly + cost.AveragePerMonth[i]
			validMonthlyReadings = validMonthlyReadings + 1
		}
	}
	cost.AverageMonthly = cost.AverageMonthly / float64(validMonthlyReadings)
	return cost, nil
}

// Gets the correct single rate price based on the plan's rate brackets and how much energy we've
// currently used.
// Note: This only looks at the amount including the current reading. This means if the kWh used
// actually goes between the rate brackets, we won't calculate that properly. We should actually
// pass in the kWh used before this reading and after this reading into a function to calculate
// that for a perfectly accurate determination of the charge for this hour.
func getRate(kWhUsed float64, rates []struct {
	MeasureUnit *cdsenergy.EnergyPlanContractFullTariffPeriodSingleRateRatesMeasureUnit "json:\"measureUnit,omitempty\""
	UnitPrice   string                                                                  "json:\"unitPrice\""
	Volume      *float32                                                                "json:\"volume,omitempty\""
}) (float64, error) {
	for _, rate := range rates {
		if rate.Volume != nil {
			volume := float64(*rate.Volume)
			if kWhUsed < volume {
				price, err := strconv.ParseFloat(rate.UnitPrice, 64)
				if err != nil {
					return 0, fmt.Errorf("couldn't parse unit price: %w", err)
				}
				return util.WithGST(price), nil
			}
		} else {
			// If volume is nil, that indicates the rate for the "remaining" energy.
			// If we get to this point, we've used more than the non-nil ranges so we use this value.
			price, err := strconv.ParseFloat(rate.UnitPrice, 64)
			if err != nil {
				return 0, fmt.Errorf("couldn't parse unit price: %w", err)
			}
			return util.WithGST(price), nil
		}
	}
	// This should never happen, unless we've misinterpreted the possible values for the rates array
	// (which is always possible)
	return 0, errors.New("couldn't find a rate")
}

func (c *Calculator) calculateTimeOfUse(usage nem12.UsageData, plan *cdsenergy.EnergyPlanDetail) (Cost, error) {
	panic("unimplemented")
}
