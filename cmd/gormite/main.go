package main

import (
	"context"
	"log"
	"os"
	"slices"

	"flag"
	"github.com/KoNekoD/gormite/pkg/runners"
)

func main() {
	scenario := runners.ScenarioTypeDiff
	args := os.Args[1:]
	if len(args) > 0 && slices.Contains([]string{runners.ScenarioTypeDiff, runners.ScenarioTypeValidate}, args[0]) {
		scenario = args[0]
		args = args[1:]
	}

	c := flag.NewFlagSet("Gormite CLI", flag.ExitOnError)
	ctx := context.Background()
	opts := runners.DiffRunnerOptions{Scenario: scenario}

	var checks []func()

	switch scenario {
	case runners.ScenarioTypeDiff:
		c.StringVar(&opts.Tool, "tool", "", "migration tool, allowed: migrate, goose")
		checks = append(
			checks, func() {
				if !slices.Contains([]string{"migrate", "goose"}, opts.Tool) {
					log.Fatal("invalid migration tool name")
				}
			},
		)
	}

	c.StringVar(&opts.Dsn, "dsn", "", "database connection string")
	c.StringVar(&opts.ConfigPath, "config", "gormite.yaml", "config file path")

	err := c.Parse(args)
	if err != nil {
		log.Fatal(err)
	}

	for _, check := range checks {
		check()
	}

	err = runners.NewDiffRunner(opts).Run(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
