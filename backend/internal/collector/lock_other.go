//go:build !unix

package collector

// lockFile is a no-op outside unix: the collector only runs in Linux
// containers, and this keeps the package building elsewhere.
func lockFile(string) (func(), error) {
	return func() {}, nil
}
