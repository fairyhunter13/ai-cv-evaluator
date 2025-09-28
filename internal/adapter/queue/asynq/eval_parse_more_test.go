package asynqadp

import "testing"

func Test_parseCVExtract_SuccessAndInvalid(t *testing.T) {
	good := `prefix {"skills":["go"],"experiences":[],"projects":[],"summary":"ok."} suffix`
	if _, err := parseCVExtract(good); err != nil { t.Fatalf("unexpected err: %v", err) }
	bad := `no json here`
	if _, err := parseCVExtract(bad); err == nil { t.Fatalf("expected error") }
}

func Test_parseProjectExtract_SuccessAndInvalid(t *testing.T) {
	good := `prefix {"requirements":["r1"],"architecture":[],"strengths":[],"risks":[],"summary":"ok"} suffix`
	if _, err := parseProjectExtract(good); err != nil { t.Fatalf("unexpected err: %v", err) }
	bad := `{not json}`
	if _, err := parseProjectExtract(bad); err == nil { t.Fatalf("expected error") }
}
