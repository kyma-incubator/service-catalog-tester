package notifier

import (
	"bytes"
	"text/template"

	"github.com/pkg/errors"
)

// MessageRenderer renders Slack message
type MessageRenderer struct {
	headerReportTmpl *template.Template
	bodyReportTmpl   *template.Template
	footerReportTmpl *template.Template
}

// NewMessageRenderer returns new instance of MessageRenderer
func NewMessageRenderer() (*MessageRenderer, error) {
	headerReportTmpl, err := template.New("header").Parse(header)
	if err != nil {
		return nil, errors.Wrapf(err, "while parsing header template")
	}

	bodyReportTmpl, err := template.New("body").Parse(body)
	if err != nil {
		return nil, errors.Wrapf(err, "while parsing body template")
	}

	footerReportTmpl, err := template.New("footer").Parse(footer)
	if err != nil {
		return nil, errors.Wrapf(err, "while parsing footer template")
	}

	return &MessageRenderer{
		headerReportTmpl: headerReportTmpl,
		bodyReportTmpl:   bodyReportTmpl,
		footerReportTmpl: footerReportTmpl,
	}, nil
}

// RenderSlackMessageInput holds input parameters required to render test summary
type RenderSlackMessageInput struct {
	Details     string
	Header      string
	ClusterName string
	LogID       string
}

// RenderSlackMessage returns header and body summary of given tests
func (s *MessageRenderer) RenderSlackMessage(in RenderSlackMessageInput) (string, string, string, error) {
	header := &bytes.Buffer{}
	if err := s.headerReportTmpl.Execute(header, in); err != nil {
		return "", "", "", errors.Wrapf(err, "while executing header template")
	}

	body := &bytes.Buffer{}
	if err := s.bodyReportTmpl.Execute(body, in); err != nil {
		return "", "", "", errors.Wrapf(err, "while executing body template")
	}

	footer := &bytes.Buffer{}
	if err := s.footerReportTmpl.Execute(footer, in); err != nil {
		return "", "", "", errors.Wrapf(err, "while executing footer template")
	}

	return header.String(), body.String(), footer.String(), nil
}
