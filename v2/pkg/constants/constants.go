package constants

// annotation keys
const (
	AnnPool = "coil.cybozu.com/pool"
)

// Label keys
const (
	LabelPool = "coil.cybozu.com/pool"
	LabelNode = "coil.cybozu.com/node"
)

// Index keys
const (
	IndexController = ".metadata.controller"
)

// Finalizers
const (
	FinCoil = "coil.cybozu.com"
)

// Keys in CNI_ARGS
const (
	PodNameKey      = "K8S_POD_NAME"
	PodNamespaceKey = "K8S_POD_NAMESPACE"
	PodContainerKey = "K8S_POD_INFRA_CONTAINER_ID"
)

// Misc
const (
	DefaultPool = "default"
)