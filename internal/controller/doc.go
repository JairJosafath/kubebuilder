// Package controller contains the Kubebuilder-scaffolded Superpod reconciler.
//
// The reconciler is a thin adapter between controller-runtime and the rest of
// the operator: it reads the Superpod, asks internal/webpage and
// internal/resources what should exist, applies it, and reports status. Business
// rules belong in those packages, not here; see docs/architecture.md.
package controller
