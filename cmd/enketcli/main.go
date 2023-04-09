package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/georgesolomos/enket/internal/energyplan"
	"github.com/georgesolomos/enket/internal/nem12"

	"golang.org/x/exp/slog"
)

func main() {
	start := time.Now()
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
	for k := range nem12Data {
		logger.Info(fmt.Sprintf("NMI %v parsed", k))
	}
	parsingDone := time.Now()

	fetcher, err := energyplan.NewPlanFetcher(logger, "energy-locals")
	if err != nil {
		logger.Error(err.Error())
	}

	plans, err := fetcher.FetchAllPlans()
	if err != nil {
		logger.Error(err.Error())
	}
	planListDone := time.Now()

	_, err = fetcher.FetchPlan(plans[0].PlanId)
	if err != nil {
		logger.Error(err.Error())
	}
	planDetailDone := time.Now()

	logger.Info(fmt.Sprintf("Fetched %v plans", strconv.Itoa(len(plans))))
	logger.Info(fmt.Sprintf("Parsing took %v, fetching all plans took %v, fetching plan details took %v",
		parsingDone.Sub(start), planListDone.Sub(parsingDone), planDetailDone.Sub(planListDone)))
}
