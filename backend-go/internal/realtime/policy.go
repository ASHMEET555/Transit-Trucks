package realtime

import (
	"strings"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
)

// Allowed Event Types
const (
	EventTripCreated         = "trip.created"
	EventTripDispatched      = "trip.dispatched"
	EventTripCompleted       = "trip.completed"
	EventTripCancelled       = "trip.cancelled"
	EventVehicleStatusChange = "vehicle.status_changed"
	EventDriverStatusChange  = "driver.status_changed"
	EventMaintenanceOpened   = "maintenance.opened"
	EventMaintenanceClosed   = "maintenance.closed"
	EventExpenseCreated      = "expense.created"
	EventNotificationCreated = "notification.created"
)

// AllEventTypes lists all standard event types in Phase 5
var AllEventTypes = []string{
	EventTripCreated,
	EventTripDispatched,
	EventTripCompleted,
	EventTripCancelled,
	EventVehicleStatusChange,
	EventDriverStatusChange,
	EventMaintenanceOpened,
	EventMaintenanceClosed,
	EventExpenseCreated,
	EventNotificationCreated,
}

// CanReceive determines whether a user with a given role and ownership status
// is authorized to receive a specific event type.
//
// Rules from spec:
// - Fleet Manager & Admin: receive all fleet events.
// - Driver: receives trip.* events only when ownsTrip=true; receives driver.status_changed
//   only when ownsTrip=true (their own driver profile); never receives expense.created or maintenance.
// - Financial Analyst: receives expense.created and trip.completed (ids only);
//   never receives driver.status_changed or maintenance events.
// - Safety Officer: receives driver.status_changed, maintenance.*, and notification.created;
//   never receives expense.created or trips.
func CanReceive(eventType, role string, ownsTrip bool) bool {
	switch role {
	case rbac.RoleAdmin, rbac.RoleFleetManager:
		return true

	case rbac.RoleDriver:
		if strings.HasPrefix(eventType, "trip.") {
			return ownsTrip
		}
		if eventType == EventDriverStatusChange {
			return ownsTrip
		}
		return false

	case rbac.RoleFinancialAnalyst:
		if eventType == EventExpenseCreated || eventType == EventTripCompleted {
			return true
		}
		return false

	case rbac.RoleSafetyOfficer:
		if eventType == EventDriverStatusChange ||
			eventType == EventMaintenanceOpened ||
			eventType == EventMaintenanceClosed ||
			eventType == EventNotificationCreated {
			return true
		}
		return false

	default:
		return false
	}
}

// SanitizedClientPayload returns a copy of IDs stripped of any potential PII fields.
// Payloads contain no driver names, license numbers, phones, or email addresses.
func SanitizedClientPayload(rawIDs map[string]any) map[string]any {
	out := make(map[string]any, len(rawIDs))
	forbiddenKeys := map[string]bool{
		"name":           true,
		"driver_name":    true,
		"email":          true,
		"actor_email":    true,
		"phone":          true,
		"contact_number": true,
		"license_number": true,
		"driver_license": true,
	}

	for k, v := range rawIDs {
		lower := strings.ToLower(k)
		if !forbiddenKeys[lower] {
			out[k] = v
		}
	}
	return out
}
