// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package helpers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/spf13/cobra"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	createbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/create/bundle"
)

func CreateBundle(t ginkgo.GinkgoTInterface, bundleFile, cfgFile string, platforms ...string) {
	platformFlags := make([]string, 0, len(platforms))
	for _, p := range platforms {
		platformFlags = append(platformFlags, "--platform", p)
	}

	createBundleCmd := NewCommand(t, createbundle.NewCommand)
	createBundleCmd.SetArgs(append([]string{
		"--output-file", bundleFile,
		"--images-file", cfgFile,
	}, platformFlags...))
	gomega.ExpectWithOffset(1, createBundleCmd.Execute()).To(gomega.Succeed())
}

func CreateBundleOCI(t ginkgo.GinkgoTInterface, bundleFile, cfgFile string) {
	createBundleCmd := NewCommand(t, createbundle.NewCommand)
	createBundleCmd.SetArgs([]string{
		"--output-file", bundleFile,
		"--oci-artifacts-file", cfgFile,
	})
	gomega.ExpectWithOffset(1, createBundleCmd.Execute()).To(gomega.Succeed())
}

func CreateBundleOCIAndImages(t ginkgo.GinkgoTInterface, bundleFile, ociArtifactsFiles string, imagesFile string, platforms ...string) {
	platformFlags := make([]string, 0, len(platforms))
	for _, p := range platforms {
		platformFlags = append(platformFlags, "--platform", p)
	}

	createBundleCmd := NewCommand(t, createbundle.NewCommand)
	createBundleCmd.SetArgs(append([]string{
		"--output-file", bundleFile,
		"--oci-artifacts-file", ociArtifactsFiles,
		"--images-file", imagesFile,
	}, platformFlags...))
	gomega.ExpectWithOffset(1, createBundleCmd.Execute()).To(gomega.Succeed())
}

func NewCommand(
	t ginkgo.GinkgoTInterface,
	newFn func(out output.Output) *cobra.Command,
) *cobra.Command {
	t.Helper()
	ctrllog.SetLogger(ginkgo.GinkgoLogr)
	cmd := newFn(output.NewNonInteractiveShell(ginkgo.GinkgoWriter, ginkgo.GinkgoWriter, 10))
	cmd.SilenceUsage = true
	return cmd
}

// GetFirstNonLoopbackIP returns the first non-loopback IP of the current host.
func GetFirstNonLoopbackIP(t ginkgo.GinkgoTInterface) net.IP {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	gomega.ExpectWithOffset(1, err).ToNot(gomega.HaveOccurred())
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP
			}
		}
	}
	ginkgo.Fail("no available non-loopback IP address")
	return net.IP{}
}

func WaitForTCPPort(t ginkgo.GinkgoTInterface, addr string, port int) {
	t.Helper()
	gomega.EventuallyWithOffset(1, func() error {
		conn, err := net.DialTimeout(
			"tcp",
			net.JoinHostPort(addr, strconv.Itoa(port)),
			1*time.Second,
		)
		if err != nil {
			return err
		}
		defer conn.Close()
		return nil
	}, 5*time.Second).Should(gomega.Succeed())
}

func GenerateCertificateAndKeyWithIPSAN(
	t ginkgo.GinkgoTInterface, destDir string, ipAddr net.IP,
) (caCertFile, caKeyFile, certFile, keyFile string) {
	t.Helper()

	caPriv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"d2iq", "mindthegap", "e2e-ca"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1 * time.Hour),

		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDerBytes, err := x509.CreateCertificate(
		rand.Reader,
		&caTemplate,
		&caTemplate,
		&caPriv.PublicKey,
		caPriv,
	)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

	caCertFile = filepath.Join(destDir, "ca.crt")
	caCertF, err := os.Create(caCertFile)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	defer caCertF.Close()
	gomega.ExpectWithOffset(1, pem.Encode(caCertF, &pem.Block{Type: "CERTIFICATE", Bytes: caDerBytes})).
		To(gomega.Succeed())

	b, err := x509.MarshalECPrivateKey(caPriv)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	pemBlock := pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	caKeyFile = filepath.Join(destDir, "ca.key")
	caKeyF, err := os.Create(caKeyFile)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	defer caKeyF.Close()
	gomega.ExpectWithOffset(1, pem.Encode(caKeyF, &pemBlock)).To(gomega.Succeed())

	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"d2iq", "mindthegap", "e2e"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.IPAddresses = append(template.IPAddresses, ipAddr)

	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&caTemplate,
		&priv.PublicKey,
		caPriv,
	)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

	certFile = filepath.Join(destDir, "tls.crt")
	certF, err := os.Create(certFile)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	defer certF.Close()
	gomega.ExpectWithOffset(1, pem.Encode(certF, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})).
		To(gomega.Succeed())

	b, err = x509.MarshalECPrivateKey(priv)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	pemBlock = pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	keyFile = filepath.Join(destDir, "tls.key")
	keyF, err := os.Create(keyFile)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	defer keyF.Close()
	gomega.ExpectWithOffset(1, pem.Encode(keyF, &pemBlock)).To(gomega.Succeed())

	return caCertFile, caKeyFile, certFile, keyFile
}

func ValidateImageIsAvailable(
	t ginkgo.GinkgoTInterface,
	addr string,
	port int,
	registryPath, image, tag string,
	platforms []*v1.Platform,
	forceOCIMediaTypes bool,
	opts ...remote.Option,
) {
	t.Helper()

	imagePath := path.Join(strings.TrimLeft(registryPath, "/"), image)
	imageName := fmt.Sprintf("%s:%d/%s:%s", addr, port, imagePath, tag)
	ref, err := name.ParseReference(imageName, name.StrictValidation)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	idx, err := remote.Index(
		ref,
		opts...,
	)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

	manifest, err := idx.IndexManifest()
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	if forceOCIMediaTypes {
		gomega.ExpectWithOffset(1, manifest.MediaType).To(gomega.Equal(types.OCIImageIndex))
	}

	gomega.ExpectWithOffset(1, manifest.Manifests).To(gomega.HaveLen(len(platforms)))

	for _, p := range platforms {
		gomega.ExpectWithOffset(1, manifest.Manifests).To(
			gomega.ContainElement(
				gstruct.MatchFields(
					gstruct.IgnoreExtras|gstruct.IgnoreMissing,
					gstruct.Fields{
						"Platform": gomega.Equal(p),
					},
				),
			),
		)
	}
}
