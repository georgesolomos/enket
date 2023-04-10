package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/georgesolomos/enket/internal/calculator"
	"github.com/georgesolomos/enket/internal/energyplan"
	"github.com/georgesolomos/enket/internal/nem12"

	"golang.org/x/exp/slog"
)

func main() {
	slogOpts := slog.HandlerOptions{Level: slog.LevelInfo}.NewJSONHandler(os.Stdout)
	logger := slog.New(slogOpts)
	slog.SetDefault(logger)

	nem12Path := flag.String("nem12path", "", "The path to your NEM12 file")
	flag.Parse()
	if *nem12Path == "" {
		logger.Error("A NEM12 path must be provided")
		os.Exit(1)
	}

	nem12File, err := os.Open(*nem12Path)
	if err != nil {
		logger.Error("Could not open NEM12 file")
		os.Exit(1)
	}
	defer nem12File.Close()

	parser := nem12.NewParser(logger, nem12File)
	nem12Data, err := parser.Parse()
	if err != nil {
		logger.Error(err.Error())
	}
	for k := range *nem12Data {
		logger.Info(fmt.Sprintf("NMI %v parsed", k))
	}

	fetcher, err := energyplan.NewPlanFetcher(logger, "energy-locals")
	if err != nil {
		logger.Error(err.Error())
	}

	// plans, err := fetcher.FetchAllPlans()
	// if err != nil {
	// 	logger.Error(err.Error())
	// }

	plan, err := fetcher.FetchPlan("LCL526934MRE4@EME")
	if err != nil {
		logger.Error(err.Error())
	}

	calculator := calculator.NewCalculator(logger)
	costs, err := calculator.CalculateMonthly(nem12Data, plan)
	if err != nil {
		logger.Error(err.Error())
	}
	logger.Info(fmt.Sprintf("Costs: %v", costs))
}
