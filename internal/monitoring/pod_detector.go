package monitoring

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	typesCoreV1 "k8s.io/api/core/v1"
	informersCoreV1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/reference"
)

// EventWatchNotifier allows to watch events for a given Pod and send notification to Slack if received event Type is different that `Normal`
type EventWatchNotifier interface {
	Register(ref *typesCoreV1.ObjectReference) error
	Unregister(ref *typesCoreV1.ObjectReference) error
}

// PodDetector dynamically register and unregister Pods for event watching.
// Registered Pods belongs to requested Deployments.
type PodDetector struct {
	watcher     EventWatchNotifier
	log         logrus.FieldLogger
	podInformer informersCoreV1.PodInformer

	observables map[string][]Labels
}

// Labels describes the common type for labels field
type Labels map[string]string

// Observable defines which Pods should be observed
type Observable struct {
	Namespace       string
	PodLabelsGroups []Labels
}

// NewPodDetector returns new instance of the PodDetector
func NewPodDetector(podInformer informersCoreV1.PodInformer, watcher EventWatchNotifier, log logrus.FieldLogger, observables ...Observable) *PodDetector {
	mapped := map[string][]Labels{}
	for _, ob := range observables {
		for _, labelGroup := range ob.PodLabelsGroups {
			mapped[ob.Namespace] = append(mapped[ob.Namespace], labelGroup)
		}
		spew.Dump(mapped[ob.Namespace])
	}
	return &PodDetector{
		watcher:     watcher,
		podInformer: podInformer,
		observables: mapped,
		log:         log.WithField("service", "monitoring:pod-detector"),
	}
}

// Start starts the process of registering Pods from given Deployment into EventWatcher
func (e *PodDetector) Start() error {
	e.podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    e.addPod,
		UpdateFunc: e.updatePod,
		DeleteFunc: e.deletePod,
	})

	return nil
}

func (e *PodDetector) addPod(obj interface{}) {
	pod, ok := obj.(*typesCoreV1.Pod)
	if !ok {
		e.log.Warnf("while handling addition: cannot covert obj [%+v] of type %T to *Pod", obj, obj)
		return
	}

	if !e.shouldActOn(pod) {
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

func (e *PodDetector) deletePod(obj interface{}) {
	pod, ok := obj.(*typesCoreV1.Pod)
	if !ok {
		e.log.Warnf("while handling deletion: cannot covert obj [%+v] of type %T to *Pod", obj, obj)
		return
	}

	if !e.shouldActOn(pod) {
		return
	}

	ref, err := reference.GetReference(scheme.Scheme, pod)
	if err != nil {
		logrus.Errorf("Got error while getting Pod %q reference: %v", pod.Name, err)
		return
	}

	e.log.Debugf("Stopping watching Pod %s", pod.Name)
	if err := e.watcher.Unregister(ref); err != nil {
		e.log.Errorf("while unregistering Pod: %v", err)
		return
	}
}

func (e *PodDetector) updatePod(oldObj, newObj interface{}) {
	e.addPod(newObj)
}

func (e *PodDetector) shouldActOn(pod *typesCoreV1.Pod) bool {
	matchLabelsGroups, found := e.observables[pod.Namespace]
	if !found {
		return false
	}

	gotLabels := pod.DeepCopy().Labels
	delete(gotLabels, "pod-template-hash") // it's additional label added by the k8s controller
	for _, exp := range matchLabelsGroups {
		if assert.ObjectsAreEqualValues(exp, gotLabels) {
			return true
		}
	}

	return false
}
