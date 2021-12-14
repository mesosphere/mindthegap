package testutils

import "testing"

func SkipIfShort(t *testing.T, msgAndArgs ...interface{}) {
	t.Helper()
	if testing.Short() {
		t.Skip(msgAndArgs...)
	}
}
