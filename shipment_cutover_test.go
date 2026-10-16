package cutover

import (
	"context"
	"testing"
)

func TestValidateShipments(t *testing.T) {
	tests := []struct {
		name      string
		shipment  Shipment
		wantError bool
	}{
		{
			name:     "in transit",
			shipment: Shipment{ID: "S-1", Events: []ShipmentEvent{{Kind: "dispatched", At: "2026-10-01T08:00:00Z"}}},
		},
		{
			name: "delivered with proof",
			shipment: Shipment{
				ID: "S-2", Events: []ShipmentEvent{{Kind: "delivered", At: "2026-10-01T09:00:00Z"}},
				Proof: &ProofOfDelivery{ObjectKey: "pod/S-2.pdf", SHA256: "8d969eef"},
			},
		},
		{
			name:      "delivered without proof",
			shipment:  Shipment{ID: "S-3", Events: []ShipmentEvent{{Kind: "delivered", At: "2026-10-01T09:00:00Z"}}},
			wantError: true,
		},
		{
			name: "open exception",
			shipment: Shipment{
				ID: "S-4", Events: []ShipmentEvent{{Kind: "dispatched", At: "2026-10-01T08:00:00Z"}},
				Exceptions: []Exception{{Code: "ADDRESS_CHECK", Resolved: false}},
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateShipments([]Shipment{test.shipment})
			if (err != nil) != test.wantError {
				t.Fatalf("ValidateShipments() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}

type fakeDNS struct {
	record Record
	key    string
}

func (f *fakeDNS) ZoneID(context.Context, string) (string, error) { return "zone_123", nil }
func (f *fakeDNS) UpsertRecord(_ context.Context, record Record, key string) error {
	f.record, f.key = record, key
	return nil
}

func TestCutoverProducesRollbackAndShortTTL(t *testing.T) {
	dns := &fakeDNS{}
	request := CutoverRequest{
		Domain: "example.com", Hostname: "track.example.com", PreviousTarget: "192.0.2.10", NextTarget: "192.0.2.20",
		Shipments: []Shipment{{ID: "S-9", Events: []ShipmentEvent{{Kind: "dispatched", At: "2026-10-01T08:00:00Z"}}}},
	}
	result, err := (Service{DNS: dns}).Cutover(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if dns.record.ZoneID != "zone_123" || dns.record.TTL != 60 || dns.record.Content != request.NextTarget {
		t.Fatalf("unexpected DNS record: %+v", dns.record)
	}
	if dns.key == "" || result.Rollback.Target != request.PreviousTarget {
		t.Fatalf("missing retry key or rollback: %+v", result)
	}
}
