---
page_title: "ory_relationship Resource - ory"
subcategory: ""
description: |-
  Manages an Ory Keto relationship tuple for fine-grained authorization.
---

# ory_relationship (Resource)

Manages an Ory Keto relationship tuple for fine-grained authorization.

Relationships are the foundation of Ory Permissions (Keto), which implements [Google's Zanzibar](https://research.google/pubs/pub48190/) authorization model. They define who (subject) has what relation to which resource (object) in a namespace.

## Prerequisites

~> **Important:** You must configure Keto namespaces via `ory_project_config` **before** creating relationships. If the namespace doesn't exist, the API will return a 404 error.

```terraform
resource "ory_project_config" "main" {
  keto_namespaces = ["documents", "folders", "groups"]
}

resource "ory_relationship" "example" {
  namespace  = "documents"
  object     = "doc-123"
  relation   = "viewer"
  subject_id = "user-456"

  depends_on = [ory_project_config.main]
}
```

## Relationship Tuple Structure

A relationship tuple consists of:
- **namespace**: The type of resource (e.g., `documents`, `folders`) — must be configured in `keto_namespaces`
- **object**: The specific resource ID
- **relation**: The type of relationship (e.g., `viewer`, `editor`, `owner`)
- **subject**: Either a user ID (`subject_id`) or another relationship tuple (`subject_set_*`)

~> **Note:** `subject_id` and `subject_set_*` attributes are mutually exclusive. You must provide either `subject_id` (for a direct user) or all three `subject_set_*` attributes (for an inherited permission), but not both.

## Example Usage

```terraform
# Prerequisite: configure Keto namespaces first
resource "ory_project_config" "main" {
  keto_namespaces = ["documents", "folders", "groups", "projects"]
}

# Direct user permission: user-123 can view doc-456
resource "ory_relationship" "user_view_doc" {
  namespace  = "documents"
  object     = "doc-456"
  relation   = "viewer"
  subject_id = "user-123"

  depends_on = [ory_project_config.main]
}

# User is member of a group
resource "ory_relationship" "user_in_group" {
  namespace  = "groups"
  object     = "engineering"
  relation   = "member"
  subject_id = "user-123"
}

# Group members can edit project (subject set)
resource "ory_relationship" "group_edit_project" {
  namespace             = "projects"
  object                = "project-abc"
  relation              = "editor"
  subject_set_namespace = "groups"
  subject_set_object    = "engineering"
  subject_set_relation  = "member"
}

# Hierarchical permissions: folder viewers can view documents in folder
resource "ory_relationship" "folder_viewer_inheritance" {
  namespace             = "documents"
  object                = "doc-789"
  relation              = "viewer"
  subject_set_namespace = "folders"
  subject_set_object    = "folder-123"
  subject_set_relation  = "viewer"
}

# Complete RBAC example
locals {
  users = {
    alice = "user-alice"
    bob   = "user-bob"
  }

  docs = {
    public  = "public-doc"
    private = "private-doc"
  }
}

# Alice owns the private doc
resource "ory_relationship" "alice_owns_private" {
  namespace  = "documents"
  object     = local.docs.private
  relation   = "owner"
  subject_id = local.users.alice
}

# Bob can view the public doc
resource "ory_relationship" "bob_views_public" {
  namespace  = "documents"
  object     = local.docs.public
  relation   = "viewer"
  subject_id = local.users.bob
}
```

## Important Behaviors

- **Relationships are immutable.** Any change to any field forces a new resource (destroy + create).
- **All three `subject_set_*` fields must be set together.** Setting only `subject_set_namespace` without `subject_set_object` and `subject_set_relation` will produce an error.

## Import

Import using Zanzibar-style tuple notation:

```shell
# Simple subject (user ID)
terraform import ory_relationship.example "namespace:object#relation@subject_id"

# Subject set (inherited permission)
terraform import ory_relationship.example "namespace:object#relation@subject_namespace:subject_object#subject_relation"
```

### Examples

```shell
# user-456 is a viewer of doc-123 in documents namespace
terraform import ory_relationship.user_can_view "documents:doc-123#viewer@user-456"

# editors of folder-789 are viewers of doc-123
terraform import ory_relationship.editors_can_view "documents:doc-123#viewer@folders:folder-789#editor"
```

## API Details

This resource manages Keto relationship tuples via the `/admin/relation-tuples` API endpoint. Authentication requires a **Project API Key** (`ory_pat_...`).

For debugging, you can interact with the API directly:

```shell
# List relationships in a namespace
curl -s -H "Authorization: Bearer $ORY_PROJECT_API_KEY" \
  "https://$ORY_PROJECT_SLUG.projects.oryapis.com/admin/relation-tuples?namespace=documents"

# Check a specific permission
curl -s -H "Authorization: Bearer $ORY_PROJECT_API_KEY" \
  "https://$ORY_PROJECT_SLUG.projects.oryapis.com/relation-tuples/check?namespace=documents&object=doc-123&relation=viewer&subject_id=user-456"
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `namespace` (String) The namespace of the relationship (e.g., 'documents', 'folders').
- `object` (String) The object ID in the namespace.
- `relation` (String) The relation type (e.g., 'viewer', 'editor', 'owner').

### Optional

- `subject_id` (String) The subject ID (user ID). Mutually exclusive with subject_set_* attributes.
- `subject_set_namespace` (String) The namespace for a subject set. Use with subject_set_object and subject_set_relation.
- `subject_set_object` (String) The object ID for a subject set.
- `subject_set_relation` (String) The relation for a subject set.

### Read-Only

- `id` (String) Internal Terraform ID (composite of namespace:object#relation@subject).
