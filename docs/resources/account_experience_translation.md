---
page_title: "ory_account_experience_translation Resource - ory"
subcategory: ""
description: |-
  Manages the custom text of the Ory Account Experience, the hosted login UI, for one locale.
---

# ory_account_experience_translation (Resource)

Manages the custom text of the Ory Account Experience, the hosted login UI, for one locale. This is the text that the locale editor in the Ory Console changes.

Each resource owns one locale and leaves the other locales unchanged. Separate configurations can therefore manage separate locales of one project.

~> **Note:** Do not run two applies against the same project at the same time. Ory applies each change to the project configuration as it was when the request started, then saves the whole result. Two concurrent changes can therefore overwrite each other. The next plan shows the lost change, and the next apply writes it again. Within one apply, the provider sends the changes to a project one at a time.

-> **Plan:** Available on all Ory Network plans.

## Example Usage

```terraform
# Custom text for the English hosted login. A key that is not in the map keeps
# the built-in Ory Elements text.
resource "ory_account_experience_translation" "en" {
  locale = "en"
  messages = {
    "login.title"        = "Sign in to Example"
    "login.subtitle"     = "Sign in with {parts}"
    "recovery.title"     = "Forgot your password?"
    "registration.title" = "Create your Example account"
  }
}

# Custom text for the German hosted login, kept in a JSON file with the same
# flat format as the Ory Elements locale files. The hosted login shows the text
# only for a locale it serves, so "de" must be in
# account_experience_enabled_locales on your ory_project_config resource.
resource "ory_account_experience_translation" "de" {
  locale   = "de"
  messages = jsondecode(file("${path.module}/translations/de.json"))
}
```

The `translations/de.json` file that the example reads:

```json
{
  "login.title": "Bei Example anmelden",
  "recovery.title": "Passwort vergessen?",
  "registration.title": "Example-Konto erstellen"
}
```

## Messages

`messages` maps message keys to the text that the hosted login shows. A key that is not in the map keeps the built-in text of the locale. The keys are the ones that the Console locale editor lists:

| Keys | Example | Source |
|------|---------|--------|
| Ory Elements messages | `login.title` | The [Ory Elements locale files](https://github.com/ory/elements/tree/main/packages/elements-react/src/locales) |
| Ory Kratos UI messages | `identities.messages.1010001` | The ID of the [UI message](https://www.ory.com/docs/kratos/concepts/ui-messages). The Ory Elements locale files also hold these keys. |
| Form field labels | `forms.label.traits.email` | The trait path in the identity schema |
| Footer link labels | `footer.terms_of_service.label`, `footer.privacy_policy.label`, `footer.contact.label` | The links that `account_experience_terms_of_service_url`, `account_experience_privacy_policy_url`, and `account_experience_contact_url` set on `ory_project_config` |

The text uses ICU message syntax. Keep the placeholders of the built-in text, for example `{parts}` in `login.subtitle`.

`messages` must contain at least one entry. To restore the built-in text of a locale, destroy the resource.

## Locales

The hosted login shows the custom text of the locale that it renders a page in. When `account_experience_locale_behavior` is `force_default`, that locale is `account_experience_default_locale`. Otherwise it is the best match between the `Accept-Language` header of the browser and `account_experience_enabled_locales`, and `en` when nothing matches. Set these attributes on `ory_project_config`.

Ory stores the text of any valid locale, but the hosted login shows it only for the locale that it renders. The match is exact and case-sensitive, so set `locale` to the same string as the entry in `account_experience_enabled_locales`. For example, text for `pt-br` does not apply to `pt-BR`, and text for `pt-BR` does not apply to `pt`.

## Drift Detection

The provider sends `messages` as a JSON document. Ory stores the document and reports a URL whose file name is the SHA-512 hash of the document. The provider compares that hash with the hash of the messages in state, so an unchanged locale shows no diff and needs no download. When the hash differs, for example after an edit in the Console, the provider downloads the stored document, and the next plan shows the change.

A document that the Console wrote usually orders its keys differently, so its hash does not match. The provider then downloads it on every refresh, until an apply changes the messages of that locale.

## Important Behaviors

- **Destroying restores the built-in text.** Delete removes the custom text of the locale. It does not change `account_experience_enabled_locales`.
- **An existing locale is taken over.** When the locale already has custom text, for example from the Console, the first apply replaces that text with the configured messages.
- **`locale` cannot be changed** after creation. Changing it forces a new resource.

## Import

Import through the project ID and the locale:

```shell
terraform import ory_account_experience_translation.en <project-id>/en
```

The locale alone also works when the provider block sets `project_id`:

```shell
terraform import ory_account_experience_translation.en en
```

The import downloads the stored document, so `messages` holds every key that the locale stores.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `locale` (String) Language tag of the locale the text is for, for example en or de. The hosted login shows the text only for a locale it serves: an entry in account_experience_enabled_locales, or account_experience_default_locale, on ory_project_config. The match is exact and case-sensitive, so text for pt-br never shows for pt-BR. Changing it forces a new resource.
- `messages` (Map of String) Map of message keys, for example login.title, to the text the hosted login shows for this locale. A key that is not in the map keeps the built-in text. Must contain at least one entry.

### Optional

- `project_id` (String) Project ID. If not set, uses the provider's project_id.

### Read-Only

- `id` (String) Resource ID. The same value as locale.
