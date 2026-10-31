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
	EventVehiclePosition     = "vehicle.position_updated"
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
	EventVehiclePosition,
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
		if eventType == EventVehiclePosition {
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

// AllowedPayloadKeys defines the strict allowlist of permitted keys per event type.
// Absolutely no PII (names, license numbers, emails, phone numbers) is permitted in client payloads.
var AllowedPayloadKeys = map[string]map[string]bool{
	EventTripCreated:         {"trip_id": true, "vehicle_reg": true},
	EventTripDispatched:      {"trip_id": true, "vehicle_reg": true},
	EventTripCompleted:       {"trip_id": true, "vehicle_reg": true},
	EventTripCancelled:       {"trip_id": true, "vehicle_reg": true},
	EventVehicleStatusChange: {"vehicle_reg": true, "status": true},
	EventDriverStatusChange:  {"status": true},
	EventMaintenanceOpened:   {"maintenance_id": true, "vehicle_reg": true},
	EventMaintenanceClosed:   {"maintenance_id": true, "vehicle_reg": true},
	EventExpenseCreated:      {"expense_id": true, "trip_id": true, "type": true},
	EventNotificationCreated: {"notification_id": true, "type": true, "threshold": true},
	EventVehiclePosition:     {"vehicle_reg": true, "trip_id": true},
}

// SanitizedClientPayload returns a copy of IDs filtered strictly against the per-event-type allowlist.
// Payloads contain only minimal IDs and status/type metadata; no PII can ever reach clients.
func SanitizedClientPayload(eventType string, rawIDs map[string]any) map[string]any {
	allowed, ok := AllowedPayloadKeys[eventType]
	if !ok {
		return map[string]any{}
	}
	out := make(map[string]any)
	for k, v := range rawIDs {
		if allowed[k] {
			out[k] = v
		}
	}
	return out
}
