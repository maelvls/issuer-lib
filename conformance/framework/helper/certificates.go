/*
Copyright 2020 The cert-manager Authors.

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

package helper

import (
	"context"
	"crypto/x509"
	"fmt"
	"sort"
	"time"

	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	apiutil "github.com/cert-manager/cert-manager/pkg/api/util"
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	clientset "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/typed/certmanager/v1"
	"github.com/cert-manager/issuer-lib/conformance/framework/log"
)

// WaitForCertificateToExist waits for the named certificate to exist and returns the certificate
func (h *Helper) WaitForCertificateToExist(namespace string, name string, timeout time.Duration) (*v1.Certificate, error) {
	client := h.CMClient.CertmanagerV1().Certificates(namespace)
	var certificate *v1.Certificate
	logf, done := log.LogBackoff()
	defer done()

	pollErr := wait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		logf("Waiting for Certificate %v to exist", name)
		var err error
		certificate, err = client.Get(context.TODO(), name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("error getting Certificate %v: %v", name, err)
		}

		return true, nil
	})
	return certificate, pollErr
}

func (h *Helper) waitForCertificateCondition(client clientset.CertificateInterface, name string, check func(*v1.Certificate) bool, timeout time.Duration) (*v1.Certificate, error) {
	var certificate *v1.Certificate
	pollErr := wait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		var err error
		certificate, err = client.Get(context.TODO(), name, metav1.GetOptions{})
		if nil != err {
			certificate = nil
			return false, fmt.Errorf("error getting Certificate %v: %v", name, err)
		}

		return check(certificate), nil
	})

	return certificate, pollErr
}

// WaitForCertificateReadyAndDoneIssuing waits for the certificate resource to be in a Ready=True state and not be in an Issuing state.
// The Ready=True condition will be checked against the provided certificate to make sure that it is up-to-date (condition gen. >= cert gen.).
func (h *Helper) WaitForCertificateReadyAndDoneIssuing(cert *v1.Certificate, timeout time.Duration) (*v1.Certificate, error) {
	ready_true_condition := v1.CertificateCondition{
		Type:               v1.CertificateConditionReady,
		Status:             cmmeta.ConditionTrue,
		ObservedGeneration: cert.Generation,
	}
	issuing_true_condition := v1.CertificateCondition{
		Type:   v1.CertificateConditionIssuing,
		Status: cmmeta.ConditionTrue,
	}
	logf, done := log.LogBackoff()
	defer done()
	return h.waitForCertificateCondition(h.CMClient.CertmanagerV1().Certificates(cert.Namespace), cert.Name, func(certificate *v1.Certificate) bool {
		if !apiutil.CertificateHasConditionWithObservedGeneration(certificate, ready_true_condition) {
			logf(
				"Expected Certificate %v condition %v=%v (generation >= %v) but it has: %v",
				certificate.Name,
				ready_true_condition.Type,
				ready_true_condition.Status,
				ready_true_condition.ObservedGeneration,
				certificate.Status.Conditions,
			)
			return false
		}

		if apiutil.CertificateHasCondition(certificate, issuing_true_condition) {
			logf("Expected Certificate %v condition %v to be missing but it has: %v", certificate.Name, issuing_true_condition.Type, certificate.Status.Conditions)
			return false
		}

		if certificate.Status.NextPrivateKeySecretName != nil {
			logf("Expected Certificate %v 'next-private-key-secret-name' attribute to be empty but has: %v", certificate.Name, *certificate.Status.NextPrivateKeySecretName)
			return false
		}

		return true
	}, timeout)
}

// WaitForCertificateNotReadyAndDoneIssuing waits for the certificate resource to be in a Ready=False state and not be in an Issuing state.
// The Ready=False condition will be checked against the provided certificate to make sure that it is up-to-date (condition gen. >= cert gen.).
func (h *Helper) WaitForCertificateNotReadyAndDoneIssuing(cert *v1.Certificate, timeout time.Duration) (*v1.Certificate, error) {
	ready_false_condition := v1.CertificateCondition{
		Type:               v1.CertificateConditionReady,
		Status:             cmmeta.ConditionFalse,
		ObservedGeneration: cert.Generation,
	}
	issuing_true_condition := v1.CertificateCondition{
		Type:   v1.CertificateConditionIssuing,
		Status: cmmeta.ConditionTrue,
	}
	logf, done := log.LogBackoff()
	defer done()
	return h.waitForCertificateCondition(h.CMClient.CertmanagerV1().Certificates(cert.Namespace), cert.Name, func(certificate *v1.Certificate) bool {
		if !apiutil.CertificateHasConditionWithObservedGeneration(certificate, ready_false_condition) {
			logf(
				"Expected Certificate %v condition %v=%v (generation >= %v) but it has: %v",
				certificate.Name,
				ready_false_condition.Type,
				ready_false_condition.Status,
				ready_false_condition.ObservedGeneration,
				certificate.Status.Conditions,
			)
			return false
		}

		if apiutil.CertificateHasCondition(certificate, issuing_true_condition) {
			logf("Expected Certificate %v condition %v to be missing but it has: %v", certificate.Name, issuing_true_condition.Type, certificate.Status.Conditions)
			return false
		}

		if certificate.Status.NextPrivateKeySecretName != nil {
			logf("Expected Certificate %v 'next-private-key-secret-name' attribute to be empty but has: %v", certificate.Name, *certificate.Status.NextPrivateKeySecretName)
			return false
		}

		return true
	}, timeout)
}

func (h *Helper) waitForIssuerCondition(client clientset.IssuerInterface, name string, check func(issuer *v1.Issuer) bool, timeout time.Duration) (*v1.Issuer, error) {
	var issuer *v1.Issuer
	pollErr := wait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		var err error
		issuer, err = client.Get(context.TODO(), name, metav1.GetOptions{})
		if nil != err {
			issuer = nil
			return false, fmt.Errorf("error getting Issuer %v: %v", name, err)
		}
		return check(issuer), nil
	})

	if pollErr != nil && issuer != nil {
		log.Logf("Failed waiting for issuer %v :%v\n", name, pollErr.Error())
	}

	return issuer, pollErr
}

// WaitIssuerReady waits for the Issuer resource to be in a Ready=True state
// The Ready=True condition will be checked against the provided issuer to make sure its ready.
func (h *Helper) WaitIssuerReady(issuer *v1.Issuer, timeout time.Duration) (*v1.Issuer, error) {
	ready_true_condition := v1.IssuerCondition{
		Type:   v1.IssuerConditionReady,
		Status: cmmeta.ConditionTrue,
	}

	logf, done := log.LogBackoff()
	defer done()
	return h.waitForIssuerCondition(h.CMClient.CertmanagerV1().Issuers(issuer.Namespace), issuer.Name, func(issuer *v1.Issuer) bool {
		if !apiutil.IssuerHasCondition(issuer, ready_true_condition) {
			logf(
				"Expected Issuer %v condition %v=%v but it has: %v",
				issuer.Name,
				ready_true_condition.Type,
				ready_true_condition.Status,
				issuer.Status.Conditions,
			)
			return false
		}
		return true
	}, timeout)
}

func (h *Helper) waitForClusterIssuerCondition(client clientset.ClusterIssuerInterface, name string, check func(issuer *v1.ClusterIssuer) bool, timeout time.Duration) (*v1.ClusterIssuer, error) {
	var issuer *v1.ClusterIssuer
	pollErr := wait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		var err error
		issuer, err = client.Get(context.TODO(), name, metav1.GetOptions{})
		if nil != err {
			issuer = nil
			return false, fmt.Errorf("error getting Issuer %v: %v", name, err)
		}
		return check(issuer), nil
	})

	if pollErr != nil && issuer != nil {
		log.Logf("Failed waiting for issuer %v :%v\n", name, pollErr.Error())
	}

	return issuer, pollErr
}

// WaitClusterIssuerReady waits for the Cluster Issuer resource to be in a Ready=True state
// The Ready=True condition will be checked against the provided issuer to make sure its ready.
func (h *Helper) WaitClusterIssuerReady(issuer *v1.ClusterIssuer, timeout time.Duration) (*v1.ClusterIssuer, error) {
	ready_true_condition := v1.IssuerCondition{
		Type:   v1.IssuerConditionReady,
		Status: cmmeta.ConditionTrue,
	}
	logf, done := log.LogBackoff()
	defer done()
	return h.waitForClusterIssuerCondition(h.CMClient.CertmanagerV1().ClusterIssuers(), issuer.Name, func(issuer *v1.ClusterIssuer) bool {
		if !apiutil.IssuerHasCondition(issuer, ready_true_condition) {
			logf(
				"Expected Cluster Issuer %v condition %v=%v but it has: %v",
				issuer.Name,
				ready_true_condition.Type,
				ready_true_condition.Status,
				issuer.Status.Conditions,
			)
			return false
		}
		return true
	}, timeout)
}

func (h *Helper) deduplicateExtKeyUsages(us []x509.ExtKeyUsage) []x509.ExtKeyUsage {
	extKeyUsagesMap := make(map[x509.ExtKeyUsage]bool)
	for _, e := range us {
		extKeyUsagesMap[e] = true
	}

	us = make([]x509.ExtKeyUsage, 0)
	for e, ok := range extKeyUsagesMap {
		if ok {
			us = append(us, e)
		}
	}

	return us
}

func (h *Helper) defaultKeyUsagesToAdd(ns string, issuerRef *cmmeta.ObjectReference) (x509.KeyUsage, []x509.ExtKeyUsage, error) {
	var issuerSpec *v1.IssuerSpec
	switch issuerRef.Kind {
	case "ClusterIssuer":
		issuerObj, err := h.CMClient.CertmanagerV1().ClusterIssuers().Get(context.TODO(), issuerRef.Name, metav1.GetOptions{})
		if err != nil {
			return 0, nil, fmt.Errorf("failed to find referenced ClusterIssuer %v: %s",
				issuerRef, err)
		}

		issuerSpec = &issuerObj.Spec
	default:
		issuerObj, err := h.CMClient.CertmanagerV1().Issuers(ns).Get(context.TODO(), issuerRef.Name, metav1.GetOptions{})
		if err != nil {
			return 0, nil, fmt.Errorf("failed to find referenced Issuer %v: %s",
				issuerRef, err)
		}

		issuerSpec = &issuerObj.Spec
	}

	var keyUsages x509.KeyUsage
	var extKeyUsages []x509.ExtKeyUsage

	// Vault and ACME issuers will add server auth and client auth extended key
	// usages by default so we need to add them to the list of expected usages
	if issuerSpec.ACME != nil || issuerSpec.Vault != nil {
		extKeyUsages = append(extKeyUsages, x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth)
	}

	// Vault issuers will add key agreement key usage
	if issuerSpec.Vault != nil {
		keyUsages |= x509.KeyUsageKeyAgreement
	}

	// Venafi issue adds server auth key usage
	if issuerSpec.Venafi != nil {
		extKeyUsages = append(extKeyUsages, x509.ExtKeyUsageServerAuth)
	}

	return keyUsages, extKeyUsages, nil
}

func (h *Helper) keyUsagesMatch(aKU x509.KeyUsage, aEKU []x509.ExtKeyUsage,
	bKU x509.KeyUsage, bEKU []x509.ExtKeyUsage) bool {
	if aKU != bKU {
		return false
	}

	if len(aEKU) != len(bEKU) {
		return false
	}

	sort.SliceStable(aEKU, func(i, j int) bool {
		return aEKU[i] < aEKU[j]
	})

	sort.SliceStable(bEKU, func(i, j int) bool {
		return bEKU[i] < bEKU[j]
	})

	for i := range aEKU {
		if aEKU[i] != bEKU[i] {
			return false
		}
	}

	return true
}
