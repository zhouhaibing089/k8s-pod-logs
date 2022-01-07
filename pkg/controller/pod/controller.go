package pod

import (
	"bytes"
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/zhouhaibing089/k8s-pod-logs/pkg/storage"
)

type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	K8sClient    kubernetes.Interface
	NodeSelector map[string]string
	LogKeyFunc   func(map[string]interface{}) (string, error)
	Storage      storage.Interface
	Delete       bool

	serializer *json.Serializer
}

func (p *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var pod corev1.Pod
	if err := p.Get(ctx, req.NamespacedName, &pod); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch pod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("pod phase", "phase", pod.Status.Phase)
	// For pods that are not in Succeeded phase or Failed phase, there is no
	// need to process its logs.
	if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
		return ctrl.Result{}, nil
	}

	// validate node selector
	for key, value := range p.NodeSelector {
		pvalue, ok := pod.Spec.NodeSelector[key]
		if !ok || pvalue != value {
			return ctrl.Result{}, nil
		}
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

	podKey := logKey + "/pod.yaml"
	exists, err := p.Storage.Has(podKey)
	if err != nil {
		log.Error(err, "failed to check pod yaml existence", "key", podKey)
		return ctrl.Result{Requeue: true}, err
	}
	if !exists {
		var buf bytes.Buffer
		if err := p.serializer.Encode(&pod, &buf); err != nil {
			log.Error(err, "failed to encode pod as yaml")
			return ctrl.Result{Requeue: true}, err
		}
		if err := p.Storage.Put(podKey, buf.Bytes()); err != nil {
			log.Error(err, "failed to save pod yaml")
			return ctrl.Result{Requeue: true}, err
		}
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
			log.Error(err, "failed to save logs")
			return ctrl.Result{Requeue: true}, err
		}
		log.Info("logs saved", "key", key)
	}

	if p.Delete {
		if err := p.Client.Delete(ctx, &pod); err != nil {
			if errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			log.Error(err, "failed to delete")
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

func (p *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p.serializer = json.NewYAMLSerializer(json.DefaultMetaFactory, mgr.GetScheme(), mgr.GetScheme())
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(p)
}
