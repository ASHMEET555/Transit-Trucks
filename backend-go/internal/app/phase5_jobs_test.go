package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

func TestCompulsory_LicenseCheckJobAndNotifications(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	cfg := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "production",
		CookieDomain:   "transitops.com",
		JobSecret:      "job-secret-license-check-999",
	}
	app := New(db, cfg)

	// 1. Unauthorized call without X-Job-Secret -> 401
	reqNoSecret := httptest.NewRequest("POST", "/api/v1/internal/jobs/license-check", nil)
	wNoSecret := httptest.NewRecorder()
	app.ServeHTTP(wNoSecret, reqNoSecret)

	if wNoSecret.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for job without secret, got %d", wNoSecret.Code)
	}

	// 2. Call with invalid X-Job-Secret -> 401
	reqWrong := httptest.NewRequest("POST", "/api/v1/internal/jobs/license-check", nil)
	reqWrong.Header.Set("X-Job-Secret", "wrong-secret")
	wWrong := httptest.NewRecorder()
	app.ServeHTTP(wWrong, reqWrong)

	if wWrong.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for job with wrong secret, got %d", wWrong.Code)
	}

	// Create test drivers expiring in 5 days (7d threshold) and 20 days (30d threshold)
	dLic7 := fmt.Sprintf("DL-7D-%s", uuid.NewString()[:6])
	expiry7 := time.Now().UTC().AddDate(0, 0, 5)
	createTestDriver(t, db, dLic7, "Available", "Heavy", expiry7, 90.0)

	dLic30 := fmt.Sprintf("DL-30D-%s", uuid.NewString()[:6])
	expiry30 := time.Now().UTC().AddDate(0, 0, 20)
	createTestDriver(t, db, dLic30, "Available", "Standard", expiry30, 85.0)

	// 3. Authorized call with valid X-Job-Secret -> 200 OK
	reqJob := httptest.NewRequest("POST", "/api/v1/internal/jobs/license-check", nil)
	reqJob.Header.Set("X-Job-Secret", "job-secret-license-check-999")
	wJob := httptest.NewRecorder()
	app.ServeHTTP(wJob, reqJob)

	if wJob.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid license check job, got %d: %s", wJob.Code, wJob.Body.String())
	}

	var resJob struct {
		Status               string `json:"status"`
		DriversChecked       int    `json:"drivers_checked"`
		NotificationsCreated int    `json:"notifications_created"`
	}
	if err := json.Unmarshal(wJob.Body.Bytes(), &resJob); err != nil {
		t.Fatalf("failed to parse job response: %v", err)
	}
	if resJob.NotificationsCreated < 2 {
		t.Fatalf("expected at least 2 notifications created, got %d", resJob.NotificationsCreated)
	}

	// 4. Idempotence test: second run with same secret -> 0 new notifications
	reqJob2 := httptest.NewRequest("POST", "/api/v1/internal/jobs/license-check", nil)
	reqJob2.Header.Set("X-Job-Secret", "job-secret-license-check-999")
	wJob2 := httptest.NewRecorder()
	app.ServeHTTP(wJob2, reqJob2)

	var resJob2 struct {
		Status               string `json:"status"`
		NotificationsCreated int    `json:"notifications_created"`
	}
	_ = json.Unmarshal(wJob2.Body.Bytes(), &resJob2)
	if resJob2.NotificationsCreated != 0 {
		t.Fatalf("expected 0 notifications on second idempotent run, got %d", resJob2.NotificationsCreated)
	}

	// 5. Notifications API authorization and consumption
	// Safety Officer: can list notifications
	soCookie := createTestUserAndLogin(t, app, "so.notif.test@transitops.com", rbac.RoleSafetyOfficer)
	reqList := httptest.NewRequest("GET", "/api/v1/notifications", nil)
	reqList.AddCookie(soCookie)
	wList := httptest.NewRecorder()
	app.ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("expected 200 for Safety Officer listing notifications, got %d", wList.Code)
	}

	var listResp struct {
		Items []Notification `json:"items"`
	}
	if err := json.Unmarshal(wList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to parse notifications list: %v", err)
	}
	if len(listResp.Items) == 0 {
		t.Fatalf("expected at least 1 notification in list")
	}

	// Mark first notification as read
	firstNotifID := listResp.Items[0].ID
	reqRead := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/notifications/%d/read", firstNotifID), nil)
	reqRead.AddCookie(soCookie)
	wRead := httptest.NewRecorder()
	app.ServeHTTP(wRead, reqRead)

	if wRead.Code != http.StatusOK {
		t.Fatalf("expected 200 marking notification read, got %d: %s", wRead.Code, wRead.Body.String())
	}

	// Driver role: forbidden from reading notifications
	driverCookie := createTestUserAndLogin(t, app, "driver.notif.test@transitops.com", rbac.RoleDriver)
	reqDriver := httptest.NewRequest("GET", "/api/v1/notifications", nil)
	reqDriver.AddCookie(driverCookie)
	wDriver := httptest.NewRecorder()
	app.ServeHTTP(wDriver, reqDriver)

	if wDriver.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Driver accessing notifications, got %d", wDriver.Code)
	}
}
