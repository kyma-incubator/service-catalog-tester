package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kyma-incubator/service-catalog-tester/internal/monitoring"
	"github.com/kyma-incubator/service-catalog-tester/internal/notifier"
	"github.com/kyma-incubator/service-catalog-tester/internal/platform/logger"
	"github.com/kyma-incubator/service-catalog-tester/internal/platform/signal"
	"github.com/kyma-incubator/service-catalog-tester/internal/runner"
	"github.com/kyma-incubator/service-catalog-tester/internal/tests"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vrischmann/envconfig"
	"k8s.io/client-go/informers"
	k8sClientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// informerResyncPeriod defines how often informer will execute relist action. Setting to zero disable resync.
// BEWARE: too short period time will increase the CPU load.
const informerResyncPeriod = 30 * time.Minute

// Config holds application configuration
type Config struct {
	Logger         logger.Config
	Port           int           `envconfig:"default=8080"`
	KubeconfigPath string        `envconfig:"optional"`
	Throttle       time.Duration `envconfig:"default=60s"`
	SlackClient    notifier.SlackClientConfig
	ClusterName    string
	Observable     struct {
		Namespace        string
		DeploymentsNames []string
	}
}

func main() {
	var cfg Config
	err := envconfig.InitWithPrefix(&cfg, "APP")
	fatalOnError(err, "while reading configuration from environment variables")

	log := logger.New(&cfg.Logger)

	// set up signals so we can handle the first shutdown signal gracefully
	stopCh := signal.SetupChannel()

	k8sConfig, err := clientcmd.BuildConfigFromFlags("", cfg.KubeconfigPath)
	fatalOnError(err, "while creating k8s config")

	// k8s informers
	k8sCli, err := k8sClientset.NewForConfig(k8sConfig)
	fatalOnError(err, "while creating k8s clientset")
	k8sInformersFactory := informers.NewSharedInformerFactoryWithOptions(k8sCli, informerResyncPeriod, informers.WithNamespace(cfg.Observable.Namespace))

	// Slack Notifier
	slackClient := notifier.NewSlackClient(cfg.SlackClient)
	msgRenderer, err := notifier.NewMessageRenderer()
	fatalOnError(err, "while creating Slack message renderer")

	sNotifier := notifier.New(cfg.ClusterName, slackClient, msgRenderer)

	// Ecosystem Monitor
	watchSvc := monitoring.NewWatcherService(k8sCli.CoreV1(), sNotifier, log)
	observable := monitoring.Observable{
		Namespace:        cfg.Observable.Namespace,
		DeploymentsNames: cfg.Observable.DeploymentsNames,
	}

	monitor := monitoring.NewEventMonitor(k8sCli.AppsV1(), k8sInformersFactory.Core().V1().Pods(), watchSvc, observable, log)

	// Test Runner
	testRunner := runner.NewStressTestRunner(sNotifier, log)
	E2EServiceCatalogHappyPath := tests.NewE2EServiceCatalogHappyPathTest(k8sConfig)

	// Start services
	err = monitor.Start()
	fatalOnError(err, "while starting resources monitoring")

	go testRunner.Run(stopCh, cfg.Throttle, E2EServiceCatalogHappyPath)

	// Start informers
	k8sInformersFactory.Start(stopCh)
	// Wait for cache sync
	k8sInformersFactory.WaitForCacheSync(stopCh)

	runStatuszHTTPServer(stopCh, fmt.Sprintf(":%d", cfg.Port), log)
}

func fatalOnError(err error, context string) {
	if err != nil {
		logrus.Fatal(errors.Wrap(err, context).Error())
	}
}

func runStatuszHTTPServer(stop <-chan struct{}, addr string, log logrus.FieldLogger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/statusz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-stop
		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Errorf("HTTP server Shutdown: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Errorf("HTTP server ListenAndServe: %v", err)
	}
}
