package notifier

const (
	header = " {{ .Header }} :sad-frog:"
	body   = `
	*Details:*
		{{ .Details }}

	Additional information were logged with ID: {{ .LogID }}.
	`
	footer = `Check cluster _{{ .ClusterName }}_ *ASAP* to gather information about the failure.`
)
