package store

// Exported for the external test package (legacy_golden_test.go).
var TableSQLv3 = tableSQLv3

func TriggerSQL() map[string]string { return triggerSQL() }
