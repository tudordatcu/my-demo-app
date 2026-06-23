package httpapi

import (
	"context"
	"math/rand"
)

// CarrierClient simulates a downstream HLR/carrier lookup dependency.
// Failures occur at approximately failureRate; the invoice path calls Lookup
// and surfaces these failures as 502 responses.
type CarrierClient struct {
	failureRate float64
}

// NewCarrierClient returns a carrier client that fails roughly failureRate
// of the time (0 = never fail, 1 = always fail).
func NewCarrierClient(failureRate float64) *CarrierClient {
	return &CarrierClient{failureRate: failureRate}
}

// Lookup simulates a network call to the carrier for the given MSISDN.
// It returns an error approximately failureRate proportion of calls.
func (c *CarrierClient) Lookup(ctx context.Context, msisdn string) error {
	if c.failureRate <= 0 {
		return nil
	}
	if rand.Float64() < c.failureRate {
		return &carrierError{msisdn: msisdn}
	}
	return nil
}

type carrierError struct {
	msisdn string
}

func (e *carrierError) Error() string {
	return "carrier: lookup failed for " + e.msisdn
}
