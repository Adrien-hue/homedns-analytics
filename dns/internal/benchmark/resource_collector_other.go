//go:build !linux

package benchmark

// NewResourceCollector returns a no-op implementation on platforms where the
// Linux procfs and Raspberry Pi resource interfaces are unavailable.
func NewResourceCollector() ResourceCollector {
	return NoopResourceCollector{}
}
