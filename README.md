# foam-proxy

Lambda which supports authentication proxying for [foam](https://github.com/luke-h1/foam) app

## Tech stack

- [AWS Lambda](https://aws.amazon.com/lambda/)
- [Terraform](https://www.terraform.io/)
- [git-cliff](https://github.com/orhun/git-cliff) for changelog and version bumps

## Changelog & version

Changelog is generated from conventional commits. On push to `main`, the **Changelog** workflow updates `CHANGELOG.md` and commits it.

```bash
uv tool install pre-commit
uv tool install commitizen
pre-commit install
pre-commit install --hook-type commit-msg
```
