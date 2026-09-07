# gog ads mutate

Validate or apply an atomic Google Ads REST mutation using the existing Ads
OAuth connection and developer token. Use this for campaign budgets, Search
campaigns, criteria, ads, conversion goal configuration and experiment setup.

```sh
gog --account ads@example.com ads mutate 1234567890 --service googleAds --body-file request.json
# After reviewing the validation result and approving the exact request:
gog --account ads@example.com --force ads mutate 1234567890 --service googleAds --body-file request.json --apply
```

The default calls Google with `validateOnly: true`; `--dry-run` instead prints
locally without authentication or network access. File flags cannot disable
validation or enable partial failures. `--apply` requires the normal confirmation
(or `--force` in non-interactive use). `--readonly` blocks this command.

The JSON file is a REST request object containing a nonempty `mutateOperations`
array for `googleAds`, or `operations` for the resource-specific service named in
`--help`. Google validates operation schemas and resource permissions. Use paused
campaigns during setup. This generic command can also update or remove resources;
review the exact operations before applying. Validation does not reserve names,
resources, or permissions and may differ from a later apply as state changes.

Responses are JSON, including the original provider response. Applied writes do
not retry automatically. If a write times out or returns a server error, inspect
account state before retrying to avoid duplicate resources. Experiments require
separate scheduling before they can run; this mutation command does not schedule
them automatically.

To validate scheduling an existing experiment, use
`gog ads experiment schedule CUSTOMER_ID EXPERIMENT_ID`. Add `--apply` with
confirmation to schedule it. Scheduling can start serving and spending; an
operation receipt does not prove that setup has finished. Query the experiment
status and campaigns to verify completion before claiming it is running.
