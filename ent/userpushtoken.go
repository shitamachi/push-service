// Code generated by entc, DO NOT EDIT.

package ent

import (
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/shitamachi/push-service/ent/userpushtoken"
)

// UserPushToken is the model entity for the UserPushToken schema.
type UserPushToken struct {
	config `json:"-"`
	// ID of the ent.
	ID int `json:"id,omitempty"`
	// UserID holds the value of the "user_id" field.
	UserID string `json:"user_id,omitempty"`
	// Token holds the value of the "token" field.
	Token string `json:"token,omitempty"`
	// AppID holds the value of the "app_id" field.
	AppID string `json:"app_id,omitempty"`
	// CreatedAt holds the value of the "created_at" field.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// UpdatedAt holds the value of the "updated_at" field.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// scanValues returns the types for scanning values from sql.Rows.
func (*UserPushToken) scanValues(columns []string) ([]interface{}, error) {
	values := make([]interface{}, len(columns))
	for i := range columns {
		switch columns[i] {
		case userpushtoken.FieldID:
			values[i] = new(sql.NullInt64)
		case userpushtoken.FieldUserID, userpushtoken.FieldToken, userpushtoken.FieldAppID:
			values[i] = new(sql.NullString)
		case userpushtoken.FieldCreatedAt, userpushtoken.FieldUpdatedAt:
			values[i] = new(sql.NullTime)
		default:
			return nil, fmt.Errorf("unexpected column %q for type UserPushToken", columns[i])
		}
	}
	return values, nil
}

// assignValues assigns the values that were returned from sql.Rows (after scanning)
// to the UserPushToken fields.
func (upt *UserPushToken) assignValues(columns []string, values []interface{}) error {
	if m, n := len(values), len(columns); m < n {
		return fmt.Errorf("mismatch number of scan values: %d != %d", m, n)
	}
	for i := range columns {
		switch columns[i] {
		case userpushtoken.FieldID:
			value, ok := values[i].(*sql.NullInt64)
			if !ok {
				return fmt.Errorf("unexpected type %T for field id", value)
			}
			upt.ID = int(value.Int64)
		case userpushtoken.FieldUserID:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field user_id", values[i])
			} else if value.Valid {
				upt.UserID = value.String
			}
		case userpushtoken.FieldToken:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field token", values[i])
			} else if value.Valid {
				upt.Token = value.String
			}
		case userpushtoken.FieldAppID:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field app_id", values[i])
			} else if value.Valid {
				upt.AppID = value.String
			}
		case userpushtoken.FieldCreatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field created_at", values[i])
			} else if value.Valid {
				upt.CreatedAt = value.Time
			}
		case userpushtoken.FieldUpdatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field updated_at", values[i])
			} else if value.Valid {
				upt.UpdatedAt = value.Time
			}
		}
	}
	return nil
}

// Update returns a builder for updating this UserPushToken.
// Note that you need to call UserPushToken.Unwrap() before calling this method if this UserPushToken
// was returned from a transaction, and the transaction was committed or rolled back.
func (upt *UserPushToken) Update() *UserPushTokenUpdateOne {
	return (&UserPushTokenClient{config: upt.config}).UpdateOne(upt)
}

// Unwrap unwraps the UserPushToken entity that was returned from a transaction after it was closed,
// so that all future queries will be executed through the driver which created the transaction.
func (upt *UserPushToken) Unwrap() *UserPushToken {
	tx, ok := upt.config.driver.(*txDriver)
	if !ok {
		panic("ent: UserPushToken is not a transactional entity")
	}
	upt.config.driver = tx.drv
	return upt
}

// String implements the fmt.Stringer.
func (upt *UserPushToken) String() string {
	var builder strings.Builder
	builder.WriteString("UserPushToken(")
	builder.WriteString(fmt.Sprintf("id=%v", upt.ID))
	builder.WriteString(", user_id=")
	builder.WriteString(upt.UserID)
	builder.WriteString(", token=")
	builder.WriteString(upt.Token)
	builder.WriteString(", app_id=")
	builder.WriteString(upt.AppID)
	builder.WriteString(", created_at=")
	builder.WriteString(upt.CreatedAt.Format(time.ANSIC))
	builder.WriteString(", updated_at=")
	builder.WriteString(upt.UpdatedAt.Format(time.ANSIC))
	builder.WriteByte(')')
	return builder.String()
}

// UserPushTokens is a parsable slice of UserPushToken.
type UserPushTokens []*UserPushToken

func (upt UserPushTokens) config(cfg config) {
	for _i := range upt {
		upt[_i].config = cfg
	}
}
