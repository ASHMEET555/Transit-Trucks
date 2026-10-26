# TransitOps KPI Definitions & Formula Reference

This document defines the mathematical formulas and filtering behavior for all fleet operational KPIs in TransitOps (Task 4.3 / Phase 4 Remediation).

## Fleet Headline Metrics

1. **Active Vehicles**:
   - `COUNT(*) FILTER (WHERE status = 'On Trip' AND is_active = true)`
   - Filtered by `type` and `status` when provided.

2. **Available Vehicles**:
   - `COUNT(*) FILTER (WHERE status = 'Available' AND is_active = true)`
   - Filtered by `type` and `status` when provided.

3. **In Shop Vehicles**:
   - `COUNT(*) FILTER (WHERE status = 'In Shop' AND is_active = true)`
   - Filtered by `type` and `status` when provided.

4. **Retired Vehicles**:
   - `COUNT(*) FILTER (WHERE status = 'Retired' OR is_active = false)`
   - Filtered by `type` and `status` when provided.

5. **Fleet Utilization (%)**:
   - Formula: `(active_vehicles / (total_active_non_retired_vehicles)) * 100`
   - Expressed as a percentage rounded to 2 decimal places. 0.00% if divisor is 0.

6. **Active Trips**:
   - `COUNT(*) FILTER (WHERE status = 'Dispatched')`
   - Realtime active trips currently in transit. Not range-filtered.

7. **Pending Trips**:
   - `COUNT(*) FILTER (WHERE status = 'Draft')`
   - Trips scheduled or drafted awaiting dispatch. Not range-filtered. (Resolved per remediation B1: Draft trips only, excluding Dispatched).

8. **Completed Trips**:
   - `COUNT(*) FILTER (WHERE status = 'Completed')`
   - In date range (`from` to `to`, checked against `completed_at` / `updated_at` in UTC).

9. **Total Revenue**:
   - Sum of `revenue` for **Completed** trips in the specified date range. Cancelled and Draft trips are strictly excluded.

10. **Total Distance (km)**:
    - Sum of actual odometer difference `(final_odometer - start_odometer)` for **Completed** trips in range where `final_odometer >= start_odometer`.
    - **Decision Log (B3)**: The `planned_distance` fallback was removed. If odometer readings are missing or invalid, actual distance traversed cannot be assumed and defaults to 0.

11. **Total Operational Cost**:
    - Sum of:
      - Fuel expenses in range
      - Other expenses (Toll, Maintenance, Other) in range
      - Closed maintenance log costs in range (where `end_date` is in range)
    - Open maintenance logs and inactive trip expenses are excluded.

12. **Overall Fuel Efficiency (km/L)**:
    - Formula: `total_distance / total_fuel_liters`
    - Evaluated across completed trips and logged fuel expenses in the range. Returns `null` if total fuel liters is 0.

## Per-Vehicle Analytics

1. **Vehicle Revenue**: Total revenue from Completed trips for that vehicle in range.
2. **Vehicle Distance**: Actual odometer difference `SUM(final_odometer - start_odometer)` for Completed trips for that vehicle in range.
3. **Vehicle Fuel Cost**: Total cost of `type = 'Fuel'` expenses for that vehicle in range.
4. **Vehicle Fuel Liters**: Total liters of `type = 'Fuel'` expenses for that vehicle in range.
5. **Vehicle Maintenance Cost**: Sum of closed maintenance logs for that vehicle in range + maintenance expenses in range.
6. **Vehicle Operational Cost**: Total expenses + closed maintenance logs in range.
7. **Vehicle Fuel Efficiency (km/L)**: `distance / fuel_liters`. `null` if `fuel_liters == 0`.
8. **Vehicle ROI (%)**:
   - Formula: `((revenue - (fuel_cost + maintenance_cost)) / acquisition_cost) * 100`
   - `null` if `acquisition_cost <= 0`.

## Filter Behaviors

- `from` / `to`: Applied strictly to Completed trip timestamps, expense dates, and closed maintenance end dates. Does not filter snapshot counts (`active_vehicles`, `active_trips`, `pending_trips`).
- `type`: Filters vehicle types (`Van`, `Box Truck`, `Flatbed`, `Semi`, etc.). Rejects unlisted types with `422 VALIDATION_FAILED`.
- `status`: Filters vehicle statuses (`Available`, `On Trip`, `In Shop`, `Retired`). Rejects unlisted statuses with `422 VALIDATION_FAILED`.
- **Decision Log (B2 - Region Filter)**: The `region` column exists solely in the `destinations` table. The `vehicles`, `drivers`, `expenses`, and `maintenance_logs` tables do not have regional association. Therefore, `region` is omitted from vehicle dashboard analytics.
