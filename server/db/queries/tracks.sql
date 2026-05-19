-- name: ListTracks :many
SELECT code, display_name, sort_order FROM tracks ORDER BY sort_order, code;
