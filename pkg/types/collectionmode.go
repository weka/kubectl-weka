package types

// CollectionMode defines which collectors to run
type CollectionMode string

const (
	CollectionModeOperator CollectionMode = "operator"
	CollectionModeCluster  CollectionMode = "cluster"
	CollectionModeClient   CollectionMode = "client"
	CollectionModeCSI      CollectionMode = "csi"
	CollectionModeK8s      CollectionMode = "k8s"
	CollectionModeAll      CollectionMode = "all"
)
