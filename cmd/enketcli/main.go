package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
	for k := range nem12Data {
		logger.Info(fmt.Sprintf("%v", k))
	}

	fetcher := energyplan.NewPlanFetcher(logger)
	plan, err := fetcher.FetchPlan("amber", "AMB397318MR@VEC")
	if err != nil {
		logger.Error(err.Error())
	}
	js, _ := json.Marshal(plan)
	logger.Info(string(js))
}
