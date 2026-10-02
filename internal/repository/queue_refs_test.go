package repository

import (
	"math"
	"testing"

	"github.com/goatkit/goatflow/internal/models"
)

// Reference ids beyond the INT column range must be rejected before any SQL
// runs, not wrapped onto some other system address / salutation / signature.
func TestQueueRefIDsOutOfRangeRejected(t *testing.T) {
	repo := NewQueueRepository(nil) // validation must fail before the DB is touched
	for _, q := range []*models.Queue{
		{Name: "q", SystemAddressID: math.MaxInt32 + 2},
		{Name: "q", SalutationID: math.MaxInt32 + 2},
		{Name: "q", SignatureID: math.MaxInt32 + 2},
	} {
		if err := repo.Create(q); err == nil {
			t.Errorf("Create(%+v) should reject out-of-range reference", q)
		}
		if err := repo.Update(q); err == nil {
			t.Errorf("Update(%+v) should reject out-of-range reference", q)
		}
	}

	sys, sal, sig, err := queueRefIDs(&models.Queue{SystemAddressID: math.MaxInt32, SignatureID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !sys.Valid || sys.Int32 != math.MaxInt32 || sal.Valid || !sig.Valid || sig.Int32 != 3 {
		t.Fatalf("got sys=%+v sal=%+v sig=%+v", sys, sal, sig)
	}
}
