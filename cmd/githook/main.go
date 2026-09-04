package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	gh "github.com/gnailuy/githook/internal/githook"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: githook <serve|worker|reconcile|deploy-run>")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	q, err := gh.OpenQueue(env("GITHOOK_DATABASE", "/var/lib/githook/githook.db"))
	if err != nil {
		fatal(err.Error())
	}
	defer q.Close()
	g := gh.GitHub{Repository: env("GITHOOK_REPOSITORY", "gnailuy/gnailuy.com"), Token: os.Getenv("GITHUB_TOKEN")}
	d := gh.Deployer{ReleasesDir: env("GITHOOK_RELEASES", "/srv/gnailuy/releases"), CurrentLink: env("GITHOOK_CURRENT", "/srv/gnailuy/current"), SmokeURLs: split(os.Getenv("GITHOOK_SMOKE_URLS"))}
	w := gh.Worker{Queue: q, GitHub: g, Deployer: d, WorkflowName: env("GITHOOK_WORKFLOW_NAME", "Verify site"), WorkflowPath: env("GITHOOK_WORKFLOW_PATH", ".github/workflows/verify.yml"), Branch: env("GITHOOK_BRANCH", "master")}
	switch os.Args[1] {
	case "serve":
		r := gh.Receiver{Secret: []byte(os.Getenv("GITHOOK_WEBHOOK_SECRET")), Repository: g.Repository, Queue: q}
		s := gh.Service{WebhookPath: env("GITHOOK_WEBHOOK_PATH", gh.DefaultWebhookPath), Receiver: r, Queue: q}
		fatalIf(gh.ListenAndServe(ctx, env("GITHOOK_LISTEN", "127.0.0.1:20182"), s))
	case "worker":
		requireWorkerConfig(g.Token, d.SmokeURLs)
		fatalIf(w.Run(ctx))
	case "deploy-run":
		fs := flag.NewFlagSet("deploy-run", flag.ExitOnError)
		sha := fs.String("sha", "", "expected full head SHA")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 1 || *sha == "" {
			fatal("usage: githook deploy-run --sha <sha> <run-id>")
		}
		requireWorkerConfig(g.Token, d.SmokeURLs)
		id, e := strconv.ParseInt(fs.Arg(0), 10, 64)
		fatalIf(e)
		fatalIf(w.ProcessRun(ctx, id, *sha))
	case "reconcile":
		requireWorkerConfig(g.Token, d.SmokeURLs)
		fatalIf(reconcile(ctx, w))
	default:
		fatal("unknown command")
	}
}
func reconcile(ctx context.Context, w gh.Worker) error {
	u := strings.TrimRight(env("GITHUB_API_URL", "https://api.github.com"), "/") + "/repos/" + w.GitHub.Repository + "/actions/workflows/verify.yml/runs?branch=" + w.Branch + "&event=push&status=success&per_page=1"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+w.GitHub.Token)
	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("latest run: %s", resp.Status)
	}
	var v struct {
		Runs []gh.Run `json:"workflow_runs"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return err
	}
	if len(v.Runs) != 1 {
		return fmt.Errorf("no eligible run")
	}
	return w.ProcessRun(ctx, v.Runs[0].ID, v.Runs[0].HeadSHA)
}
func requireWorkerConfig(token string, smoke []string) {
	if token == "" {
		fatal("GITHUB_TOKEN is required")
	}
	if len(smoke) == 0 {
		fatal("GITHOOK_SMOKE_URLS is required")
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
func fatal(s string) { log.Fatal(s) }
func fatalIf(e error) {
	if e != nil {
		fatal(fmt.Sprint(e))
	}
}
