package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"guarantee-agent/internal/config"
	"guarantee-agent/internal/env"
	"guarantee-agent/internal/logging"
	"guarantee-agent/internal/plan"
)

// newPlanCommand 构建 `autoqa plan` 命令:探索目标页面并生成 Markdown 用例(自检后写入 specs/)。
func newPlanCommand() *cobra.Command {
	var url string
	var headless bool
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Explore a page with the Eino agent and generate a Markdown spec",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := runPlan(context.Background(), url, headless)
			if err != nil {
				return exitError{ExitRuntime, err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "explored %s; generated %d spec(s) under specs/\n", res.URL, len(res.Specs))
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "page URL to explore (defaults to AUTOQA_BASE_URL / config plan.baseUrl)")
	cmd.Flags().BoolVar(&headless, "headless", true, "run browser headless")
	return cmd
}

// newPlanExploreCommand 构建 `autoqa plan-explore`:仅探索,写出 explore summary。
func newPlanExploreCommand() *cobra.Command {
	var url string
	var headless bool
	cmd := &cobra.Command{
		Use:   "plan-explore",
		Short: "Explore a page and write the exploration summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := runPlanExplore(context.Background(), url, headless)
			if err != nil {
				return exitError{ExitRuntime, err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "explored %s; summary written under .autoqa/runs/\n", res.URL)
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "page URL to explore")
	cmd.Flags().BoolVar(&headless, "headless", true, "run browser headless")
	return cmd
}

// newPlanGenerateCommand 构建 `autoqa plan-generate`:从已有 explore summary 生成用例。
func newPlanGenerateCommand() *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "plan-generate",
		Short: "Generate a Markdown spec from an exploration summary JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := runPlanGenerate(context.Background(), from)
			if err != nil {
				return exitError{ExitRuntime, err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "generated %d spec(s): %v\n", len(paths), paths)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "path to explore summary JSON (defaults to latest plan-explore run)")
	return cmd
}

// planResult 是一次 plan 流程的汇总结果。
type planResult struct {
	URL   string
	Specs []string
}

// runPlan 执行完整 plan:加载配置/env → 探索 → 生成用例 → 写出 explore summary。
func runPlan(ctx context.Context, url string, headless bool) (planResult, error) {
	cfg, _, runDir, err := planSetup(url)
	if err != nil {
		return planResult{}, err
	}
	exploreDir := filepath.Join(runDir, "plan-explore")
	if err := os.MkdirAll(exploreDir, 0755); err != nil {
		return planResult{}, err
	}
	logger, err := logging.New(runDir, false)
	if err != nil {
		return planResult{}, err
	}
	defer logger.Close()

	exp, err := plan.Explore(ctx, cfg, cfg.Plan.BaseURL, exploreDir, headless, logger)
	if err != nil {
		return planResult{}, err
	}
	summaryPath := filepath.Join(exploreDir, "summary.json")
	if err := writeJSON(summaryPath, exp); err != nil {
		return planResult{}, err
	}
	specsDir := filepath.Join(cwd(), "specs")
	paths, err := plan.GenerateSpecs(ctx, cfg, exp, specsDir, logger)
	if err != nil {
		return planResult{}, err
	}
	return planResult{URL: exp.URL, Specs: paths}, nil
}

// runPlanExplore 仅探索并写出 summary。
func runPlanExplore(ctx context.Context, url string, headless bool) (planResult, error) {
	cfg, _, runDir, err := planSetup(url)
	if err != nil {
		return planResult{}, err
	}
	exploreDir := filepath.Join(runDir, "plan-explore")
	if err := os.MkdirAll(exploreDir, 0755); err != nil {
		return planResult{}, err
	}
	logger, err := logging.New(runDir, false)
	if err != nil {
		return planResult{}, err
	}
	defer logger.Close()
	exp, err := plan.Explore(ctx, cfg, cfg.Plan.BaseURL, exploreDir, headless, logger)
	if err != nil {
		return planResult{}, err
	}
	if err := writeJSON(filepath.Join(exploreDir, "summary.json"), exp); err != nil {
		return planResult{}, err
	}
	return planResult{URL: exp.URL}, nil
}

// runPlanGenerate 从 summary JSON 生成用例;from 为空时取最近一次 plan-explore 的 summary。
func runPlanGenerate(ctx context.Context, from string) ([]string, error) {
	cfg, _, runDir, err := planSetup("")
	if err != nil {
		return nil, err
	}
	if from == "" {
		from = filepath.Join(runDir, "plan-explore", "summary.json")
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return nil, fmt.Errorf("read explore summary %s: %w", from, err)
	}
	var exp plan.ExploreResult
	if err := json.Unmarshal(data, &exp); err != nil {
		return nil, fmt.Errorf("parse explore summary: %w", err)
	}
	logger, err := logging.New(runDir, false)
	if err != nil {
		return nil, err
	}
	defer logger.Close()
	specsDir := filepath.Join(cwd(), "specs")
	return plan.GenerateSpecs(ctx, cfg, exp, specsDir, logger)
}

// planSetup 完成配置/env 加载、URL 解析、run 目录与 logger 路径准备,供 plan 子命令复用。
func planSetup(url string) (config.AutoqaConfig, string, string, error) {
	root := cwd()
	if _, err := env.Load(root, ""); err != nil {
		return config.AutoqaConfig{}, "", "", err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return config.AutoqaConfig{}, "", "", err
	}
	baseURL := url
	if baseURL == "" {
		baseURL = os.Getenv("AUTOQA_BASE_URL")
	}
	if baseURL == "" {
		baseURL = cfg.Plan.BaseURL
	}
	if baseURL == "" {
		return config.AutoqaConfig{}, "", "", fmt.Errorf("base URL is required via --url, AUTOQA_BASE_URL, or config plan.baseUrl")
	}
	cfg.Plan.BaseURL = baseURL
	runID := logging.RunID()
	runDir := filepath.Join(root, ".autoqa", "runs", runID)
	return cfg, runID, runDir, nil
}

// writeJSON 以缩进 JSON 写出 value 到 path。
func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
