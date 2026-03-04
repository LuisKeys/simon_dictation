package input

// Send enqueues the given text to be typed asynchronously by the internal
// serialized sender. For a blocking send (wait until typing finishes) use
// `SendSync` from this package.
func Send(text string) {
	Enqueue(text)
}
