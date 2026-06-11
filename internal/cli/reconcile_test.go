package cli

import (
	"strings"
	"testing"
)

// TestReconcile locks in the post-load integrity check that hardens pgpipe
// against silent row loss: a run that processed N rows must have imported +
// skipped == N, with zero skips, to be considered clean. Anything else must
// surface as a non-nil error (whose message operators may grep) so the CLI
// exits non-zero.
func TestReconcile(t *testing.T) {
	cases := []struct {
		name                         string
		processed, imported, skipped int64
		wantErr                      bool
		wantSubstr                   string
	}{
		{"clean load", 1000, 1000, 0, false, ""},
		{"empty source table", 0, 0, 0, false, ""},
		{"rows skipped but accounted", 1000, 990, 10, true, "incomplete load"},
		{"silent gap (imported < processed, skipped 0)", 1000, 944, 0, true, "silent loss"},
		{"silent gap (the 56k-names regression)", 11299121, 11242681, 0, true, "silent loss"},
		{"over-count (imported > processed)", 1000, 1010, 0, true, "gap="},
		{"gap even with some skips counted", 1000, 900, 50, true, "silent loss"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := reconcile(tc.processed, tc.imported, tc.skipped)
			switch {
			case tc.wantErr && err == nil:
				t.Errorf("reconcile(%d,%d,%d) = nil, want error",
					tc.processed, tc.imported, tc.skipped)
			case !tc.wantErr && err != nil:
				t.Errorf("reconcile(%d,%d,%d) = %v, want nil",
					tc.processed, tc.imported, tc.skipped, err)
			case tc.wantSubstr != "" && err != nil && !strings.Contains(err.Error(), tc.wantSubstr):
				t.Errorf("reconcile(%d,%d,%d) error = %q, want substring %q",
					tc.processed, tc.imported, tc.skipped, err.Error(), tc.wantSubstr)
			}
		})
	}
}
