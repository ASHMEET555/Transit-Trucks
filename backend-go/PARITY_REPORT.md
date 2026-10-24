# Parity report

Static implementation review completed for 28 HTTP method/path handlers and one WebSocket endpoint. Runtime parity could not run because Docker Desktop's Linux engine is unavailable on this machine. Use the commands in the project README before accepting production parity.

Intentional retained behaviours: JWT bearer authentication only, no logout or `me` endpoint (despite the task wording; neither exists in Python), permissive wildcard CORS with credentials, direct plaintext seed passwords accepted by login, and the Python KPI treatment of all trips as revenue/distance regardless of status.

The Go port follows the Postgres Alembic schema. SQLite adds `CHECK` constraints for roles, vehicle status, driver status, trip status, maintenance status and expense type; the Alembic/Postgres schema has none of these. SQLite permits nullable `status` and `safety_score` due to omitted `NOT NULL`, while Postgres requires both. Their default values otherwise correspond.

Known Python behaviours reproduced: all authenticated users may complete/cancel trips; no route verifies source/destination existence before dispatch (the database foreign key does); status values supplied during vehicle/driver creation are not validated in application code; KPI revenue/distance includes Draft and Cancelled trips; dashboard maintenance cost includes every log state; `/api/auth/login` treats non-bcrypt stored passwords as plaintext; CORS combines `*` origin and credentials; no `logout` or `me` endpoints exist.
