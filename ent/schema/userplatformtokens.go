package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// UserPlatformTokens holds the schema definition for the UserPlatformTokens entity.
type UserPlatformTokens struct {
	ent.Schema
}

// Fields of the UserPlatformTokens.
func (UserPlatformTokens) Fields() []ent.Field {
	return []ent.Field{
		field.Uint8("type"),
		field.String("user_id"),
		field.String("device_id"),
		field.String("token"),
		field.String("app_id"),
		field.Time("created_at"),
		field.Time("updated_at"),
	}
}

// Edges of the UserPlatformTokens.
func (UserPlatformTokens) Edges() []ent.Edge {
	return nil
}
