# API examples (`mmq api`, `mmq <noun>`)

Inputs are validated against the live schema of the connected broker (`mmq schema <path>`).

```bash
mmq --json user createUser --args @examples/api/create-user.json
mmq --json user createUser username=operator1 password=change-me-123 isAdmin=false   # flat key=value
mmq --json publishBatch --args @examples/api/publish-batch.json
mmq --json archiveGroup create --args @examples/api/create-archive-group.json
mmq --json topicSchema create --args @examples/api/create-topic-schema-policy.json
mmq --json namespace create --args @examples/api/create-topic-namespace.json
mmq --json namespace get line1-sensors            # first positional = first argument
mmq --json api user.createUser --args @examples/api/create-user.json --dry-run   # show generated GraphQL
```
