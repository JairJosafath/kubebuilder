package v1

// ReadyCondition is the condition type in SuperpodStatus.Conditions. It is True
// when the nginx Pod is ready and the Ingress has an address; it does not verify
// browser DNS or connectivity.
const ReadyCondition = "Ready"

// Reasons for the Ready condition. Users and tools match on these values, so
// they are part of the API: add new reasons, but do not rename existing ones.
const (
	// ResourcesReadyReason means the nginx Pod is ready and the Ingress has an address.
	ResourcesReadyReason = "ResourcesReady"
	// PodNotReadyReason means the nginx Pod is missing, not running, or failing its readiness probe.
	PodNotReadyReason = "PodNotReady"
	// IngressPendingReason means the Ingress controller has not published an address yet.
	IngressPendingReason = "IngressPending"
	// ResourceTerminatingReason means a child resource must finish deleting before it is recreated.
	ResourceTerminatingReason = "ResourceTerminating"
	// ReconcileFailedReason means a child resource could not be built, created, or updated.
	ReconcileFailedReason = "ReconcileFailed"
	// ObservationFailedReason means the Pod or Ingress could not be read to judge readiness.
	ObservationFailedReason = "ObservationFailed"
	// EmojiSelectionFailedReason means emoji selection failed; the plain page is served meanwhile.
	EmojiSelectionFailedReason = "EmojiSelectionFailed"
	// EmojiRateLimitedReason means the emoji provider asked the operator to wait before retrying.
	EmojiRateLimitedReason = "EmojiRateLimited"
)
