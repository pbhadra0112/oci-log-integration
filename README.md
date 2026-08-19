[![Community Plus header](https://github.com/newrelic/opensource-website/raw/master/src/images/categories/Community_Plus.png)](https://opensource.newrelic.com/oss-category/#community-plus)

# New Relic OCI Log Integrations

This repository contains integrations to forward logs from Oracle Cloud Infrastructure (OCI).

## Prerequisites

* [New Relic Ingest Key & API Key](https://docs.newrelic.com/docs/apis/intro-apis/new-relic-api-keys/#license-key)
* OCI user with Cloud Administrator role to create resources/stacks

## Custom forwarder metrics

The log forwarder Function can emit its own `forwarder.*` custom metrics directly to New Relic's Metric API, in addition to forwarding logs. This is opt-in via the `metrics_tier` Terraform variable (`none` / `basic` / `advanced`; default `none`). `basic` covers core health (record counts, delivery success/loss, delivery latency, pipeline lag); `advanced` adds deeper root-cause/tuning metrics (byte volumes, decode/serialize errors, batching behavior, delivery error classes, run duration, secret-fetch failures, client-cache hit rate) on top of everything in `basic`.

## Monitoring dashboard

A pre-built New Relic dashboard template for all `forwarder.*` metrics is included in [`docs/oci-log-forwarder-metrics-dashboard-template.json`](docs/oci-log-forwarder-metrics-dashboard-template.json).

**To import:**
1. In New Relic, go to **Dashboards → Import dashboard**.
2. Paste the contents of the JSON file.
3. Replace `YOUR_ACCOUNT_ID` with your numeric New Relic account ID (if you haven't already).

The dashboard has two pages:

- **Basic Metrics** — available at `metrics_tier=basic` or `advanced`. Shows invocations, record flow (received / delivered / dropped), delivery health, and pipeline lag.
- **Advanced Metrics** — available at `metrics_tier=advanced`. Shows byte volumes, error breakdown by type, batching efficiency, run duration, and client cache hit rate.

## Contributing

We encourage your contributions to improve oci-log-integration! Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project. If you have any questions, or to execute our corporate CLA, required if your contribution is on behalf of a company, please drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites, we welcome and greatly appreciate you reporting it to New Relic through [HackerOne](https://hackerone.com/newrelic).

## License

oci-log-integration is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.
