package runner

type (
	// Test allows to execute test in a generic way
	Test interface {
		Execute(stop <-chan struct{}) error
		Name() string
	}
)
