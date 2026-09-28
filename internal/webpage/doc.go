// Package webpage holds the Superpod business rules: what the webpage shows and
// when the emoji on it must be selected again.
//
// It is plain Go with no Kubernetes, controller-runtime, or HTTP dependencies.
// internal/resources stores its output in Kubernetes objects and
// internal/typesafe implements its EmojiSelector port, so these rules can be read
// and tested without a cluster or network. See docs/architecture.md.
package webpage
