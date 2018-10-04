package monitoring

import (
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	tailLines      = 2000
	logsLimitBytes = 1048576 // 1MB
)

// SlackNotifier allows sending notification about messages to Slack channel.
type SlackNotifier interface {
	Notify(id, header, details string) error
}

// WatcherService allows to watch events for a given Pod and send notification to Slack if received event Type is different that `Normal`
type WatcherService struct {
	coreCli       corev1.CoreV1Interface
	slackNotifier SlackNotifier
	log           logrus.FieldLogger

	watchedObj map[string]*watchObj
	mux        *sync.RWMutex
}

type watchObj struct {
	watcher watch.Interface
	// TODO: We used as a value the empty struct which in Go empty struct has a width of zero ( It occupies zero bytes of storage).
	// This map will be released by the GC when we will stop watching given Pod.
	// We expect that there will not be too many different events sent to Pod, so we should not have a problem with allocated memory.
	sendEvent map[string]struct{}
}

// NewWatcherService returns new instance of the WatcherService
func NewWatcherService(eventCli corev1.CoreV1Interface, slackNotifier SlackNotifier, log logrus.FieldLogger) *WatcherService {
	return &WatcherService{
		coreCli:       eventCli,
		slackNotifier: slackNotifier,
		log:           log.WithField("service", "monitoring:event-watcher"),

		mux:        &sync.RWMutex{},
		watchedObj: make(map[string]*watchObj),
	}
}

// Register registers given obj and starts watching events from it.
// Error is not returned when object with the same Name is already registered.
func (s *WatcherService) Register(ref *v1.ObjectReference) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	key := s.refKey(ref)

	if _, found := s.watchedObj[key]; found {
		return nil
	}

	nsEventCli := s.coreCli.Events(ref.Namespace)

	selector := nsEventCli.GetFieldSelector(s.selectorArgsFromRefObj(ref))
	eventWatcher, err := nsEventCli.Watch(metav1.ListOptions{
		FieldSelector: selector.String(),
	})
	if err != nil {
		return errors.Wrapf(err, "while creating watch Event interface for object %q", key)
	}

	s.watchedObj[key] = &watchObj{
		watcher:   eventWatcher,
		sendEvent: make(map[string]struct{}),
	}
	go s.startWatching(ref, eventWatcher.ResultChan())

	return nil
}

func (*WatcherService) refKey(ref *v1.ObjectReference) string {
	return fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
}

func (s *WatcherService) startWatching(ref *v1.ObjectReference, events <-chan watch.Event) {
	// We will also get the restart events here, so we are not checking the status of the pod directly
	for r := range events {
		event := r.Object.(*v1.Event)
		if event.Type != v1.EventTypeNormal {
			id := s.eventKey(event)
			failLogger := s.log.WithField("ID", id)

			obj := s.watchedObj[s.refKey(ref)]
			if _, found := obj.sendEvent[id]; found {
				continue
			}

			eventMsg := fmt.Sprintf("Event type: %s, reason: %s, message: %s", event.Type, event.Reason, event.Message)
			dumpedLogs, err := s.podLogs(ref)
			if err != nil {
				failLogger.Errorf("Got error while getting log from pod: %v", err)
			}

			failureReasonHeader := fmt.Sprintf("*[Phase: MONITORING]* _Discover that Pod %s has problems_", ref.Name)
			err = s.slackNotifier.Notify(id, failureReasonHeader, eventMsg)
			if err != nil {
				failLogger.Errorf("Got error while sending notification to Slack: %v", err)
				continue
			} else {
				obj.sendEvent[id] = struct{}{}
			}

			failLogger.Infof(eventMsg)
			failLogger.Infof("Logs from Pod %s/%s: %s", ref.Namespace, ref.Name, dumpedLogs)
		}

	}
}

func (s *WatcherService) eventKey(event *v1.Event) string {
	return fmt.Sprintf("%s/%s", event.Namespace, event.Name)
}
func (s *WatcherService) podLogs(ref *v1.ObjectReference) (string, error) {
	req := s.coreCli.Pods(ref.Namespace).GetLogs(ref.Name, &v1.PodLogOptions{
		TailLines:  s.int64Ptr(tailLines),
		LimitBytes: s.int64Ptr(logsLimitBytes),
	})

	readCloser, err := req.Stream()
	if err != nil {
		return "", errors.Wrap(err, "while getting log stream")
	}
	defer readCloser.Close()

	logs, err := ioutil.ReadAll(readCloser)
	if err != nil {
		return "", errors.Wrapf(err, "while reading logs from pod %s", ref.Name)
	}

	return string(logs), nil
}

// Unregister removes obj from watcher list and stop watching the events from it.
// Error is not returned when object was not registered.
func (s *WatcherService) Unregister(ref *v1.ObjectReference) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	key := s.refKey(ref)

	obj, found := s.watchedObj[key]
	if !found {
		return nil
	}

	obj.watcher.Stop()
	delete(s.watchedObj, key)

	return nil
}

func (s *WatcherService) selectorArgsFromRefObj(ref *v1.ObjectReference) (*string, *string, *string, *string) {
	return s.strPtr(ref.Name), s.strPtr(ref.Namespace), s.strPtr(ref.Kind), s.strPtr(string(ref.UID))
}

func (*WatcherService) strPtr(s string) *string {
	return &s
}

func (*WatcherService) int64Ptr(i int64) *int64 {
	return &i
}
