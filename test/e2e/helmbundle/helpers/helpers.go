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
	"path/filepath"
	"strconv"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"

	"github.com/mesosphere/dkp-cli-runtime/core/output"

	createbundle "github.com/mesosphere/mindthegap/cmd/mindthegap/create/bundle"
	"github.com/mesosphere/mindthegap/helm"
)

func CreateBundle(t ginkgo.GinkgoTInterface, bundleFile, cfgFile string) {
	createBundleCmd := NewCommand(t, createbundle.NewCommand)
	createBundleCmd.SetArgs([]string{
		"--output-file", bundleFile,
		"--helm-charts-file", cfgFile,
	})
	gomega.ExpectWithOffset(1, createBundleCmd.Execute()).To(gomega.Succeed())
}

func NewCommand(
	t ginkgo.GinkgoTInterface,
	newFn func(out output.Output) *cobra.Command,
) *cobra.Command {
	t.Helper()
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

func ValidateChartIsAvailable(
	t ginkgo.GinkgoTInterface,
	addr string,
	port int,
	chartName, chartVersion string,
	pullOpts ...action.PullOpt,
) {
	t.Helper()
	h, cleanup := helm.NewClient(
		output.NewNonInteractiveShell(ginkgo.GinkgoWriter, ginkgo.GinkgoWriter, 10),
	)
	ginkgo.DeferCleanup(cleanup)

	helmTmpDir := t.TempDir()

	d, err := h.GetChartFromRepo(
		helmTmpDir,
		"",
		fmt.Sprintf("%s://%s:%d/charts/%s", helm.OCIScheme, addr, port, chartName),
		chartVersion,
		[]helm.ConfigOpt{helm.RegistryClientConfigOpt()},
		pullOpts...,
	)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	chrt, err := helm.LoadChart(d)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
	gomega.ExpectWithOffset(1, chrt.Metadata.Name).To(gomega.Equal(chartName))
	gomega.ExpectWithOffset(1, chrt.Metadata.Version).To(gomega.Equal(chartVersion))
}
