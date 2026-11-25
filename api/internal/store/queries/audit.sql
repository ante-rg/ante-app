-- name: InsertAuditLog :exec
INSERT INTO audit_log (actor, method, path, status) VALUES (@actor, @method, @path, @status);
