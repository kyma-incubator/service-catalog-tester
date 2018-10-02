package monitoring

import (
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
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
}

type watchObj struct {
	watcher   watch.Interface
	sendEvent map[types.UID]struct{}
}

// NewWatcherService returns new instance of the WatcherService
func NewWatcherService(eventCli corev1.CoreV1Interface, slackNotifier SlackNotifier, log logrus.FieldLogger) *WatcherService {
	return &WatcherService{
		coreCli:       eventCli,
		slackNotifier: slackNotifier,
		log:           log.WithField("service", "event:watcher"),

		watchedObj: make(map[string]*watchObj),
	}
}

// Register registers given obj and starts watching events from it.
// Error is not returned when object with the same Name is already registered
func (s *WatcherService) Register(ref *v1.ObjectReference) error {
	if _, found := s.watchedObj[ref.Name]; found { // TODO add ns context
		return nil
	}

	nsEventCli := s.coreCli.Events(ref.Namespace)

	selector := nsEventCli.GetFieldSelector(s.selectorArgsFromRefObj(ref))
	eventWatcher, err := nsEventCli.Watch(metav1.ListOptions{
		FieldSelector: selector.String(),
	})
	if err != nil {
		return errors.Wrapf(err, "while creating watch Event interface for object %q", ref.Name)
	}

	go s.startWatching(ref, eventWatcher.ResultChan())
	s.watchedObj[ref.Name] = &watchObj{
		watcher:   eventWatcher,
		sendEvent: make(map[types.UID]struct{}),
	}

	return nil
}

func (s *WatcherService) startWatching(ref *v1.ObjectReference, events <-chan watch.Event) {
	// We will also get the restart events here, so we are not checking the status of the pod directly
	for r := range events {
		event := r.Object.(*v1.Event)
		if event.Type != "Normal" {
			failLogger := s.log.WithField("ID", event.GetUID())
			obj := s.watchedObj[ref.Name]
			if _, found := obj.sendEvent[event.GetUID()]; found {
				continue
			}

			eventMsg := fmt.Sprintf("Event type: %s, reason: %s, message: %s", event.Type, event.Reason, event.Message)
			dumpedLogs, err := s.podLogs(ref)
			if err != nil {
				failLogger.Errorf("Got error while getting log from pod: %v", err)
			}

			failureReasonHeader := fmt.Sprintf("*[Phase: MONITORING]* _Discover that Pod %s has problems_", ref.Name)
			err = s.slackNotifier.Notify(string(event.UID), failureReasonHeader, eventMsg)
			if err != nil {
				failLogger.Errorf("Got error while sending notification to Slack: %v", err)
				continue
			} else {
				obj.sendEvent[event.UID] = struct{}{}
			}

			failLogger.Infof(eventMsg)
			failLogger.Infof("Logs from pod %s/%s: %s", ref.Namespace, ref.Name, dumpedLogs)
		}

	}
}

func (s *WatcherService) podLogs(ref *v1.ObjectReference) (string, error) {
	req := s.coreCli.Pods(ref.Namespace).GetLogs(ref.Name, &v1.PodLogOptions{})

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
func (s *WatcherService) Unregister(name string) error {
	obj, found := s.watchedObj[name]
	if !found {
		return fmt.Errorf("object with name %q was not registered", name)
	}

	obj.watcher.Stop()
	delete(s.watchedObj, name)

	return nil
}

func (s *WatcherService) selectorArgsFromRefObj(ref *v1.ObjectReference) (*string, *string, *string, *string) {
	return s.strPtr(ref.Name), s.strPtr(ref.Namespace), s.strPtr(ref.Kind), s.strPtr(string(ref.UID))
}

func (*WatcherService) strPtr(s string) *string {
	return &s
}
