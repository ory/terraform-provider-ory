// Maps the SCIM user resource to identity traits. The SCIM payload is
// available as `claims`.
local claims = std.extVar('claims');
{
  identity: {
    traits: {
      email: claims.email,
    },
  },
}
