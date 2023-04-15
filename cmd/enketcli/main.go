package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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
		os.Exit(1)
	}

	fetcher, err := energyplan.NewPlanFetcher(logger, "origin")
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// plans, err := fetcher.FetchAllPlans()
	// if err != nil {
	// 	logger.Error(err.Error())
	// }

	plan, err := fetcher.FetchPlan("ORI431087MRE6@EME")
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	calculator := calculator.NewCalculator(logger)
	cost, err := calculator.CalculateMonthly(nem12Data, plan)
	if err != nil {
		logger.Error(err.Error())
	}

	var log strings.Builder
	for _, c := range cost.AveragePerMonth {
		log.WriteString(fmt.Sprintf("$%.2f, ", float64(c)/100))
	}
	logger.Info(log.String())
}
