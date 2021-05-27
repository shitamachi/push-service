package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"time"
)

// UserPushToken holds the schema definition for the UserPushToken entity.
type UserPushToken struct {
	ent.Schema
}

// Fields of the UserPushToken.
func (UserPushToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").Unique(),
		field.String("token"),
		field.String("app_id"),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the UserPushToken.
func (UserPushToken) Edges() []ent.Edge {
	return nil
}
