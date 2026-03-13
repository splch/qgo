//go:build integration

package quantinuum

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/splch/goqu/backend"
	"github.com/splch/goqu/circuit/builder"
)

func TestIntegrationSyntaxChecker(t *testing.T) {
	email := os.Getenv("QUANTINUUM_EMAIL")
	password := os.Getenv("QUANTINUUM_PASSWORD")
	if email == "" || password == "" {
		t.Skip("QUANTINUUM_EMAIL and QUANTINUUM_PASSWORD not set")
	}

	b := New(email, password, WithDevice("H1-1SC"))

	// Build a Bell circuit.
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	job, err := b.Submit(ctx, &backend.SubmitRequest{
		Circuit: c,
		Shots:   10,
		Name:    "goqu-integration-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("submitted job %s to %s", job.ID, b.Name())

	// Poll for completion.
	for {
		status, err := b.Status(ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("job %s: %s", job.ID, status.State)
		if status.State.Terminal() {
			if status.State == backend.StateFailed {
				t.Fatalf("job failed: %s", status.Error)
			}
			break
		}
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			t.Fatal("timeout waiting for job")
		}
	}

	result, err := b.Result(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("results: %v", result.Counts)
}
