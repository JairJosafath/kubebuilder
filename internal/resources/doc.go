// Package resources is the Kubernetes adapter for a Superpod. It translates
// between the Superpod API and Kubernetes objects:
//
//   - New* functions build the desired children. They do not contact the API
//     server; pass a Superpod read from Kubernetes so its namespace and UID are set.
//   - NewConfigMap and StoredSelection keep the internal/webpage page and emoji
//     selection in a ConfigMap.
//   - Readiness judges the observed Pod and Ingress.
//   - Ensure creates or updates one child through the API server.
//
// Business rules belong in internal/webpage; see docs/architecture.md.
package resources
