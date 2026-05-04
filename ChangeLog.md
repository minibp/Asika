## v20260504DEV
- Fix self-update endpoint returning 500 error (github.NewClient(nil) fixed with proper auth)
- Add webhook retry mechanism with exponential backoff (max 10 attempts)
- Add audit logging system with API endpoint GET /api/v1/logs
- Add PR comment support (API + CLI: asika pr comment)
- Add draft PR detection for GitHub/GitLab/Gitea with merge queue skip
- Add PR conflict detection with merge queue skip
- Add batch operations: approve/close/label multiple PRs (API + CLI)
- Add search filters: is_draft, author, label, created_after, updated_after, pagination
- Add Discord notifier with channel message support
- Add Discord interactive bot with PR management commands (!prs, !pr, !approve, !close, !spam, !queue, etc.)
- Add Gitea @ mention notification support (gitea_at)
- Register gitea_at and discord notification types in config and handlers
- Update asika.toml.example with Discord and Gitea @ notification config
- Improve error handling: return 502 when platform client unavailable instead of silent empty response
- Add fallback to 'default' repo group when requested group not found

## v20260503DEV
- Inital commit
- 75% function supported
