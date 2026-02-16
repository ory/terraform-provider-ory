# Workspace API keys can only be created through the Ory Console.
# Use this resource to import and manage existing workspace API keys.
#
# Step 1: Add the resource block
resource "ory_workspace_api_key" "main" {
  name = "My API Key"
}

# Step 2: Import the key
# terraform import ory_workspace_api_key.main <workspace-id>/<key-id>
