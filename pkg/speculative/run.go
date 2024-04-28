package speculative

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/google/go-github/v61/github"
)

var (
	owner            string
	repo             string
	workflowFilename string
)

func Run(ctx context.Context) error {
	flag.StringVar(&owner, "github-owner", "", "Repository owner")
	flag.StringVar(&repo, "github-repo", "", "Repository name")
	flag.StringVar(&workflowFilename, "workflow-file", "preview.yaml", "Name of the workflow file to run for previews")
	flag.Parse()

	if owner == "" || repo == "" {
		if ghURL, err := gitRepoOrigin(); err == nil {
			parts := strings.Split(ghURL.Path, "/")
			if len(parts) >= 2 {
				owner = parts[0]
				repo = strings.TrimSuffix(parts[1], ".git")
				fmt.Printf("Using local repo info: %s/%s\n", owner, repo)
			}
		}
	}
	if owner == "" {
		return errors.New("Missing flag: -github-owner")
	}
	if repo == "" {
		return errors.New("Missing flag: -github-repo")
	}

	serverURL, err := serveWorkspace(ctx)
	if err != nil {
		return err
	}

	// steal token from GH CLI
	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	token := strings.TrimSpace(string(out))
	gh := github.NewClient(nil).WithAuthToken(token)

	startedAt := time.Now().UTC()

	// start workflow
	_, err = gh.Actions.CreateWorkflowDispatchEventByFileName(ctx,
		owner, repo, workflowFilename,
		github.CreateWorkflowDispatchEventRequest{
			Ref: "main",
			Inputs: map[string]interface{}{
				"workspace_transfer_url": serverURL,
			},
		},
	)
	if err != nil {
		return err
	}

	fmt.Println("Waiting for run to start...")

	// find workflow run
	var run *github.WorkflowRun
	err = backoff.Retry(func() error {
		workflows, _, err := gh.Actions.ListWorkflowRunsByFileName(
			ctx, owner, repo, workflowFilename,
			&github.ListWorkflowRunsOptions{
				Created: fmt.Sprintf(">=%s", startedAt.Format("2006-01-02T15:04")),
			},
		)
		if err != nil {
			return backoff.Permanent(err)
		}
		if len(workflows.WorkflowRuns) == 0 {
			return fmt.Errorf("no workflow runs found")
		}

		run = workflows.WorkflowRuns[0]
		return nil
	}, backoff.NewExponentialBackOff())
	if err != nil {
		return err
	}

	var jobID int64
	err = backoff.Retry(func() error {
		jobs, _, err := gh.Actions.ListWorkflowJobs(ctx,
			owner, repo, *run.ID,
			&github.ListWorkflowJobsOptions{},
		)
		if err != nil {
			return backoff.Permanent(err)
		}
		if len(jobs.Jobs) == 0 {
			return fmt.Errorf("no jobs found")
		}

		jobID = *jobs.Jobs[0].ID
		return nil
	}, backoff.NewExponentialBackOff())
	if err != nil {
		return err
	}

	logsURL, _, err := gh.Actions.GetWorkflowJobLogs(ctx, owner, repo, jobID, 2)
	if err != nil {
		return err
	}

	var readBytes int64
	for {
		n, err := streamLogs(logsURL, readBytes)
		if err != nil {
			return err
		}
		readBytes += n

		// check if job is done
		job, _, err := gh.Actions.GetWorkflowJobByID(ctx, owner, repo, jobID)
		if err != nil {
			return err
		}
		if job.CompletedAt != nil {
			fmt.Println("Job complete.")
			return nil
		}
	}
}
