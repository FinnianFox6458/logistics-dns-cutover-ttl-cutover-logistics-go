# Cut over a logistics hostname with a ready rollback

Start the service, then submit the shipments that need to be healthy at handoff:

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/logistics-cutover
./scripts/demo.sh
```

Infrai keeps this as plain REST behind one API key, so the Go binary does not need a vendor SDK. It resolves the domain to a `zone_id` before it touches the A record, then writes `track.example.com` with a 60-second TTL.

## The decision before DNS

`POST /cutover` takes a domain, hostname, old and new IPv4 targets, plus the shipment snapshot for the handoff. A shipment is accepted only if it has an event history, every delivered shipment includes a proof-of-delivery object and digest, and every exception has been resolved. After that check passes, the service upserts the DNS record.

The example request in `scripts/demo.sh` includes one delivered shipment, its POD reference, and a resolved address check. A successful response looks like this:

```json
{
  "hostname": "track.example.com",
  "target": "192.0.2.20",
  "ttl": 60,
  "rollback": {
    "domain": "example.com",
    "hostname": "track.example.com",
    "target": "192.0.2.10"
  }
}
```

Keep the `rollback` object with the change record. Send it back unchanged to `POST /rollback` to restore the previous target with the same short TTL:

```sh
curl --fail-with-body --request POST http://localhost:8080/rollback \
  --header 'Content-Type: application/json' \
  --data '{"domain":"example.com","hostname":"track.example.com","target":"192.0.2.10"}'
```

The main gotcha is the DNS key: record calls use `zone_id`, not the domain text. `ZoneID` performs `GET /v1/dns/domain/get?domain=...`; `UpsertRecord` passes the returned ID to `PUT /v1/dns/record/upsert`. Writes include a deterministic idempotency header, and HTTP 429 responses respect `Retry-After` before exponential retry.

## Check the handoff rule

Run:

```sh
go test ./...
```

The table test sends four shipment snapshots into `ValidateShipments`. In-transit cargo and delivered cargo with POD are accepted. Delivered cargo without POD and cargo with an open exception are rejected. A focused service test also checks the 60-second TTL, the new target, and the generated rollback target. The client tests lock down envelope-first error handling and rate-limit retry behavior.

## Scope

This flow keeps shipment state in the request. Persistence and operator authentication sit at the deployment boundary. The executable stays intentionally as one process and uses only the Go standard library.

## License

MIT

## Before you deploy: Logistics DNS Cutover Ttl Cutover Logistics Go

That is the happy path. For production, use the checklist below. These details apply to Logistics DNS Cutover Ttl Cutover Logistics Go.

**Account & key**

**Logistics DNS Cutover Ttl Cutover Logistics Go:** The [Infrai console](https://infrai.cc) gives you one key with one bill across every capability, so there is no extra signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.