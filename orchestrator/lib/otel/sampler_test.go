package otel

import "testing"

func TestSamplingRatioFromEnv(t *testing.T) {
	t.Run("unset defaults to 1.0", func(t *testing.T) {
		t.Setenv("OTEL_TRACES_SAMPLER_ARG", "")
		if got := samplingRatioFromEnv(); got != 1.0 {
			t.Fatalf("got %v, want 1.0", got)
		}
	})
	t.Run("parses a valid ratio", func(t *testing.T) {
		t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")
		if got := samplingRatioFromEnv(); got != 0.25 {
			t.Fatalf("got %v, want 0.25", got)
		}
	})
	t.Run("invalid value falls back to 1.0", func(t *testing.T) {
		t.Setenv("OTEL_TRACES_SAMPLER_ARG", "not-a-number")
		if got := samplingRatioFromEnv(); got != 1.0 {
			t.Fatalf("got %v, want 1.0", got)
		}
	})
}
