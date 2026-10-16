#!/bin/sh
set -eu

curl --fail-with-body --request POST http://localhost:8080/cutover \
  --header 'Content-Type: application/json' \
  --data '{
    "domain":"example.com",
    "hostname":"track.example.com",
    "previous_target":"192.0.2.10",
    "next_target":"192.0.2.20",
    "shipments":[{
      "id":"SHP-1042",
      "events":[{"kind":"delivered","at":"2026-10-01T09:30:00Z"}],
      "proof_of_delivery":{"object_key":"pod/SHP-1042.pdf","sha256":"8d969eef"},
      "exceptions":[{"code":"ADDRESS_CHECK","resolved":true}]
    }]
  }'
