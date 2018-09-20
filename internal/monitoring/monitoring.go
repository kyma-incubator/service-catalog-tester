package monitoring

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	typesCoreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	informersCoreV1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	appsV1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/reference"
)

// EventWatchNotifier allows to watch events for a given Pod and send notification to Slack if received event Type is different that `Normal`
type EventWatchNotifier interface {
	Register(ref *typesCoreV1.ObjectReference) error
	Unregister(name string) error
}

// EventMonitor dynamically register and unregister Pods for event watching.
// Registered Pods belongs to requested Deployments.
type EventMonitor struct {
	watcher     EventWatchNotifier
	log         logrus.FieldLogger
	appsCli     appsV1.AppsV1Interface
	podInformer informersCoreV1.PodInformer
	observable  Observable

	matchLabels []Label
}

// Label describes the common type for labels field
type Label map[string]string

// Observable defines which Deployment should be observed
type Observable struct {
	Namespace        string
	DeploymentsNames []string
}

// NewEventMonitor returns new instance of the EventMonitor
func NewEventMonitor(appsCli appsV1.AppsV1Interface, podInformer informersCoreV1.PodInformer, watcher EventWatchNotifier, observable Observable, log logrus.FieldLogger) *EventMonitor {
	return &EventMonitor{
		watcher:     watcher,
		appsCli:     appsCli,
		podInformer: podInformer,
		observable:  observable,
		log:         log.WithField("service", "monitoring:monitor"),
	}
}

// Start starts the process of registering Pods from given Deployment into EventWatcher
func (e *EventMonitor) Start() error {
	for _, deployName := range e.observable.DeploymentsNames {
		d, err := e.appsCli.Deployments(e.observable.Namespace).Get(deployName, metaV1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "while getting deployment %q", deployName)
		}
		e.matchLabels = append(e.matchLabels, d.Spec.Template.Labels)
	}

	e.podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    e.addPod,
		UpdateFunc: e.updatePod,
		DeleteFunc: e.deletePod,
	})

	return nil
}

func (e *EventMonitor) addPod(obj interface{}) {
	pod, ok := obj.(*typesCoreV1.Pod)
	if !ok {
		e.log.Warnf("while handling addition: cannot covert obj [%+v] of type %T to *Pod", obj, obj)
		return
	}

	l := pod.DeepCopy()
	delete(l.Labels, "pod-template-hash") // it's additional label added by the k8s controller
	if !e.shouldWatch(e.matchLabels, l.Labels) {
		return
	}

	ref, err := reference.GetReference(scheme.Scheme, pod)
	if err != nil {
		logrus.Errorf("Got error while getting Pod %q reference: %v", pod.Name, err)
		return
	}

	e.log.Infof("Starting watching Pod %q", pod.Name)
	if err = e.watcher.Register(ref); err != nil {
		logrus.Errorf("Got error while registering  Pod %q: %v", pod.Name, err)
		return
	}
}

func (e *EventMonitor) deletePod(obj interface{}) {
	pod, ok := obj.(*typesCoreV1.Pod)
	if !ok {
		e.log.Warnf("while handling deletion: cannot covert obj [%+v] of type %T to *Pod", obj, obj)
		return
	}

	e.log.Debugf("Stopping watching Pod %s", pod.Name)
	if err := e.watcher.Unregister(pod.Name); err != nil {
		e.log.Errorf("while unregistering Pod: %v", err)
		return
	}
}

func (e *EventMonitor) updatePod(oldObj, newObj interface{}) {
	e.addPod(newObj)
}

func (e *EventMonitor) shouldWatch(expected []Label, got Label) bool {
	for _, exp := range expected {
		if reflect.DeepEqual(exp, got) {
			return true
		}
	}
	return false
}
