// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/kubectl/pkg/cmd/cp"
	"k8s.io/kubectl/pkg/cmd/exec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const usage = `copy-file-to-pod is a simple utility that atomically copies a file to a pod and then
exits. This is different to kubectl cp which copies a file to the destination file in a pod
directly, whereas this utility copies the file to a temporary file in the pod and then renames it to
the destination file.

Usage:

    copy-file-to-pod [--kubeconfig file] [--namespace namespace] [--container container] file pod:/path
`

// Copy file to a pod.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	kubeconfig := flag.String("kubeconfig", "", "path to the kubeconfig file")
	namespace := flag.String("namespace", "", "namespace of the pod to copy to")
	container := flag.String("container", "", "container of the pod to copy to")

	flag.Usage = func() {
		fmt.Print(usage)
	}
	flag.Parse()

	if flag.NArg() != 2 {
		exit(usage)
	}

	fileSrc := flag.Arg(0)
	podAndFileDest := flag.Arg(1)

	pod, fileDest, found := strings.Cut(podAndFileDest, ":")
	if !found {
		exit(usage)
	}

	kubeConfigFlags := genericclioptions.NewConfigFlags(false)
	kubeConfigFlags.KubeConfig = kubeconfig
	kubeConfigFlags.Namespace = namespace

	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	var rootKubectlArgs []string
	if *namespace != "" {
		rootKubectlArgs = append(rootKubectlArgs, "--namespace", *namespace)
	}
	if *kubeconfig != "" {
		rootKubectlArgs = append(rootKubectlArgs, "--kubeconfig", *kubeconfig)
	}

	cpCmd := cp.NewCmdCp(
		f,
		genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	)
	kubeConfigFlags.AddFlags(cpCmd.Flags())
	matchVersionKubeConfigFlags.AddFlags(cpCmd.Flags())

	cpCmdArgs := slices.Clone(rootKubectlArgs)
	if *container != "" {
		cpCmdArgs = append(cpCmdArgs, "--container", *container)
	}
	cpCmdArgs = append(cpCmdArgs, fileSrc, fmt.Sprintf("%s:%s.tmp", pod, fileDest))

	cpCmd.SetArgs(cpCmdArgs)
	if err := cpCmd.ExecuteContext(ctx); err != nil {
		exit("failed to copy file to pod: %v", err)
	}

	execCmd := exec.NewCmdExec(
		f,
		genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	)
	kubeConfigFlags.AddFlags(execCmd.Flags())
	matchVersionKubeConfigFlags.AddFlags(execCmd.Flags())

	execCmdArgs := slices.Clone(rootKubectlArgs)
	execCmdArgs = append(execCmdArgs, pod)
	if *container != "" {
		execCmdArgs = append(execCmdArgs, "--container", *container)
	}
	execCmd.SetArgs(append(execCmdArgs, "--", "mv", fmt.Sprintf("%s.tmp", fileDest), fileDest))
	if err := execCmd.ExecuteContext(ctx); err != nil {
		exit("failed to rename file in pod: %v", err)
	}

	printOutput("successfully copied %s to %s:%s", fileSrc, pod, fileDest)
}

func printOutput(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func exit(format string, a ...interface{}) {
	printOutput(format, a...)
	os.Exit(1)
}
