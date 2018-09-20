package tests

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	scTypes "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scClient "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"

	bucTypes "github.com/kyma-project/kyma/components/binding-usage-controller/pkg/apis/servicecatalog/v1alpha1"
	bucClient "github.com/kyma-project/kyma/components/binding-usage-controller/pkg/client/clientset/versioned"

	"github.com/pkg/errors"
	appsTypes "k8s.io/api/apps/v1beta1"
	k8sCoreTypes "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

const timeoutPerStep = time.Minute

// E2EServiceCatalogHappyPathTest tests the Service Catalog basic functionality:
// - Creating ServiceInstance
// - Create ServiceBininding
// - and injecting those bindings to sample application
type E2EServiceCatalogHappyPathTest struct {
	k8sClientCfg *restclient.Config
}

// NewE2EServiceCatalogHappyPathTest returns new instance of E2EServiceCatalogHappyPathTest
func NewE2EServiceCatalogHappyPathTest(k8sClientCfg *restclient.Config) *E2EServiceCatalogHappyPathTest {
	return &E2EServiceCatalogHappyPathTest{
		k8sClientCfg: k8sClientCfg,
	}
}

// Execute executes basic Service Catalog test
func (t *E2EServiceCatalogHappyPathTest) Execute(stop <-chan struct{}) (retErr error) {
	// setup
	ts, err := t.newTestSuite()
	if err != nil {
		return errors.Wrap(err, "while creating test suite")
	}

	if err := ts.createTestNamespace(); err != nil {
		return errors.Wrap(err, "while creating test namespace")
	}
	// clean-up
	defer func() {
		if err := ts.ensureTestNamespaceIsDeleted(stop); err != nil {
			retErr = t.appendErr(retErr, errors.Wrap(err, "while ensuring that test namespace is deleted"))
		}
	}()

	// e2e creation steps
	steps := []func(timeout time.Duration) error{
		ts.createAndWaitForRedisInstance,
		ts.createAndWaitForRedisServiceBinding,
		ts.createTesterDeploymentAndService,
		ts.createBindingUsageForTesterDeployment,
	}

	for _, step := range steps {
		if err := step(timeoutPerStep); err != nil {
			return err
		}
	}

	// verification
	if err := ts.assertInjectedEnvVariable("PORT", "6379", 2*timeoutPerStep); err != nil {
		return errors.Wrap(err, "while checking that envs are injected")
	}

	return nil
}

// Name returns the name of the stress test
func (t *E2EServiceCatalogHappyPathTest) Name() string {
	return "E2E ServiceCatalog Happy Path"
}

func (t *E2EServiceCatalogHappyPathTest) newTestSuite() (*testSuite, error) {
	randID := rand.String(5)

	k8sCli, err := kubernetes.NewForConfig(t.k8sClientCfg)
	if err != nil {
		return nil, err
	}

	scCli, err := scClient.NewForConfig(t.k8sClientCfg)
	if err != nil {
		return nil, err
	}

	bucCli, err := bucClient.NewForConfig(t.k8sClientCfg)
	if err != nil {
		return nil, err
	}

	return &testSuite{
		k8sCli: k8sCli,
		scCli:  scCli,
		bucCli: bucCli,

		namespace:            fmt.Sprintf("stress-test-%s", randID),
		testerDeploymentName: fmt.Sprintf("stress-test-env-tester-%s", randID),

		serviceInstanceName:     fmt.Sprintf("stress-test-instance-a-%s", randID),
		bindingName:             fmt.Sprintf("stress-test-credential-a-%s", randID),
		testerDeploymentSvcName: fmt.Sprintf("stress-test-svc-id-a-%s", randID),
	}, nil
}

type testSuite struct {
	k8sCli *kubernetes.Clientset
	scCli  *scClient.Clientset
	bucCli *bucClient.Clientset

	namespace string

	testerDeploymentName string

	serviceInstanceName     string
	bindingName             string
	testerDeploymentSvcName string
}

