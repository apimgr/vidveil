# TODO.AI.md

## Provider-native workflow verification

- [ ] Resolve the historical full-history TruffleHog finding from
  `src/notify/notify_test.go` without weakening secret scanning or rewriting
  Git history. The current push-range scan passes; the scheduled full-history
  scan failed on an older commit.
- [ ] GitLab: run `.gitlab-ci.yml` on a configured GitLab runner and verify
  lint, test/coverage, security, release, and Docker jobs. This checkout has no
  GitLab remote or runner credentials.
- [ ] Gitea: run the CI workflow on a configured Gitea runner and verify each
  job's exit status. The local YAML parse passed, but Gitea-specific contexts
  cannot be validated by GitHub `act`.
- [ ] Forgejo: run the CI workflow on a configured Forgejo runner and verify
  each job's exit status. The local YAML parse passed, but Forgejo-specific
  contexts cannot be validated by GitHub `act`.
- [ ] Jenkins: run the pipeline on a configured Jenkins controller with the
  required `amd64` and `arm64` agents and credentials. Verify the 3am release
  build and 4am devel-image build use the scheduled timestamp for
  classification.
