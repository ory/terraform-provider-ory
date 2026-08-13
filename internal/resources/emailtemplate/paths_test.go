package emailtemplate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplatePath(t *testing.T) {
	r := &EmailTemplateResource{}
	cases := map[string]string{
		"recovery_code_valid":        "recovery_code/valid",
		"recovery_code_invalid":      "recovery_code/invalid",
		"verification_code_valid":    "verification_code/valid",
		"login_code_valid":           "login_code/valid",
		"registration_code_valid":    "registration_code/valid",
		"verifiable_address_changed": "verifiable_address_changed",
		"recovery":                   "recovery",
	}
	for templateType, want := range cases {
		t.Run(templateType, func(t *testing.T) {
			assert.Equal(t, want, r.templatePath(templateType))
		})
	}
}

// TestRecoveryCodeValidConfigPaths pins the config paths the acceptance test's
// destroy check watches. That check reads the API directly, so a path that
// stopped matching what Create writes would make it pass for the wrong reason:
// an absent key reads as "already cleared". Asserting the paths here keeps that
// failure mode out of the acceptance run, where it would be invisible.
// See issue #333.
func TestRecoveryCodeValidConfigPaths(t *testing.T) {
	r := &EmailTemplateResource{}
	base := fmt.Sprintf("/services/identity/config/courier/templates/%s/email",
		r.templatePath("recovery_code_valid"))

	assert.Equal(t, "/services/identity/config/courier/templates/recovery_code/valid/email/subject",
		base+"/subject")
	assert.Equal(t, "/services/identity/config/courier/templates/recovery_code/valid/email/body/html",
		base+"/body/html")
}
