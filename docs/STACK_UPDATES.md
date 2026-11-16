# Managed stack updates

Manual **Pull** is available for ordinary projects even without an enabled check
schedule, after a check reports current images, or when automatic updates are
disabled. It runs Compose pull without recreating containers. Per-project and
bulk Pull use the same rule. Stack Manager itself uses Settings > Update.

Manual **Update** also does not require a prior check or an available-update
notice. It respects the project's explicit update policy and uses its existing
Compose/environment files for pull and up (or its configured update hook).

Individual manual Pull and Update buttons start tracked background jobs. Their
default pull timeout is `0` (no deadline), so a large image can finish after
the operator leaves the page. Set a positive timeout in seconds when a limit is
needed. The job and its output remain available in action history when it
finishes. A controller restart can interrupt a running job; check Docker state
before retrying. Bulk actions and scheduled updates retain their configured
time limits and should be planned separately for large images.

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

## Post-update verification

After Compose recreates the containers, Stack Manager samples their state
again 30 seconds later to catch restart loops. If a service is running but its
healthcheck still reports starting, verification keeps sampling every 10
seconds for up to 5 minutes before it gives up. Slow starters such as GitLab and
Pi-hole therefore pass once they turn healthy. A service that turns unhealthy,
restarts, or exits fails at once. If the healthcheck is still starting when the
5 minutes run out, the job fails and says how long it waited.

## Automatic post-update Docker cleanup

After a managed stack update passes container stability/health verification,
Stack Manager runs these commands on the stack's owning Docker daemon:

```sh
docker image prune --force --filter until=168h
docker builder prune --all --force --filter until=168h --keep-storage 1GB
```

The default removes only dangling, unused images and aged unused build cache.
It never prunes containers, networks or volumes, and does not pass `--all` to
image pruning. Tagged images and images still referenced by a container are
preserved. Cleanup is daemon-wide, including eligible artifacts from other
projects on that host. It is not limited to the updated Compose project.
This is separate from the existing manual/scheduled full-prune options,
which can delete volumes and are not used by automatic cleanup.

Docker's image age filter uses **image creation time**, not the time an image
became unused. An old image can become eligible immediately after an update
replaces its last container. Before a production update, retain an explicit tag
on any image needed for rollback, or verify its immutable digest can be pulled
again. Pruned images/cache cannot be undeleted: recovery means pulling the
retained digest or rebuilding. Keep verified data backups; cleanup does not
create them. Tagged images are deliberately retained even if that means this
policy cannot reclaim all disk usage.

Configure each controller/agent host separately in its `.env` and recreate its
server/agent container through the normal approved deployment workflow. The
development pipeline preserves the installed `.env` byte for byte:

| Setting | Default | Behavior |
| --- | --- | --- |
| `UPDATE_CLEANUP_ENABLED` | `1` | Set to `0` or `false` to disable automatic cleanup. |
| `UPDATE_CLEANUP_MIN_AGE_HOURS` | `168` | Integer from `24` to `87600`; applies to image and cache pruning. |

Standalone CLI installations use the same variables in `stack-manager.conf`
or the environment. `--no-update-cleanup` disables cleanup for that invocation.
CLI dry runs describe the commands without running verification or pruning.
CLI cleanup requires the `timeout` utility; if unavailable, cleanup is skipped
with a warning while the established update action remains usable. Each prune
command has a two-minute limit. API/agent cleanup is serialized within the
process; external Docker operations can still cause a prune conflict, which is
reported as a warning rather than retried automatically.

Direct API and bulk updates, dashboard background jobs, scheduled updates and
updated agents share this behavior. The standalone CLI uses the same cleanup
policy and samples container IDs/restart counts across the verification window.
A failed pull, failed recreate, or failed verification never triggers cleanup.
Custom update hooks still replace pull/up; after a successful hook, container
verification gates cleanup only. A hook that intentionally leaves a stack down
keeps its existing success status, with a cleanup-skipped warning. Pull-only,
check, start, restart and Stack Manager's dedicated self-update actions do not
trigger this automatic cleanup. Older peers/agents need their own approved
software update before they gain this behavior.

Update output/action history and CLI logs include cleanup commands, Docker's
per-operation reclaimed-space totals (`Total reclaimed space` or BuildKit's
`Total`), and any errors. A cleanup failure preserves the successful update
status and reports a warning; inspect the daemon and retry cleanup separately
instead of rerunning an update that may include migrations. Invalid cleanup
settings also skip cleanup with a warning. The standalone CLI's `--prune`
combined with `update` uses only this verified image/cache cleanup; the standalone
legacy `prune` command retains its explicit resource-pruning behavior.

References: [Docker image prune](https://docs.docker.com/reference/cli/docker/image/prune/)
and [Docker builder prune](https://docs.docker.com/reference/cli/docker/builder/prune/).

## Schedules for deleted projects

Deleting a project disables every local schedule that targets it, and the
delete result says how many were disabled. Schedules for agents are left alone
because they target a project on another node. If a schedule still runs for a
project that no longer exists, for example because the directory was removed by
hand, the scheduler records that run as failed once and then disables the
schedule. The schedule keeps its history and can be re-enabled
after the project is restored.

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
