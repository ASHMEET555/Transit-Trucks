package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
)

func TestSuggest_PureScoringFunction(t *testing.T) {
	// Candidate A: 5000 kg cargo, 10000 kg cap (0.5 util), safety 90.0, odo 50000
	// score = (0.5 * 100) + (90 * 0.6) - (50000 / 100000) = 50 + 54 - 0.5 = 103.5
	scoreA := ScoreCandidate(5000, 10000, 90.0, 50000)
	if scoreA != 103.5 {
		t.Fatalf("expected score 103.5, got %f", scoreA)
	}

	// Candidate B: 5000 kg cargo, 6000 kg cap (~0.8333 util), safety 80.0, odo 20000
	// score = (0.83333 * 100) + (80 * 0.6) - (20000 / 100000) = 83.33 + 48 - 0.2 = 131.13
	scoreB := ScoreCandidate(5000, 6000, 80.0, 20000)
	if scoreB <= scoreA {
		t.Fatalf("higher utilization vehicle should score higher: scoreB=%f, scoreA=%f", scoreB, scoreA)
	}

	// Zero capacity protection
	if ScoreCandidate(5000, 0, 90.0, 10000) != -1.0 {
		t.Fatal("expected -1.0 for zero capacity")
	}
}

func TestSuggest_EndpointReadOnlyAndNoRowMutation(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "suggest_admin@transitops.com", rbac.RoleAdmin)

	// Count rows before
	var vCountBefore, dCountBefore, tCountBefore int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM vehicles").Scan(&vCountBefore)
	_ = db.QueryRow(ctx, "SELECT count(*) FROM drivers").Scan(&dCountBefore)
	_ = db.QueryRow(ctx, "SELECT count(*) FROM trips").Scan(&tCountBefore)

	body, _ := json.Marshal(map[string]any{
		"cargo_weight":     4500,
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
	})
	req := httptest.NewRequest("POST", "/api/v1/trips/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from suggest, got %d: %s", w.Code, w.Body.String())
	}

	// Count rows after: MUST BE IDENTICAL (Suggest never mutates data)
	var vCountAfter, dCountAfter, tCountAfter int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM vehicles").Scan(&vCountAfter)
	_ = db.QueryRow(ctx, "SELECT count(*) FROM drivers").Scan(&dCountAfter)
	_ = db.QueryRow(ctx, "SELECT count(*) FROM trips").Scan(&tCountAfter)

	if vCountBefore != vCountAfter || dCountBefore != dCountAfter || tCountBefore != tCountAfter {
		t.Fatalf("suggest endpoint must not mutate database: v=%d/%d, d=%d/%d, t=%d/%d",
			vCountBefore, vCountAfter, dCountBefore, dCountAfter, tCountBefore, tCountAfter)
	}

	var res suggestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check that ranked pairs are sorted descending by score
	for i := 1; i < len(res.RankedPairs); i++ {
		if res.RankedPairs[i].Score > res.RankedPairs[i-1].Score {
			t.Fatalf("ranked pairs not sorted descending: [%d]=%f > [%d]=%f",
				i, res.RankedPairs[i].Score, i-1, res.RankedPairs[i-1].Score)
		}
	}
}

func TestSuggest_CompatibilityAndIneligibleReasons(t *testing.T) {
	// Category Heavy drives all
	if !IsLicenseCategoryCompatible("Heavy", "Semi") || !IsLicenseCategoryCompatible("Heavy", "Van") {
		t.Fatal("Heavy license must be compatible with all vehicles")
	}

	// Category Medium drives Van, Light Truck, Medium Truck, Box Truck
	if !IsLicenseCategoryCompatible("Medium", "Box Truck") {
		t.Fatal("Medium license must be compatible with Box Truck")
	}
	if IsLicenseCategoryCompatible("Medium", "Semi") {
		t.Fatal("Medium license must NOT be compatible with Semi")
	}

	// Category Van drives Van and Light Truck only
	if !IsLicenseCategoryCompatible("Van", "Van") || !IsLicenseCategoryCompatible("Van", "Light Truck") {
		t.Fatal("Van license must be compatible with Van and Light Truck")
	}
	if IsLicenseCategoryCompatible("Van", "Heavy Truck") {
		t.Fatal("Van license must NOT be compatible with Heavy Truck")
	}

	// Expiry check
	today := time.Now().UTC()
	if !IsDriverLicenseExpired(today.AddDate(-1, 0, 0).Format("2006-01-02"), today) {
		t.Fatal("past date must be expired")
	}
	if IsDriverLicenseExpired(today.AddDate(1, 0, 0).Format("2006-01-02"), today) {
		t.Fatal("future date must not be expired")
	}
}
