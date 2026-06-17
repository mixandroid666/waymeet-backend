// Package location powers the "nearby people" discovery tab.
//
// Responsibilities:
//   - Update the current user's last-known location (GEOGRAPHY point).
//   - Radius search with filters (distance, gender, age range) using PostGIS
//     ST_DWithin over a GiST index on users.location.
//
// Maps to the Flutter Location screen. This geospatial requirement is the main
// reason the stack is Postgres + PostGIS rather than a document store.
package location
