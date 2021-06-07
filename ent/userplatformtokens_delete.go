// Code generated by entc, DO NOT EDIT.

package ent

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/shitamachi/push-service/ent/predicate"
	"github.com/shitamachi/push-service/ent/userplatformtokens"
)

// UserPlatformTokensDelete is the builder for deleting a UserPlatformTokens entity.
type UserPlatformTokensDelete struct {
	config
	hooks    []Hook
	mutation *UserPlatformTokensMutation
}

// Where adds a new predicate to the UserPlatformTokensDelete builder.
func (uptd *UserPlatformTokensDelete) Where(ps ...predicate.UserPlatformTokens) *UserPlatformTokensDelete {
	uptd.mutation.predicates = append(uptd.mutation.predicates, ps...)
	return uptd
}

// Exec executes the deletion query and returns how many vertices were deleted.
func (uptd *UserPlatformTokensDelete) Exec(ctx context.Context) (int, error) {
	var (
		err      error
		affected int
	)
	if len(uptd.hooks) == 0 {
		affected, err = uptd.sqlExec(ctx)
	} else {
		var mut Mutator = MutateFunc(func(ctx context.Context, m Mutation) (Value, error) {
			mutation, ok := m.(*UserPlatformTokensMutation)
			if !ok {
				return nil, fmt.Errorf("unexpected mutation type %T", m)
			}
			uptd.mutation = mutation
			affected, err = uptd.sqlExec(ctx)
			mutation.done = true
			return affected, err
		})
		for i := len(uptd.hooks) - 1; i >= 0; i-- {
			mut = uptd.hooks[i](mut)
		}
		if _, err := mut.Mutate(ctx, uptd.mutation); err != nil {
			return 0, err
		}
	}
	return affected, err
}

// ExecX is like Exec, but panics if an error occurs.
func (uptd *UserPlatformTokensDelete) ExecX(ctx context.Context) int {
	n, err := uptd.Exec(ctx)
	if err != nil {
		panic(err)
	}
	return n
}

func (uptd *UserPlatformTokensDelete) sqlExec(ctx context.Context) (int, error) {
	_spec := &sqlgraph.DeleteSpec{
		Node: &sqlgraph.NodeSpec{
			Table: userplatformtokens.Table,
			ID: &sqlgraph.FieldSpec{
				Type:   field.TypeInt,
				Column: userplatformtokens.FieldID,
			},
		},
	}
	if ps := uptd.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	return sqlgraph.DeleteNodes(ctx, uptd.driver, _spec)
}

// UserPlatformTokensDeleteOne is the builder for deleting a single UserPlatformTokens entity.
type UserPlatformTokensDeleteOne struct {
	uptd *UserPlatformTokensDelete
}

// Exec executes the deletion query.
func (uptdo *UserPlatformTokensDeleteOne) Exec(ctx context.Context) error {
	n, err := uptdo.uptd.Exec(ctx)
	switch {
	case err != nil:
		return err
	case n == 0:
		return &NotFoundError{userplatformtokens.Label}
	default:
		return nil
	}
}

// ExecX is like Exec, but panics if an error occurs.
func (uptdo *UserPlatformTokensDeleteOne) ExecX(ctx context.Context) {
	uptdo.uptd.ExecX(ctx)
}