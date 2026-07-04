package e2e_encryption

import (
	"testing"
)

// These tests call the shared test functions which
// can be called from both standard Go tests and Ginkgo tests.
//
// This situation is temporary until we verify the new e2e-aws-operator-encryption-serial-ote job.
// Eventually all tests will be run only as part of the OTE framework.

func TestEncryptionTypeIdentity(t *testing.T) {
	testEncryptionTypeIdentity(t.Context(), t)
}

func TestEncryptionTypeUnset(t *testing.T) {
	testEncryptionTypeUnset(t.Context(), t)
}

func TestEncryptionTurnOnAndOff(t *testing.T) {
	testEncryptionTurnOnAndOff(t.Context(), t)
}
