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
