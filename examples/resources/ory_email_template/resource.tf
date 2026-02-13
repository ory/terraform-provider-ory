# Recovery code email
resource "ory_email_template" "recovery_code" {
  template_type = "recovery_code_valid"
  subject       = "Reset your password"

  body_html = <<-HTML
    <!DOCTYPE html>
    <html>
    <head>
      <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .code { font-size: 32px; font-weight: bold; color: #0066cc; letter-spacing: 4px; }
      </style>
    </head>
    <body>
      <div class="container">
        <h1>Password Reset</h1>
        <p>Hi {{ .Identity.traits.name.first }},</p>
        <p>Your recovery code is:</p>
        <p class="code">{{ .RecoveryCode }}</p>
        <p>This code expires in 1 hour.</p>
        <p>If you didn't request this, please ignore this email.</p>
      </div>
    </body>
    </html>
  HTML

  body_plaintext = <<-TEXT
    Password Reset

    Hi {{ .Identity.traits.name.first }},

    Your recovery code is: {{ .RecoveryCode }}

    This code expires in 1 hour.

    If you didn't request this, please ignore this email.
  TEXT
}

# Verification code email
resource "ory_email_template" "verification_code" {
  template_type = "verification_code_valid"
  subject       = "Verify your email address"

  body_html = <<-HTML
    <!DOCTYPE html>
    <html>
    <body>
      <h1>Email Verification</h1>
      <p>Welcome! Please verify your email address.</p>
      <p>Your verification code is: <strong>{{ .VerificationCode }}</strong></p>
    </body>
    </html>
  HTML

  body_plaintext = <<-TEXT
    Email Verification

    Welcome! Please verify your email address.

    Your verification code is: {{ .VerificationCode }}
  TEXT
}

# Login code email
resource "ory_email_template" "login_code" {
  template_type = "login_code_valid"
  subject       = "Your login code"

  body_html = <<-HTML
    <!DOCTYPE html>
    <html>
    <body>
      <h1>Login Code</h1>
      <p>Your login code is: <strong>{{ .LoginCode }}</strong></p>
      <p>This code expires in 15 minutes.</p>
    </body>
    </html>
  HTML

  body_plaintext = <<-TEXT
    Login Code

    Your login code is: {{ .LoginCode }}

    This code expires in 15 minutes.
  TEXT
}

# Registration code email
resource "ory_email_template" "registration_code" {
  template_type = "registration_code_valid"
  subject       = "Complete your registration"

  body_html = <<-HTML
    <!DOCTYPE html>
    <html>
    <body>
      <h1>Welcome!</h1>
      <p>Your registration code is: <strong>{{ .RegistrationCode }}</strong></p>
      <p>Enter this code to complete your account setup.</p>
    </body>
    </html>
  HTML

  body_plaintext = <<-TEXT
    Welcome!

    Your registration code is: {{ .RegistrationCode }}

    Enter this code to complete your account setup.
  TEXT
}
