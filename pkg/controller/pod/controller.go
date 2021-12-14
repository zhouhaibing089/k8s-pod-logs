package pod

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/zhouhaibing089/k8s-pod-logs/pkg/storage"
)

type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	K8sClient  kubernetes.Interface
	LogKeyFunc func(map[string]interface{}) (string, error)
	Storage    storage.Interface
}

func (p *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var pod corev1.Pod
	if err := p.Get(ctx, req.NamespacedName, &pod); err != nil {
		log.Error(err, "unable to fetch pod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("pod phase", "phase", pod.Status.Phase)
	// For pods that are not in Succeeded phase or Failed phase, there is no
	// need to process its logs.
	if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
		return ctrl.Result{}, nil
	}

	unstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
	if err != nil {
		log.Error(err, "failed to convert pod into unstructured")
		return ctrl.Result{Requeue: true}, err
	}

	logKey, err := p.LogKeyFunc(unstructured)
	if err != nil {
		log.Error(err, "failed to get log key")
		return ctrl.Result{Requeue: true}, err
	}

	for _, container := range pod.Spec.Containers {
		key := logKey + "/" + container.Name

		exists, err := p.Storage.Has(key)
		if err != nil {
			log.Error(err, "failed to check existence", "key", key)
			return ctrl.Result{Requeue: true}, err
		}
		// Already saved
		if exists {
			continue
		}

		logs, err := p.K8sClient.CoreV1().Pods(req.Namespace).GetLogs(req.Name, &corev1.PodLogOptions{
			Container: container.Name,
		}).DoRaw(ctx)
		if err != nil {
			log.Error(err, "failed to get log")
			return ctrl.Result{Requeue: true}, err
		}

		err = p.Storage.Put(key, logs)
		if err != nil {
			log.Error(err, "failed to put logs")
			return ctrl.Result{Requeue: true}, err
		}
		log.Info("logs saved", "key", key)
	}

	return ctrl.Result{}, nil
}

func (p *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(p)
}
