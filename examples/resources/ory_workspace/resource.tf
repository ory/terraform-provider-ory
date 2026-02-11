# Workspaces can only be created through the Ory Console.
# Use this resource to import and manage existing workspaces.
#
# Step 1: Add the resource block
resource "ory_workspace" "main" {
  name = "My Workspace"
}

# Step 2: Import the workspace
# terraform import ory_workspace.main <workspace-id>
