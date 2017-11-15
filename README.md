# TeamCity Exporter

## Environment variables

* `TE_LISTEN_ADDRESS` – Listen address (`:9190`)
* `TE_METRIC_PATH` – Metric path (`/metrics`)
* `TE_API_LOGIN` – API login
* `TE_API_PASSWORD` – API password
* `TE_API_URL` – API URL

## Metrics

* `teamcity_up` – Was the last query of TeamCity successful
* `teamcity_build_queue_count` – How many builds in queue at the last query
