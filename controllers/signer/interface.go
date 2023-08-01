/*
Copyright 2023 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package signer

import (
	"context"
	"crypto/x509"
	"time"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/cert-manager/issuer-lib/api/v1alpha1"
)

type Sign func(ctx context.Context, cr CertificateRequestObject, issuerObject v1alpha1.Issuer) ([]byte, error)
type Check func(ctx context.Context, issuerObject v1alpha1.Issuer) error

// CertificateRequestObject is an interface that represents either a
// cert-manager CertificateRequest or a Kubernetes CertificateSigningRequest
// resource. This interface hides the spec fields of the underlying resource
// and exposes a Certificate template and the raw CSR bytes instead. This
// allows the signer to be agnostic of the underlying resource type and also
// agnostic of the way the spec fields should be interpreted, such as the
// defaulting logic that is applied to it. It is still possible to access the
// labels and annotations of the underlying resource or any other metadata
// fields that might be useful to the signer. Also, the signer can use the
// GetConditions method to retrieve the conditions of the underlying resource.
// To update the conditions, the special error "SetCertificateRequestConditionError"
// can be returned from the Sign method.
type CertificateRequestObject interface {
	metav1.Object

	GetRequest() (template *x509.Certificate, duration time.Duration, csr []byte, err error)

	GetConditions() []cmapi.CertificateRequestCondition
}

// IgnoreIssuer is an optional function that can prevent the issuer controllers from
// reconciling an issuer resource. By default, the controllers will reconcile all
// issuer resources that match the owned types.
// This function will be called by the issuer reconcile loops for each type that matches
// the owned types. If the function returns true, the controller will not reconcile the
// issuer resource.
type IgnoreIssuer func(
	ctx context.Context,
	issuerObject v1alpha1.Issuer,
) (bool, error)

// IgnoreCertificateRequest is an optional function that can prevent the CertificateRequest
// and Kubernetes CSR controllers from reconciling a CertificateRequest resource. By default,
// the controllers will reconcile all CertificateRequest resources that match the issuerRef type.
// This function will be called by the CertificateRequest reconcile loop and the Kubernetes CSR
// reconcile loop for each type that matches the issuerRef type. If the function returns true,
// the controller will not reconcile the CertificateRequest resource.
type IgnoreCertificateRequest func(
	ctx context.Context,
	cr CertificateRequestObject,
	issuerGvk schema.GroupVersionKind,
	issuerName types.NamespacedName,
) (bool, error)
