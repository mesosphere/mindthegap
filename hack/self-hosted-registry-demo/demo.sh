#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

GIT_ROOT="$(git rev-parse --show-toplevel)"
readonly GIT_ROOT

MINDTHEGAP_VERSION="$(gojq -r .version "${GIT_ROOT}/dist/metadata.json")"
readonly MINDTHEGAP_VERSION

IMAGE_VERSION="${MINDTHEGAP_VERSION}-$(go env GOARCH)"
readonly IMAGE_VERSION

# Build the image bundle
"${GIT_ROOT}/dist/mindthegap_$(go env GOOS)_$(go env GOARCH)/mindthegap" \
  create bundle \
  --images-file "${SCRIPT_DIR}/bootstrap-images.txt" \
  --output-file "${SCRIPT_DIR}/bundle.tar" \
  --platform linux/"$(go env GOARCH)" \
  --image-pull-concurrency 20 \
  --overwrite

export KUBECONFIG="${SCRIPT_DIR}/kubeconfig"

# Create the kind cluster, mounting the bundle into the control-plane node of the KinD cluster
kind create cluster --name self-hosted-registry-demo --config <(
  cat <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: "${SCRIPT_DIR}/bundle.tar"
        containerPath: /registry-data/bundle.tar
  - role: worker
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
networking:
  # the default CNI will not be installed
  disableDefaultCNI: true
EOF
)

# Show that the nodes are not Ready - no CNI has been installed yet (default kindnet has been disabled above).
kubectl get nodes

# Load the necessary images into the KinD cluster - this could be done by building the KinD node image
# with the necessary images, but this is a simpler way to demonstrate the concept.
kind load docker-image --name self-hosted-registry-demo --quiet \
  ko.local/{mindthegap,wait-for-file-to-exist,copy-file-to-pod}:"${IMAGE_VERSION}"

# Configure containerd on each node to use the in-cluster registry service address - this will
# ensure that the images are only pulled from the in-cluster registry service.
readonly DEFAULT_REGISTRY_DIR="/etc/containerd/certs.d/_default"
for node in $(kind get nodes --name self-hosted-registry-demo); do
  docker container exec "${node}" mkdir -p "${DEFAULT_REGISTRY_DIR}"
  cat <<EOF | docker container exec -i "${node}" tee "${DEFAULT_REGISTRY_DIR}/hosts.toml" >/dev/null
server = "http://10.96.0.20"

# See https://github.com/containerd/containerd/issues/10027 for containerd bug that requires this workaround,
# having an empty host tree here.
[host]
EOF
done

# Create the registry pod.
cat <<EOF | kubectl apply --server-side -n kube-system -f -
apiVersion: v1
kind: Pod
metadata:
  namespace: kube-system
  name: temporary-registry
  labels:
    app: in-cluster-registry
spec:
  tolerations:
    - operator: Exists
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: node-role.kubernetes.io/control-plane
                operator: Exists
  hostNetwork: true
  initContainers:
    - name: wait
      image: ko.local/wait-for-file-to-exist:${IMAGE_VERSION}
      args:
        - /registry-data/bundle.tar
      volumeMounts:
        - name: shared
          mountPath: /registry-data
  containers:
    - name: registry
      image: ko.local/mindthegap:${IMAGE_VERSION}
      args:
        - serve
        - bundle
        - --bundle
        - /registry-data/bundle.tar
        - --listen-address
        - "0.0.0.0"
        - --listen-port
        - "5000"
      volumeMounts:
        - name: shared
          mountPath: /registry-data
      ports:
        - name: http
          containerPort: 5000
  restartPolicy: Never
  volumes:
    - name: shared
      emptyDir: {}
EOF

# And the registry service.
cat <<EOF | kubectl apply --server-side -n kube-system -f -
apiVersion: v1
kind: Service
metadata:
  namespace: kube-system
  name: registry
spec:
  selector:
    app: in-cluster-registry
  clusterIP: 10.96.0.20
  ports:
    - name: registry
      port: 80
      targetPort: http
EOF

# Copy the bundle to the registry pod via a job.
cat <<EOF | kubectl apply --server-side -n kube-system -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: kube-system
  name: registry-seeder
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: kube-system
  name: registry-seeder
rules:
  - apiGroups:
      - ""
    resources:
      - pods
    resourceNames:
      - temporary-registry
    verbs:
      - get
  - apiGroups:
      - ""
    resources:
      - pods/exec
    resourceNames:
      - temporary-registry
    verbs:
      - create
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: kube-system
  name: registry-seeder
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: registry-seeder
subjects:
  - kind: ServiceAccount
    name: registry-seeder
    namespace: kube-system
---
apiVersion: batch/v1
kind: Job
metadata:
  namespace: kube-system
  name: copy-bundle-to-registry
spec:
  ttlSecondsAfterFinished: 10
  template:
    spec:
      tolerations:
        - operator: Exists
      serviceAccountName: registry-seeder
      restartPolicy: OnFailure
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/control-plane
                    operator: Exists
      hostNetwork: true
      containers:
        - name: copy
          image: ko.local/copy-file-to-pod:${IMAGE_VERSION}
          args:
            - --namespace
            - kube-system
            - --pod
            - temporary-registry
            - --container
            - wait
            - /registry-data/bundle.tar
            - /registry-data/bundle.tar
          volumeMounts:
            - name: bundle
              mountPath: /registry-data/bundle.tar
      volumes:
        - name: bundle
          hostPath:
            path: "/registry-data/bundle.tar"
            type: File
EOF

# Install the CNI - Cilium in this case.
helm upgrade --install \
  cilium https://helm.cilium.io/cilium-1.16.2.tgz \
  --namespace kube-system \
  --wait --wait-for-jobs \
  --values <(
    cat <<EOF
cni:
  chainingMode: portmap
  exclusive: false
hubble:
  enabled: false
  relay:
    enabled: false
ipam:
  mode: kubernetes
image:
  useDigest: false
operator:
  image:
    useDigest: false
certgen:
  image:
    useDigest: false
socketLB:
  hostNamespaceOnly: true
envoy:
  image:
    useDigest: false
EOF
  )

# Show that the nodes are Ready - the CNI has been installed and the nodes are now Ready.
kubectl wait --for=condition=Ready nodes --all
kubectl get nodes

# Show that only internal images can be pulled - this will time out showing that the images are not available
# from the internal registry and that containerd is configured so as not to pull from there.
kubectl run busybox \
  --image=cgr.dev/chainguard/busybox \
  --rm --attach --restart=Never \
  --pod-running-timeout=30s \
  --command -- /bin/sh -c 'echo "Hello, world!"' &&
  echo "If you're seeing this then things are not configured correctly - this was supposed to fail!" ||
  echo "Yay everything worked as expected - this was supposed to fail!"
