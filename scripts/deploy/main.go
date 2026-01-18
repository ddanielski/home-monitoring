// Package main provides deployment automation for the home-monitoring infrastructure.
// Usage: go run ./scripts/deploy --help
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Config struct {
	ProjectID   string
	Region      string
	Environment string
	Service     string
	ImageTag    string
	DryRun      bool
}

func main() {
	cfg := parseFlags()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		slog.Error("deployment failed", "error", err)
		os.Exit(1)
	}

	slog.Info("deployment completed successfully")
}

func parseFlags() Config {
	cfg := Config{}

	flag.StringVar(&cfg.ProjectID, "project", os.Getenv("GCP_PROJECT"), "GCP project ID")
	flag.StringVar(&cfg.Region, "region", "us-central1", "GCP region")
	flag.StringVar(&cfg.Environment, "env", "dev", "Environment (dev, prod)")
	flag.StringVar(&cfg.Service, "service", "telemetry-api", "Service to deploy")
	flag.StringVar(&cfg.ImageTag, "tag", "", "Image tag (default: git short SHA)")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Print commands without executing")

	flag.Parse()

	if cfg.ProjectID == "" {
		fmt.Fprintln(os.Stderr, "Error: --project or GCP_PROJECT env var is required")
		flag.Usage()
		os.Exit(1)
	}

	if cfg.ImageTag == "" {
		cfg.ImageTag = getGitShortSHA()
	}

	return cfg
}

func run(ctx context.Context, cfg Config) error {
	slog.Info("starting deployment",
		"project", cfg.ProjectID,
		"region", cfg.Region,
		"environment", cfg.Environment,
		"service", cfg.Service,
		"tag", cfg.ImageTag,
	)

	steps := []struct {
		name string
		fn   func(context.Context, Config) error
	}{
		{"Authenticate with GCP", authenticateGCP},
		{"Configure Docker for Artifact Registry", configureDocker},
		{"Build container image", buildImage},
		{"Push container image", pushImage},
		{"Deploy to Cloud Run", deployCloudRun},
	}

	for _, step := range steps {
		slog.Info("executing step", "step", step.name)
		if err := step.fn(ctx, cfg); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}

	return nil
}

func authenticateGCP(ctx context.Context, cfg Config) error {
	// Check if already authenticated
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	if err := cmd.Run(); err != nil {
		slog.Info("not authenticated, please run: gcloud auth login")
		return fmt.Errorf("not authenticated with GCP")
	}

	// Set project
	return runCmd(ctx, cfg.DryRun, "gcloud", "config", "set", "project", cfg.ProjectID)
}

func configureDocker(ctx context.Context, cfg Config) error {
	return runCmd(ctx, cfg.DryRun, "gcloud", "auth", "configure-docker",
		fmt.Sprintf("%s-docker.pkg.dev", cfg.Region), "--quiet")
}

func buildImage(ctx context.Context, cfg Config) error {
	imageURL := getImageURL(cfg)
	servicePath := fmt.Sprintf("./services/%s", cfg.Service)

	return runCmd(ctx, cfg.DryRun, "docker", "build",
		"-t", imageURL,
		"-t", fmt.Sprintf("%s:latest", strings.Split(imageURL, ":")[0]),
		"--platform", "linux/amd64",
		servicePath,
	)
}

func pushImage(ctx context.Context, cfg Config) error {
	imageURL := getImageURL(cfg)
	latestURL := fmt.Sprintf("%s:latest", strings.Split(imageURL, ":")[0])

	if err := runCmd(ctx, cfg.DryRun, "docker", "push", imageURL); err != nil {
		return err
	}
	return runCmd(ctx, cfg.DryRun, "docker", "push", latestURL)
}

func deployCloudRun(ctx context.Context, cfg Config) error {
	imageURL := getImageURL(cfg)

	return runCmd(ctx, cfg.DryRun, "gcloud", "run", "deploy", cfg.Service,
		"--image", imageURL,
		"--region", cfg.Region,
		"--platform", "managed",
		"--no-allow-unauthenticated",
		"--set-env-vars", fmt.Sprintf("GCP_PROJECT=%s,ENVIRONMENT=%s", cfg.ProjectID, cfg.Environment),
		"--memory", "512Mi",
		"--cpu", "1",
		"--min-instances", "0",
		"--max-instances", "10",
		"--timeout", "300s",
	)
}

func getImageURL(cfg Config) string {
	return fmt.Sprintf("%s-docker.pkg.dev/%s/home-monitoring/%s:%s",
		cfg.Region, cfg.ProjectID, cfg.Service, cfg.ImageTag)
}

func getGitShortSHA() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("dev-%d", time.Now().Unix())
	}
	return strings.TrimSpace(string(out))
}

func runCmd(ctx context.Context, dryRun bool, name string, args ...string) error {
	cmdStr := fmt.Sprintf("%s %s", name, strings.Join(args, " "))

	if dryRun {
		slog.Info("dry-run", "command", cmdStr)
		return nil
	}

	slog.Info("running", "command", cmdStr)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
