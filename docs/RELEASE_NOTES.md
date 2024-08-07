# MongoDB Kubernetes Operator 0.11.0

## Migrating agent images to ubi
All agent images were updated to use the ubi repo

## Documentation improvements
Improvements were made to the documentatio of using the community operator as well as the one for local development.

## Logging changes
- Added `AuditLogRotate` field to `AgentConfiguration`
- Fixed JSON key to be lower case: `logRotate` 

## Bug Fixes
- Users removed from the resource are now also deleted from the database and their connection string secretes are cleaned up
- Colisions of the scram secret name will now be spotted by spec validation

## Important Bumps
- Bumped go to 1.22