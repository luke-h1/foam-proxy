# Changesets

Each file in this directory is one release note. A pull request adds one with:

```sh
bun run changeset
```

Write one or two sentences for the person who runs or calls the proxy. The
bump type sets the heading the note lands under and the next version:

| Bump    | Heading          | Next tag              |
| ------- | ---------------- | --------------------- |
| `major` | Breaking changes | `X+1.0.0`             |
| `minor` | New              | `X.Y+1.0`             |
| `patch` | Fixed            | `X.Y.Z+1`             |

A change with nothing to tell a user (a refactor, a test, CI) still has to pass
the `Changeset present` check. Add an empty note:

```sh
bun run changeset -- --empty
```

The notes stay here until a merge to `main` deploys production. The release
job reads the highest bump type, applies it to the latest `X.Y.Z` tag, moves
the notes into `CHANGELOG.md` under that version, deletes the files, tags the
commit and creates the GitHub release. A merge with only empty notes deploys
but cuts no release. The `version` field in `package.json` is not used: the
git tags are the source of truth.

Preview the notes that are waiting:

```sh
bun run release:notes pending
```
