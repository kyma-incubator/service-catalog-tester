package runner

import (
	"fmt"
	"time"

	"github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

// StressTestRunner is a test runner
type StressTestRunner struct {
	log           logrus.FieldLogger
	slackNotifier SlackNotifier
}

// SlackNotifier allows sending notification about messages to Slack channel.
type SlackNotifier interface {
	Notify(id, header, details string) error
}

// NewStressTestRunner is a constructor for StressTestRunner
func NewStressTestRunner(slackNotifier SlackNotifier, log logrus.FieldLogger) *StressTestRunner {
	return &StressTestRunner{
		log:           log.WithField("service", "test:runner"),
		slackNotifier: slackNotifier,
	}
}

// Run executes given test in a loop with given throttle
func (r *StressTestRunner) Run(stopCh <-chan struct{}, throttle time.Duration, test Test) error {
	for {
		if r.shutdownRequested(stopCh) {
			return nil
		}

		testID := r.generateTestID()
		testLogger := r.log.WithField("ID", testID)
		testLogger.Infof("Starting test %q", test.Name())

		startTime := time.Now()
		if err := test.Execute(stopCh); err != nil {
			testLogger.Errorf("Test %q end with error [start time: %v, duration: %v]: %v", test.Name(), startTime, time.Since(startTime), err)

			failureReasonHeader := fmt.Sprintf("*[Phase: TESTING]* _Stress tests *%s* failed_", test.Name())
			if err := r.slackNotifier.Notify(testID, failureReasonHeader, err.Error()); err != nil {
				testLogger.Errorf("Got error when sending Slack notification: %v", err)
			}
		} else {
			testLogger.Infof("Test %q end with success [start time: %v, duration: %v]", test.Name(), startTime, time.Since(startTime))
		}

		r.log.Infof("Throttle test %s for %v", test.Name(), throttle)
		if canceled := r.throttleTest(stopCh, throttle); canceled {
			return nil
		}
	}
}

// generateTestID generates random test ID
func (r *StressTestRunner) generateTestID() string {
	return uuid.NewV4().String()
}

func (r *StressTestRunner) shutdownRequested(stopCh <-chan struct{}) bool {
	select {
	case <-stopCh:
		r.log.Debug("Stop channel called. Shutdown test runner")
		return true
	default:
	}

	return false
}

func (r *StressTestRunner) throttleTest(stopCh <-chan struct{}, throttle time.Duration) bool {
	select {
	case <-stopCh:
		r.log.Debug("Stop channel called. Shutdown test runner")
		return true
	case <-time.After(throttle):
	}

	return false
}
