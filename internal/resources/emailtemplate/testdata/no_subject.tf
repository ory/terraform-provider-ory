resource "ory_email_template" "test" {
  template_type  = "verification_code_valid"
  body_html      = "<p>Your verification code is: {{ .VerificationCode }}</p>"
  body_plaintext = "Your verification code is: {{ .VerificationCode }}"
}
