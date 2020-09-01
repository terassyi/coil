package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	coilv2 "github.com/cybozu-go/coil/v2/api/v2"
	"github.com/cybozu-go/coil/v2/pkg/ipam"
)

// BlockRequestWatcher watches BlockRequest status on each node.
type BlockRequestWatcher struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	NodeIPAM ipam.NodeIPAM
}

// +kubebuilder:rbac:groups=coil.cybozu.com,resources=blockrequests,verbs=get;list;watch
// +kubebuilder:rbac:groups=coil.cybozu.com,resources=blockrequests/status,verbs=get

// Reconcile implements Reconcile interface.
// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.6.1/pkg/reconcile?tab=doc#Watcher
func (r *BlockRequestWatcher) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("blockrequest", req.Name)
	br := &coilv2.BlockRequest{}
	err := r.Client.Get(ctx, req.NamespacedName, br)

	if err != nil {
		// as Delete event is ignored, this is unlikely to happen.
		log.Error(err, "failed to get")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if len(br.Status.Conditions) == 0 {
		// as this case is excluded by the event filter, this should not happen.
		return ctrl.Result{}, nil
	}

	r.NodeIPAM.Notify(br.Spec.PoolName)
	return ctrl.Result{}, nil
}

// SetupWithManager registers this with the manager.
func (r *BlockRequestWatcher) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&coilv2.BlockRequest{}, builder.WithPredicates(predicate.Funcs{
			// predicate.Funcs returns true by default
			CreateFunc: func(ev event.CreateEvent) bool {
				// This needs to be the same as UpdateFunc because
				// sometimes updates can be merged into a create event.
				req := ev.Object.(*coilv2.BlockRequest)
				return len(req.Status.Conditions) > 0
			},
			UpdateFunc: func(ev event.UpdateEvent) bool {
				req := ev.ObjectNew.(*coilv2.BlockRequest)
				return len(req.Status.Conditions) > 0
			},
			DeleteFunc: func(event.DeleteEvent) bool {
				return false
			},
		})).
		Complete(r)
}