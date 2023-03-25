package main

import (
	"flag"
	"os"

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
	parser.Parse()
}
