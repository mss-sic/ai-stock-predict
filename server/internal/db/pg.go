package db

import "strings"

// CodesToInClause builds a safe SQL IN clause from stock codes (always 6-digit numeric).
// Example: ["300936","301151"] → "'300936','301151'"
func CodesToInClause(codes []string) string {
	if len(codes) == 0 {
		return "''"
	}
	quoted := make([]string, len(codes))
	for i, c := range codes {
		quoted[i] = "'" + c + "'"
	}
	return strings.Join(quoted, ",")
}
