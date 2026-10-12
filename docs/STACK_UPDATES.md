# Managed stack updates

Manual **Pull** is available for ordinary projects even without an enabled check
schedule, after a check reports current images, or when automatic updates are
disabled. It runs Compose pull without recreating containers. Per-project and
bulk Pull use the same rule. Stack Manager itself uses Settings > Update.

Manual **Update** also does not require a prior check or an available-update
notice. It respects the project's explicit update policy and uses its existing
Compose/environment files for pull and up (or its configured update hook).

This procedure applies to Docker Compose projects managed by Stack Manager. It
does not describe upgrading Stack Manager itself; see [the platform guide](PLATFORM.md).

## Checks and updates are separate actions

- An update check is read-only. It reports registry/source state and does not
  pull images, restart containers, or edit `compose.yml` or `.env`.
- Checks run only for projects with an enabled `check` schedule. There is no
  startup or nightly sweep of every project's registries.
- An image update is a separate action. It runs only when an operator starts it
  or an explicit `update` schedule is due. The normal Compose path pulls the
  images and runs `docker compose up -d`; it uses the project's existing Compose
  and environment files as-is.

## Production update procedure

1. Select the owning host and project. Review that project's README, vendor
   upgrade notes, and any Stack Manager update hook before acting. A documented
   migration or hook takes precedence over the generic pull/up sequence.
2. Create a Stack Manager project backup before the update. Confirm the backup
   completed and is visible in the project's Backups view. Check that the
   configured backup destination has enough space and is reachable.
3. Run the project's explicit update action or its approved update schedule.
   Do not edit, regenerate, replace, or normalize `compose.yml` or `.env` as part
   of an image update unless the project's own upgrade instructions require that
   exact change.
4. Read back `docker ps -a` on the owning host and preserve the actual state of
   every service. Check the project's documented health endpoint and relevant
   startup logs. Confirm persistent mounts and the expected application version.
5. Confirm that `compose.yml` and `.env` are unchanged by the update. If either
   changed unexpectedly or a service fails, stop further update attempts and
   investigate before restoring from the verified backup.

Scheduled update actions must follow the same backup requirement. A check
schedule is not an update schedule and cannot initiate a pull or restart.
Stack Manager currently does not automatically create or require a successful
backup before a scheduled update. For production, use a manual update after a
verified backup; treat automatic backup-before-update enforcement as an
unimplemented safeguard, not as something a separate backup schedule guarantees.

## Startup host-port conflicts

If Compose cannot start because a published host port is already in use, the
dashboard shows a warning beside the failed action/session output. Identify the
port owner on that stack's server. If needed, the operator can choose a free
**host** port using the stack's documented port variable or Compose mapping,
keeping the container port unchanged and updating any external client URLs.
Stack Manager must not change default ports, rewrite configuration, or stop
another service to resolve a host-specific conflict. OpenBrain Voice retains
its original Mem0 host default of 8765; a collision does not justify changing
that default for everyone.

## Catalog template replacement

Applying a catalog template is a configuration migration, not a routine image
update. It can replace the Compose model and merge template defaults into `.env`.
Review the full preview and any GPU, port, environment, service, and mount
changes before explicitly confirming it. Stack Manager makes timestamped
Compose and `.env` backups before writing, and refuses replacement when it
cannot verify template provenance or when persistent mount topology changes.
Never use catalog replacement merely to pull newer images.

## Project-specific instructions

Keep upgrade instructions beside each project in a README or `docs/` directory
and link to upstream release notes. Specify required pre-update backups,
configuration changes, migration commands, health checks, and rollback steps.
If there are no project-specific instructions for changing Compose or `.env`,
preserve those files and use the existing pull/up update path.
