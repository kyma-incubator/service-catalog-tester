package collector

import (
	"github.com/kyma-incubator/service-catalog-tester/internal/monitoring"
	"github.com/pkg/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/apps/v1"
)

// DeploymentConfig holds configuration for labels collector from given Deployments
type DeploymentConfig struct {
	Namespace string
	Names     []string
}

// CollectPodLabelsFromDeployments resolves and returns labels from requested Deployments
func CollectPodLabelsFromDeployments(appsCli v1.AppsV1Interface, requestedDeployments DeploymentConfig) (monitoring.Observable, error) {
	var labels []monitoring.Labels
	for _, deployName := range requestedDeployments.Names {

		d, err := appsCli.Deployments(requestedDeployments.Namespace).Get(deployName, metaV1.GetOptions{})
		if err != nil {
			return monitoring.Observable{}, errors.Wrapf(err, "while getting Deployment %q", deployName)
		}
		labels = append(labels, d.Spec.Template.Labels)
	}

	return monitoring.Observable{
		Namespace:       requestedDeployments.Namespace,
		PodLabelsGroups: labels,
	}, nil

}
