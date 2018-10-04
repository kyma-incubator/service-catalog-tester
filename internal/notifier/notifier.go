package notifier

import (
	"github.com/pkg/errors"
)

type (
	slackClient interface {
		Send(header, body, footer, color string) error
	}
	msgRenderer interface {
		RenderSlackMessage(in RenderSlackMessageInput) (string, string, string, error)
	}
)

const redColor = "#d92626"

// SlackNotifier sends notification messages to Slack channel.
type SlackNotifier struct {
	slack       slackClient
	msgRenderer msgRenderer
	clusterName string
}

// New returns new instance of SlackNotifier
func New(clusterName string, slack slackClient, testRenderer msgRenderer) *SlackNotifier {
	return &SlackNotifier{
		slack:       slack,
		msgRenderer: testRenderer,
		clusterName: clusterName,
	}
}

// Notify sends notification message to Slack channel
func (s *SlackNotifier) Notify(id, header, details string) error {
	header, body, footer, err := s.msgRenderer.RenderSlackMessage(RenderSlackMessageInput{
		LogID:       id,
		Header:      header,
		Details:     details,
		ClusterName: s.clusterName,
	})
	if err != nil {
		return errors.Errorf("Cannot render slack message, got error: %v", details)
	}

	if err := s.slack.Send(header, body, footer, redColor); err != nil {
		return errors.Errorf("Cannot render slack message, got error: %v", err)
	}

	return nil
}
