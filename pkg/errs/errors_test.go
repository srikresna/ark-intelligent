package errs_test

import (
	"errors"
	"testing"

	"github.com/arkcode369/ark-intelligent/pkg/errs"
)

func TestSentinelErrorsAreDistinct(t *testing.T) {
	sentinels := []error{
		errs.ErrNoData,
		errs.ErrRateLimited,
		errs.ErrNotFound,
		errs.ErrTimeout,
		errs.ErrBadData,
		errs.ErrUpstream,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %d (%v) should not match sentinel %d (%v)", i, a, j, b)
			}
		}
	}
}

func TestWrap(t *testing.T) {
	wrapped := errs.Wrap(errs.ErrRateLimited, "alphavantage")
	if wrapped == nil {
		t.Fatal("Wrap returned nil")
	}
	if !errors.Is(wrapped, errs.ErrRateLimited) {
		t.Error("wrapped error should match ErrRateLimited via errors.Is")
	}
	want := "alphavantage: rate limited"
	if wrapped.Error() != want {
		t.Errorf("got %q, want %q", wrapped.Error(), want)
	}
}

func TestWrapNil(t *testing.T) {
	if errs.Wrap(nil, "ctx") != nil {
		t.Error("Wrap(nil) should return nil")
	}
}

func TestWrapf(t *testing.T) {
	wrapped := errs.Wrapf(errs.ErrUpstream, "socrata status %d", 503)
	if !errors.Is(wrapped, errs.ErrUpstream) {
		t.Error("Wrapf error should match ErrUpstream")
	}
	want := "socrata status 503: upstream error"
	if wrapped.Error() != want {
		t.Errorf("got %q, want %q", wrapped.Error(), want)
	}
}

func TestWrapfNil(t *testing.T) {
	if errs.Wrapf(nil, "ctx %d", 1) != nil {
		t.Error("Wrapf(nil) should return nil")
	}
}

func TestDoubleWrap(t *testing.T) {
	inner := errs.Wrap(errs.ErrNoData, "socrata")
	outer := errs.Wrap(inner, "cot.FetchLatest")
	if !errors.Is(outer, errs.ErrNoData) {
		t.Error("double-wrapped error should still match sentinel")
	}
	want := "cot.FetchLatest: socrata: no data available"
	if outer.Error() != want {
		t.Errorf("got %q, want %q", outer.Error(), want)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{errs.ErrRateLimited, true},
		{errs.ErrTimeout, true},
		{errs.ErrUpstream, true},
		{errs.ErrNoData, false},
		{errs.ErrNotFound, false},
		{errs.ErrBadData, false},
		{errs.Wrap(errs.ErrRateLimited, "av"), true},
		{errs.Wrap(errs.ErrNoData, "fred"), false},
		{nil, false},
	}
	for _, tt := range tests {
		got := errs.IsRetryable(tt.err)
		if got != tt.want {
			t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}
