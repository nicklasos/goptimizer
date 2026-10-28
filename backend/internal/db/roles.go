package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Roles is users.roles: a JSON array of role names in a TEXT column.
type Roles []string

func (r Roles) Value() (driver.Value, error) {
	if r == nil {
		return "[]", nil
	}
	encoded, err := json.Marshal([]string(r))
	if err != nil {
		return nil, err
	}
	return string(encoded), nil
}

func (r *Roles) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return fmt.Errorf("db.Roles: cannot scan %T", src)
	}
	return json.Unmarshal(raw, (*[]string)(r))
}
