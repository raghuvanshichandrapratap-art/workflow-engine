package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"workflow-engine/config"
	"workflow-engine/eval"
	"workflow-engine/services"
)

func main() {
	datasetFlag := flag.String("dataset", filepath.Join("eval", "dataset.json"),
		"comma-separated list of dataset JSON files to run")
	flag.Parse()

	if err := config.LoadEnv(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to load env:", err)
		os.Exit(1)
	}

	var cases []eval.TestCase
	for _, path := range strings.Split(*datasetFlag, ",") {
		tcs, err := eval.LoadDataset(strings.TrimSpace(path))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cases = append(cases, tcs...)
	}

	ai := services.NewAIService()
	engine := services.NewExecutionEngine()

	results := make([]eval.CaseResult, 0, len(cases))
	for i, tc := range cases {
		fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(cases), tc.ID, tc.Category)
		results = append(results, eval.RunCase(ai, engine, tc))
	}

	report := eval.BuildReport(results)
	markdown := eval.RenderMarkdown(report)

	runDir := filepath.Join("eval", "results", time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(runDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "failed to create results dir:", err)
		os.Exit(1)
	}

	if err := eval.WriteCases(filepath.Join(runDir, "cases.json"), results); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	if err := os.WriteFile(filepath.Join(runDir, "report.md"), []byte(markdown), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write report.md:", err)
	}

	fmt.Println()
	fmt.Println(markdown)
	fmt.Println("results written to", runDir)
}
