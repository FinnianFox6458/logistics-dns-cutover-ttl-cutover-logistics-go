package cutover

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type ShipmentEvent struct {
	Kind string `json:"kind"`
	At   string `json:"at"`
}

type ProofOfDelivery struct {
	ObjectKey string `json:"object_key"`
	SHA256    string `json:"sha256"`
}

type Exception struct {
	Code     string `json:"code"`
	Resolved bool   `json:"resolved"`
}

type Shipment struct {
	ID         string           `json:"id"`
	Events     []ShipmentEvent  `json:"events"`
	Proof      *ProofOfDelivery `json:"proof_of_delivery,omitempty"`
	Exceptions []Exception      `json:"exceptions,omitempty"`
}

type CutoverRequest struct {
	Domain         string     `json:"domain"`
	Hostname       string     `json:"hostname"`
	PreviousTarget string     `json:"previous_target"`
	NextTarget     string     `json:"next_target"`
	Shipments      []Shipment `json:"shipments"`
}

type RollbackRequest struct {
	Domain   string `json:"domain"`
	Hostname string `json:"hostname"`
	Target   string `json:"target"`
}

type CutoverResult struct {
	Hostname string          `json:"hostname"`
	Target   string          `json:"target"`
	TTL      int             `json:"ttl"`
	Rollback RollbackRequest `json:"rollback"`
}

type DNS interface {
	ZoneID(context.Context, string) (string, error)
	UpsertRecord(context.Context, Record, string) error
}

type Service struct{ DNS DNS }

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

func ValidateShipments(shipments []Shipment) error {
	if len(shipments) == 0 {
		return invalid("at least one shipment is required")
	}
	for _, shipment := range shipments {
		if shipment.ID == "" || len(shipment.Events) == 0 {
			return invalid("shipment %q has no event history", shipment.ID)
		}
		delivered := false
		for _, event := range shipment.Events {
			if event.Kind == "delivered" {
				delivered = true
			}
		}
		if delivered && (shipment.Proof == nil || shipment.Proof.ObjectKey == "" || shipment.Proof.SHA256 == "") {
			return invalid("shipment %s is delivered without proof", shipment.ID)
		}
		for _, exception := range shipment.Exceptions {
			if !exception.Resolved {
				return invalid("shipment %s has unresolved exception %s", shipment.ID, exception.Code)
			}
		}
	}
	return nil
}

func (s Service) Cutover(ctx context.Context, request CutoverRequest) (CutoverResult, error) {
	if err := ValidateShipments(request.Shipments); err != nil {
		return CutoverResult{}, err
	}
	if request.Domain == "" || request.Hostname == "" || request.PreviousTarget == "" || request.NextTarget == "" {
		return CutoverResult{}, invalid("domain, hostname, previous_target, and next_target are required")
	}
	if err := s.point(ctx, request.Domain, request.Hostname, request.NextTarget); err != nil {
		return CutoverResult{}, err
	}
	return CutoverResult{
		Hostname: request.Hostname, Target: request.NextTarget, TTL: 60,
		Rollback: RollbackRequest{Domain: request.Domain, Hostname: request.Hostname, Target: request.PreviousTarget},
	}, nil
}

func (s Service) Rollback(ctx context.Context, request RollbackRequest) (CutoverResult, error) {
	if request.Domain == "" || request.Hostname == "" || request.Target == "" {
		return CutoverResult{}, invalid("domain, hostname, and target are required")
	}
	if err := s.point(ctx, request.Domain, request.Hostname, request.Target); err != nil {
		return CutoverResult{}, err
	}
	return CutoverResult{Hostname: request.Hostname, Target: request.Target, TTL: 60}, nil
}

func (s Service) point(ctx context.Context, domain, hostname, target string) error {
	zoneID, err := s.DNS.ZoneID(ctx, domain)
	if err != nil {
		return err
	}
	record := Record{ZoneID: zoneID, RecordType: "A", Name: hostname, Content: target, TTL: 60}
	sum := sha256.Sum256([]byte(strings.Join([]string{zoneID, hostname, target, "60"}, "\x00")))
	return s.DNS.UpsertRecord(ctx, record, hex.EncodeToString(sum[:]))
}
