package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/itchyny/gojq"
	"github.com/zhouhaibing089/k8s-pod-logs/pkg/controller/pod"
	"github.com/zhouhaibing089/k8s-pod-logs/pkg/storage/s3"
)

var (
	setupLog = ctrl.Log.WithName("setup")

	namespace string
	s3CfgPath string
	logKey    string
)

func init() {
	flag.StringVar(&namespace, "namespace", "", "The namespace to watch")
	flag.StringVar(&s3CfgPath, "s3-config-path", "", "Path to s3 configuration file")
	flag.StringVar(&logKey, "log-key", `.metadata.namespace + "/" + .metadata.name`, "The default query on pod to generate log key")
}

func main() {
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:    scheme.Scheme,
		Namespace: namespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to new manager")
		os.Exit(1)
	}

	if s3CfgPath == "" {
		setupLog.Info("flag --s3-config-path is not set")
		os.Exit(1)
	}
	storage, err := s3.New(s3CfgPath)
	if err != nil {
		setupLog.Error(err, "failed to new s3 storage")
		os.Exit(1)
	}

	// We need a k8s client to fetch pod logs. The built-in client from
	// controller-runtime doesn't support this.
	k8sclient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "failed to new kubernetes client")
		os.Exit(1)
	}

	query, err := gojq.Parse(logKey)
	if err != nil {
		setupLog.Error(err, "failed to parse log key")
		os.Exit(1)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		setupLog.Error(err, "failed to compile query")
		os.Exit(1)
	}

	if err = (&pod.PodReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		K8sClient: k8sclient,
		LogKeyFunc: func(in map[string]interface{}) (string, error) {
			iter := code.Run(in)
			for {
				value, ok := iter.Next()
				if !ok {
					break
				}
				if err, ok := value.(error); ok {
					return "", fmt.Errorf("failed to iterate query: %s", err)
				}
				key, ok := value.(string)
				if !ok {
					return "", fmt.Errorf("unexpected value type: %T", value)
				}
				return key, nil
			}
			return "", fmt.Errorf("no value")
		},
		Storage: storage,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
}
