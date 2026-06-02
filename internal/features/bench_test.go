package features_test

import (
	"testing"

	logtest "github.com/sirupsen/logrus/hooks/test"

	"go.k6.io/k6/v2/internal/features"
)

// BenchmarkResolve measures the one-time startup resolution cost. It is for
// regression detection only; there is no absolute-ns gate (sub-ns noise makes
// that unreliable, per the design).
func BenchmarkResolve(b *testing.B) {
	logger, _ := logtest.NewNullLogger()
	cli := features.SurfaceInput{Values: []string{"native-histograms"}, Supplied: true}

	b.ReportAllocs()
	for range b.N {
		reg, err := features.Bootstrap(&features.Flags{})
		if err != nil {
			b.Fatal(err)
		}
		reg.Resolve(cli, features.SurfaceInput{}, features.SurfaceInput{}, logger)
	}
}

// BenchmarkGatedFieldRead measures the hot-path access pattern: a direct struct
// field read with no map lookup, allocation, or lock.
func BenchmarkGatedFieldRead(b *testing.B) {
	logger, _ := logtest.NewNullLogger()
	reg, err := features.Bootstrap(&features.Flags{})
	if err != nil {
		b.Fatal(err)
	}
	reg.Resolve(
		features.SurfaceInput{Values: []string{"native-histograms"}, Supplied: true},
		features.SurfaceInput{},
		features.SurfaceInput{},
		logger,
	)
	flags := reg.Flags()

	var sink int
	b.ReportAllocs()
	for range b.N {
		if flags.NativeHistograms {
			sink++
		}
	}
	runtimeSink = sink
}

// runtimeSink prevents the compiler from optimizing the benchmark loop away.
var runtimeSink int //nolint:gochecknoglobals
