package observability

import "testing"

func TestSetAppEnv_SetsDevEnvironment(t *testing.T) {
	appEnv = ""
	SetAppEnv("DEV")
	if !isDevEnv() {
		t.Fatalf("expected dev environment after SetAppEnv(\"DEV\")")
	}
}

func TestRecordJobFailureByCode_DefaultsUnknownAndCustom(_ *testing.T) {
	// These calls should be safe regardless of metric registration state and
	// exercise the UNKNOWN default path as well as a concrete code.
	RecordJobFailureByCode("evaluate", "")
	RecordJobFailureByCode("evaluate", "UPSTREAM_TIMEOUT")
}
