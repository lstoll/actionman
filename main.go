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

	githubToken := kingpin.Flag("github-token", "Token to access the github API with").Envar("ACTIONMAN_GITHUB_TOKEN").Required().String()
	debugLevel := kingpin.Flag("debug", "output debug logs").Envar("ACTIONMAN_DEBUG").Default("false").Bool()

	_ = kingpin.Parse()

	if *debugLevel {
		l.SetLevel(logrus.DebugLevel)
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	eventData, err := ioutil.ReadFile(*githubEventPath)
	if err != nil {
		l.WithError(err).Fatalf("error reading %s", *githubEventPath)
	}
	_ = eventData
	event, err := github.ParseWebHook(*githubEventName, eventData)
	if err != nil {
		l.WithError(err).Error("Parsing webhook failed")
	}

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
	return nil
}

func sp(s string) *string {
	return &s
}
