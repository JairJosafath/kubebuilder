// Package typesafe adapts TypeSafe's Jev API to the webpage.EmojiSelector port.
//
// Everything specific to the provider lives here: the endpoint and credentials,
// request limits, how the catalog is split into Choice questions, rate limiting,
// and the in-process decision cache. Replacing the provider means replacing this
// package; internal/webpage and internal/emoji stay unchanged.
package typesafe
