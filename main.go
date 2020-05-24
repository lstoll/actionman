package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/google/go-github/v31/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	ctx := context.Background()
	l := logrus.New()

	// ref https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables
	_ = kingpin.Flag("github-repository", "Name of the repository to act on").Envar("GITHUB_REPOSITORY").Required().String()
	githubEventPath := kingpin.Flag("github-event-path", "Path to event.json").Envar("GITHUB_EVENT_PATH").Required().String()
	githubEventName := kingpin.Flag("github-event-name", "name of the event that triggered this run").Envar("GITHUB_EVENT_NAME").Required().String()

	// note - the credential helper we set also expects this env var directly,
	// so if we change how we pass it to this process we need to update it for
	// that too
	githubToken := kingpin.Flag("github-token", "Token to access the github API with").Envar("ACTIONMAN_GITHUB_TOKEN").Required().String()
	debugLevel := kingpin.Flag("debug", "output debug logs").Envar("ACTIONMAN_DEBUG").Default("false").Bool()

	_ = kingpin.Parse()

	if *debugLevel {
		l.SetLevel(logrus.DebugLevel)
	}

	eventData, err := ioutil.ReadFile(*githubEventPath)
	if err != nil {
		l.WithError(err).Fatalf("error reading %s", *githubEventPath)
	}
	_ = eventData
	event, err := github.ParseWebHook(*githubEventName, eventData)
	if err != nil {
		l.WithError(err).Error("Parsing webhook failed")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	ghClient := github.NewClient(tc)

	switch *githubEventName {
	case "issue_comment":
		l.Info("Handling issue comment event")
		e := event.(*github.IssueCommentEvent)
		if err := handleComment(ctx, l, ghClient, e); err != nil {
			l.WithError(err).Error("Handling comment")
		}
	default:
		l.WithField("event", *githubEventName).Info("unhandled event, ignoring")
	}
}

func handleComment(ctx context.Context, l logrus.FieldLogger, gh *github.Client, ev *github.IssueCommentEvent) error {
	if strings.HasPrefix(*ev.Comment.Body, "/ping") {
		l.Infof("Responding to /ping comment on issue %d", *ev.Issue.Number)
		if _, _, err := gh.Issues.CreateComment(ctx, *ev.Repo.Owner.Login, *ev.Repo.Name, *ev.Issue.Number, &github.IssueComment{
			Body: sp("PONG"),
		}); err != nil {
			return fmt.Errorf("posting reply comment: %v", err)
		}
	}
	if strings.HasPrefix(*ev.Comment.Body, "/kubectl") && ev.Issue.IsPullRequest() {
		l.Infof("Responding to /kubectl comment on PR %d", *ev.Issue.Number)
		if err := handleKubectl(ctx, l, gh, ev); err != nil {
			return fmt.Errorf("handling /kubectl: %v", err)
		}
	}
	return nil
}

func handleKubectl(ctx context.Context, l logrus.FieldLogger, gh *github.Client, ev *github.IssueCommentEvent) error {
	lines := strings.Split(*ev.Comment.Body, "\n")
	args := strings.Split(lines[0], " ")
	if args[0] != "/kubectl" {
		return fmt.Errorf("consistency error - expected first arg /kubectl, got %s", args[0])
	}
	if len(args) != 3 {
		if err := writeComment(ctx, l, gh, ev, "usage: /kubectl <command> <cluster>"); err != nil {
			return err
		}
		return nil
	}
	switch args[1] {
	case "plan":
		if err := writeComment(ctx, l, gh, ev, "PLAN OUTPUT GOES HERE"); err != nil {
			return err
		}
	default:
		if err := writeComment(ctx, l, gh, ev, "invalid command %s, must be plan or apply", args[1]); err != nil {
			return err
		}
	}

	return nil
}

func writeComment(ctx context.Context, l logrus.FieldLogger, gh *github.Client, ev *github.IssueCommentEvent, format string, a ...interface{}) error {
	m := fmt.Sprintf(format, a...)
	if _, _, err := gh.Issues.CreateComment(ctx, *ev.Repo.Owner.Login, *ev.Repo.Name, *ev.Issue.Number, &github.IssueComment{
		Body: &m,
	}); err != nil {
		return fmt.Errorf("posting reply comment: %v", err)
	}
	return nil
}

func sp(s string) *string {
	return &s
}