// K8s namespace helpers
func (ts *testSuite) createTestNamespace() error {
	nsClient := ts.k8sCli.CoreV1().Namespaces()
	_, err := nsClient.Create(&k8sCoreTypes.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ts.namespace,
			Labels: map[string]string{
				"env": "true",
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (ts *testSuite) ensureTestNamespaceIsDeleted(stop <-chan struct{}) error {
	nsClient := ts.k8sCli.CoreV1().Namespaces()
	if err := nsClient.Delete(ts.namespace, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	for {
		select {
		case <-stop:
			return errors.Errorf("Stop channel closed. Test namespace %s can be still in Terminating phase", ts.namespace)
		default:
		}

		_, err := nsClient.Get(ts.namespace, metav1.GetOptions{})
		if !apiErrors.IsNotFound(err) {
			continue
		}

		break
	}

	return nil
}

// ServiceInstance helpers
func (ts *testSuite) createAndWaitForRedisInstance(timeout time.Duration) error {
	siClient := ts.scCli.ServicecatalogV1beta1().ServiceInstances(ts.namespace)
	_, err := siClient.Create(&scTypes.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: ts.serviceInstanceName,
		},
		Spec: scTypes.ServiceInstanceSpec{
			PlanReference: scTypes.PlanReference{
				ClusterServiceClassExternalName: "redis",
				ClusterServicePlanExternalName:  "micro",
			},
		},
	})
	if err != nil {
		return err
	}

	instanceInReadyState := func() error {
		si, err := siClient.Get(ts.serviceInstanceName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		isNotReady := func(instance *scTypes.ServiceInstance) bool {
			for _, cond := range instance.Status.Conditions {
				if cond.Type == scTypes.ServiceInstanceConditionReady {
					return cond.Status != scTypes.ConditionTrue
				}
			}
			return true
		}

		if isNotReady(si) {
			return fmt.Errorf("ServiceInstance %s/%s is not in ready state. Status: %+v", si.Namespace, si.Name, si.Status)
		}

		return nil
	}

	if err = repeatUntilTimeout(instanceInReadyState, timeout); err != nil {
		return err
	}

	return nil
}

// Binding helpers
func (ts *testSuite) createAndWaitForRedisServiceBinding(timeout time.Duration) error {
	bindingClient := ts.scCli.ServicecatalogV1beta1().ServiceBindings(ts.namespace)
	_, err := bindingClient.Create(&scTypes.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ts.bindingName,
			Namespace: ts.namespace,
		},
		Spec: scTypes.ServiceBindingSpec{
			ServiceInstanceRef: scTypes.LocalObjectReference{
				Name: ts.serviceInstanceName,
			},
		},
	})
	if err != nil {
		return err
	}

	err = repeatUntilTimeout(func() error {
		b, err := bindingClient.Get(ts.bindingName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		isNotReady := func(instance *scTypes.ServiceBinding) bool {
			for _, cond := range instance.Status.Conditions {
				if cond.Type == scTypes.ServiceBindingConditionReady {
					return cond.Status != scTypes.ConditionTrue
				}
			}
			return true
		}

		if isNotReady(b) {
			return fmt.Errorf("ServiceBinding %s/%s is not in ready state. Status: %+v", b.Namespace, b.Name, b.Status)
		}

		return nil
	}, timeout)
	if err != nil {
		return err
	}

	return nil
}

// BindingUsage helpers
func (ts *testSuite) createBindingUsageForTesterDeployment(timeout time.Duration) error {
	sbu := &bucTypes.ServiceBindingUsage{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceBindingUsage",
			APIVersion: "servicecatalog.kyma.cx/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "binding-usage-tester",
		},
		Spec: bucTypes.ServiceBindingUsageSpec{
			ServiceBindingRef: bucTypes.LocalReferenceByName{
				Name: ts.bindingName,
			},
			UsedBy: bucTypes.LocalReferenceByKindAndName{
				Kind: "deployment",
				Name: ts.testerDeploymentName,
			},
		},
	}

	_, err := ts.bucCli.ServicecatalogV1alpha1().ServiceBindingUsages(ts.namespace).Create(sbu)
	if err != nil {
		return err
	}

	return nil
}

// Deployment helpers
func (ts *testSuite) createTesterDeploymentAndService(timeout time.Duration) error {
	labels := map[string]string{
		"app": ts.testerDeploymentName,
	}
	deploy := ts.envTesterDeployment(labels)
	svc := ts.envTesterService(labels)

	deploymentClient := ts.k8sCli.AppsV1beta1().Deployments(ts.namespace)
	if _, err := deploymentClient.Create(deploy); err != nil {
		return err
	}

	serviceClient := ts.k8sCli.CoreV1().Services(ts.namespace)
	if _, err := serviceClient.Create(svc); err != nil {
		return err
	}

	return nil
}

func (ts *testSuite) envTesterService(labels map[string]string) *k8sCoreTypes.Service {
	return &k8sCoreTypes.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: ts.testerDeploymentSvcName,
		},
		Spec: k8sCoreTypes.ServiceSpec{
			Selector: labels,
			Ports: []k8sCoreTypes.ServicePort{
				{
					Name:       "http",
					Protocol:   "TCP",
					Port:       80,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			Type: k8sCoreTypes.ServiceTypeNodePort,
		},
	}
}

func (ts *testSuite) envTesterDeployment(labels map[string]string) *appsTypes.Deployment {
	var replicas int32 = 1
	return &appsTypes.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: ts.testerDeploymentName,
		},
		Spec: appsTypes.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: &replicas,
			Template: k8sCoreTypes.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: k8sCoreTypes.PodSpec{
					Containers: []k8sCoreTypes.Container{
						{
							Name:  "app",
							Image: "eu.gcr.io/kyma-project/acceptance-tests:0.3.319",
							Ports: []k8sCoreTypes.ContainerPort{
								{
									Name:          "http",
									Protocol:      k8sCoreTypes.ProtocolTCP,
									ContainerPort: 8080,
								},
							},
							Command: []string{"/go/bin/env-tester.bin"},
						},
					},
				},
			},
		},
	}
}

func (ts *testSuite) assertInjectedEnvVariable(envName string, envValue string, timeout time.Duration) error {
	req := fmt.Sprintf("http://%s.%s.svc.cluster.local/envs?name=%s&value=%s", ts.testerDeploymentSvcName, ts.namespace, envName, envValue)

	err := repeatUntilTimeout(func() error {
		resp, err := http.Get(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("while checking if proper env is injected, received unexpected status code [got: %d, expected: %d]", resp.StatusCode, http.StatusOK)
		}

		return nil
	}, timeout)
	if err != nil {
		return err
	}

	return nil
}

func repeatUntilTimeout(fn func() error, timeout time.Duration) error {
	tickCh := time.Tick(time.Second)
	timeoutCh := time.After(timeout)
	var lastErr error

waitingLoop:
	for {
		select {
		case <-timeoutCh:
			return errors.Errorf("Waiting for resource failed in given timeout %v, last noticed error: %v", timeout, lastErr)
		case <-tickCh:
			if err := fn(); err != nil {
				lastErr = err
				continue
			}
			break waitingLoop
		}
	}

	return nil
}

func (t *E2EServiceCatalogHappyPathTest) appendErr(err error, errToAppend ...error) error {
	var msg []string
	for _, e := range errToAppend {
		msg = append(msg, e.Error())
	}

	if len(msg) == 0 {
		return err
	}
	msg = append(msg, err.Error())

	return errors.New(strings.Join(msg, ";"))
}
