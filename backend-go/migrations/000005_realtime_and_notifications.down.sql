-- Revert Migration 000005: Realtime tickets and notifications
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS realtime_tickets CASCADE;
