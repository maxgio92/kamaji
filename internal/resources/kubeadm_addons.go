// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubeadmAddon int

const (
	AddonCoreDNS KubeadmAddon = iota
	AddonKubeProxy
)

func (d KubeadmAddon) String() string {
	return [...]string{"PhaseAddonCoreDNS", "PhaseAddonKubeProxy"}[d]
}

type KubeadmAddonResource struct {
	Client                client.Client
	Name                  string
	KubeadmAddon          KubeadmAddon
	kubeadmConfigChecksum string
}

func (r *KubeadmAddonResource) isStatusEqual(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	i, err := r.GetStatus(tenantControlPlane)
	if err != nil {
		return false
	}

	addonStatus, ok := i.(*kamajiv1alpha1.AddonStatus)
	if !ok {
		return false
	}

	return addonStatus.Checksum == r.kubeadmConfigChecksum
}

func (r *KubeadmAddonResource) SetKubeadmConfigChecksum(checksum string) {
	r.kubeadmConfigChecksum = checksum
}

func (r *KubeadmAddonResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return !r.isStatusEqual(tenantControlPlane)
}

func (r *KubeadmAddonResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	ok, err := r.getSpec(tenantControlPlane)
	if err != nil {
		return false
	}

	return ok
}

func (r *KubeadmAddonResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName(), "addon", r.KubeadmAddon.String())

	client, err := utilities.GetTenantClientSet(ctx, r.Client, tenantControlPlane)
	if err != nil {
		logger.Error(err, "cannot generate Tenant client")

		return false, err
	}

	fun, err := r.getRemoveAddonFunction()
	if err != nil {
		logger.Error(err, "cannot get the remove addon function")

		return false, err
	}

	if err := fun(ctx, client); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "error while performing clean-up")

			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *KubeadmAddonResource) Define(context.Context, *kamajiv1alpha1.TenantControlPlane) error {
	return nil
}

func (r *KubeadmAddonResource) GetKubeadmFunction() (func(clientset.Interface, *kubeadm.Configuration) error, error) {
	switch r.KubeadmAddon {
	case AddonCoreDNS:
		return kubeadm.AddCoreDNS, nil
	case AddonKubeProxy:
		return kubeadm.AddKubeProxy, nil

	default:
		return nil, fmt.Errorf("no available functionality for phase %s", r.KubeadmAddon)
	}
}

func (r *KubeadmAddonResource) getRemoveAddonFunction() (func(context.Context, clientset.Interface) error, error) {
	switch r.KubeadmAddon {
	case AddonCoreDNS:
		return kubeadm.RemoveCoreDNSAddon, nil
	case AddonKubeProxy:
		return kubeadm.RemoveKubeProxy, nil
	default:
		return nil, fmt.Errorf("no available functionality for removing addon %s", r.KubeadmAddon)
	}
}

func (r *KubeadmAddonResource) GetClient() client.Client {
	return r.Client
}

func (r *KubeadmAddonResource) GetTmpDirectory() string {
	return ""
}

func (r *KubeadmAddonResource) GetName() string {
	return r.Name
}

func (r *KubeadmAddonResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName(), "addon", r.KubeadmAddon.String())

	status, err := r.GetStatus(tenantControlPlane)
	if err != nil {
		logger.Error(err, "cannot update Tenant Control Plane status")

		return err
	}

	status.SetChecksum(r.kubeadmConfigChecksum)

	return nil
}

func (r *KubeadmAddonResource) GetStatus(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (kamajiv1alpha1.KubeadmConfigChecksumDependant, error) {
	switch r.KubeadmAddon {
	case AddonCoreDNS:
		return &tenantControlPlane.Status.Addons.CoreDNS, nil
	case AddonKubeProxy:
		return &tenantControlPlane.Status.Addons.KubeProxy, nil
	default:
		return nil, fmt.Errorf("%s has no addon status", r.KubeadmAddon)
	}
}

func (r *KubeadmAddonResource) getSpec(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	switch r.KubeadmAddon {
	case AddonCoreDNS:
		return tenantControlPlane.Spec.Addons.CoreDNS == nil, nil
	case AddonKubeProxy:
		return tenantControlPlane.Spec.Addons.KubeProxy == nil, nil
	default:
		return false, fmt.Errorf("%s has no spec", r.KubeadmAddon)
	}
}

func (r *KubeadmAddonResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	logger := log.FromContext(ctx, "resource", r.GetName(), "addon", r.KubeadmAddon.String())

	return KubeadmPhaseCreate(ctx, r, logger, tenantControlPlane)
}
