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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

    copy-file-to-pod [--kubeconfig file] [--namespace namespace] [--container container] [--pod-selector="app=remote-dst"] --pod pod file remote-path
`

// Copy file to a pod.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	kubeconfig := flag.String("kubeconfig", "", "path to the kubeconfig file")
	namespace := flag.String("namespace", "", "namespace of the pod to copy to")
	podName := flag.String("pod", "", "name of the pod to copy to")
	podSelector := flag.String("pod-selector", "", "label selector of the pod to copy to")
	container := flag.String("container", "", "container of the pod to copy to")

	flag.Usage = func() {
		fmt.Print(usage)
	}
	flag.Parse()

	if flag.NArg() != 2 {
		exit(usage)
	}

	// Enforce exactly one of --pod or --selector.
	if (*podName == "" && *podSelector == "") || (*podName != "" && *podSelector != "") {
		exit("you must specify exactly one of --pod or --pod-selector")
	}

	fileSrc := flag.Arg(0)
	fileDest := flag.Arg(1)

	kubeConfigFlags := genericclioptions.NewConfigFlags(false)
	kubeConfigFlags.KubeConfig = kubeconfig
	kubeConfigFlags.Namespace = namespace

	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	// Resolve the pod name.
	pod, err := podNameFromFlags(f, *podName, *podSelector, *namespace)
	if err != nil {
		exit("failed to resolve pod name: %v", err)
	}

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

func podNameFromFlags(f cmdutil.Factory, podName, podSelector, namespace string) (string, error) {
	if podName != "" {
		return podName, nil
	}

	cs, err := f.KubernetesClientSet()
	if err != nil {
		return "", err
	}

	pods, err := cs.CoreV1().Pods(namespace).List(
		context.Background(),
		metav1.ListOptions{LabelSelector: podSelector},
	)
	if err != nil {
		return "", fmt.Errorf("error listing pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found matching selector %q", podSelector)
	} else if len(pods.Items) > 1 {
		return "", fmt.Errorf("multiple pods found matching selector %q, use --pod instead", podSelector)
	}

	return pods.Items[0].Name, nil
}

func printOutput(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func exit(format string, a ...interface{}) {
	printOutput(format, a...)
	os.Exit(1)
}
