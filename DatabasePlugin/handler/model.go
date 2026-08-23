package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Data struct {
	ID    int `gorm:"primaryKey"`
	Path  string
	Type  string
	Dtype string
	date  time.Time
}

func SelectFunc(db *gorm.DB, ss []Select) (*gorm.DB, error) {
	for _, s := range ss {
		var value interface{}
		switch s.Operator {
		case "gt", "gte", "lt", "lte":
			if s.Field != "ID" && s.Field != "id" {
				return nil, fmt.Errorf("operator %s can only be applied to an integer field", s.Operator)
			}
			n, err := strconv.Atoi(strings.TrimSpace(s.Value))
			if err != nil {
				return nil, fmt.Errorf("invalid integer value %q for field %s: %w", s.Value, s.Field, err)
			}
			value = n
		case "eq", "neq":
			switch s.Field {
			case "ID", "id":
				n, err := strconv.Atoi(strings.TrimSpace(s.Value))
				if err != nil {
					return nil, fmt.Errorf("invalid integer value %q for field %s: %w", s.Value, s.Field, err)
				}
				value = n
			case "date":
				pt, err := parseTime(s.Value)
				if err != nil {
					return nil, fmt.Errorf("invalid time value %q for field %s: %w", s.Value, s.Field, err)
				}
				value = pt
			default:
				value = s.Value
			}
		case "before", "after":
			if s.Field != "date" {
				return nil, fmt.Errorf("operator %s can only be applied to a time field", s.Operator)
			}
			pt, err := parseTime(s.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid time value %q for field %s: %w", s.Value, s.Field, err)
			}
			value = pt
		default:
			return nil, fmt.Errorf("unknown operator: %s", s.Operator)
		}

		db = db.Where(sqlExpr(s.Field, s.Operator), value)
	}
	return db, nil
}

func sqlExpr(field, operator string) string {
	switch operator {
	case "gt", "after":
		return field + " > ?"
	case "gte":
		return field + " >= ?"
	case "lt", "before":
		return field + " < ?"
	case "eq":
		return field + " = ?"
	case "neq":
		return field + " <> ?"
	}
	return field + " = ?"
}

func parseTime(v string) (time.Time, error) {
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"}
	var lastErr error
	for _, l := range layouts {
		t, err := time.Parse(l, strings.TrimSpace(v))
		if err == nil {
			return t, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}
